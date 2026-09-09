#!/usr/bin/env python3
"""Build a Mac runtime, optionally packaging privately owned game discs."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import tarfile
import time

ROOT = Path(__file__).resolve().parents[1]
NATIVE = ROOT / "native/restoration"
ENGINE = ROOT / "vendor/dreamrefactory"
REVISION = "b43a02668f3db36519bd5b44a5892fdefd292208"
PUBLIC_WEB = {
    "index.html", "main.ts", "save-dialogs.ts", "save-extension.ts", "save-format.ts",
    "save-format-plugin.ts", "save-loading.ts", "neutral-save.ts", "neutral-cursors.ts",
    "vite.config.ts", "vitest.config.ts", "tsconfig.json", "neutral-save.test.ts",
    "save-loading.test.ts", "save-dialogs.test.ts",
}
PUBLIC_SCRIPTS = {"build-restored-app.py", "build-restored-app.sh", "verify-restored-app.py"}


def run(*args, **kwargs):
    subprocess.run([str(a) for a in args], check=True, **kwargs)


def sha(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def archive_bytes(archive, name, data, executable=False):
    entry = tarfile.TarInfo(name)
    entry.size = len(data)
    entry.mode = 0o755 if executable else 0o644
    entry.mtime = 0
    archive.addfile(entry, io.BytesIO(data))


def engine_files():
    """Only code and manifests needed to build the included engine, no media."""
    if (ENGINE / ".git").exists():
        names = subprocess.check_output(["git", "-C", str(ENGINE), "ls-files"], text=True).splitlines()
    else:
        names = json.loads((ENGINE / "UPSTREAM_SOURCE_SHA256.json").read_text()).keys()
    exact = {"LICENSE", "package.json", "package-lock.json", "tsconfig.json",
             "taoot/src/files.ts", "taoot/src/languages.ts", "site/src/games.ts"}
    packages = {f"{p}/package.json" for p in ("engine", "site", "taoot", "dust", "timelapse", "skullcracker")}
    return [ENGINE / name for name in sorted(names)
            if name in exact or name in packages or (name.startswith("engine/src/") and name.endswith(".ts"))]


def restoration_files():
    """Public corresponding-source allowlist; never include private QA ledgers."""
    files = list((NATIVE / "Sources").glob("*.swift"))
    files += [NATIVE / "Web" / name for name in PUBLIC_WEB]
    files += [ROOT / "scripts" / name for name in PUBLIC_SCRIPTS]
    files += [NATIVE / "THIRD_PARTY.md", ROOT / "BUILDING.md"]
    for name in ("Titanic.icns", "Titanic.png", "generate-icon.swift"):
        path = NATIVE / "Resources" / name
        if path.exists():
            files.append(path)
    return sorted(files)


def prepare_engine():
    if not ENGINE.exists():
        ENGINE.parent.mkdir(parents=True, exist_ok=True)
        run("git", "clone", "https://github.com/dhobi/dreamrefactory.git", ENGINE)
        run("git", "-C", ENGINE, "checkout", "--detach", REVISION)
    if (ENGINE / ".git").exists():
        actual = subprocess.check_output(["git", "-C", str(ENGINE), "rev-parse", "HEAD"], text=True).strip()
        if actual != REVISION:
            raise SystemExit(f"Expected engine {REVISION}; found {actual}. Refusing an unqualified upgrade.")
        if subprocess.run(["git", "-C", str(ENGINE), "diff", "--quiet", "HEAD"]).returncode:
            raise SystemExit("The pinned engine has tracked modifications. Restore or separately qualify them before building.")
    else:
        if (ENGINE / "UPSTREAM_REVISION").read_text().strip() != REVISION:
            raise SystemExit("The included engine source has a different revision.")
        for name, expected in json.loads((ENGINE / "UPSTREAM_SOURCE_SHA256.json").read_text()).items():
            if sha(ENGINE / name) != expected:
                raise SystemExit(f"The included engine source changed: {name}")
    if not (ENGINE / "node_modules/.bin/vite").exists():
        run("npm", "ci", "--ignore-scripts", cwd=ENGINE)


def bundle_private_game(resources):
    game = resources / "Game"
    manifest, assets = {}, {}
    for number in (1, 2):
        source = ROOT / f"discs/cd{number}"
        target = game / f"gamefiles/en/titanic{number}"
        print(f"Bundling privately supplied Disc {number}...", flush=True)
        shutil.copytree(source, target, ignore=shutil.ignore_patterns(".*"))
        for file in sorted(target.rglob("*")):
            if file.is_symlink():
                raise SystemExit(f"External game symlink is not portable: {file}")
            if file.is_file():
                relative = file.relative_to(game).as_posix()
                assets[relative] = sha(file)
                if not {"install", "support", "shots", "sneak"}.intersection(p.lower() for p in file.relative_to(target).parts[:-1]):
                    manifest[relative] = file.stat().st_size
    (game / "gamefiles.json").write_text(json.dumps(manifest, separators=(",", ":")))
    seeds = resources / "Initial Saves"
    seeds.mkdir()
    seen = set()
    for folder in (ROOT / ".titanic-restoration/preserved-saves", ROOT / "wineprefix/drive_c/Program Files/CyberFlix/Titanic", ROOT / "saves", ROOT / "wineprefix-staging-11.9/drive_c/Program Files/CyberFlix/Titanic"):
        if not folder.is_dir():
            continue
        for source in sorted(folder.iterdir()):
            if source.suffix.lower() != ".ti" or not source.is_file() or source.name.startswith("."):
                continue
            digest = sha(source)
            if digest in seen:
                continue
            destination = seeds / source.name
            if destination.exists():
                destination = seeds / f"{source.stem} (Recovered {digest[:8]}).ti"
            shutil.copy2(source, destination)
            seen.add(digest)
    return manifest, assets, {p.name: sha(p) for p in seeds.iterdir()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime-only", action="store_true", help="Public build with no game data or saved-game seeds; never installs")
    parser.add_argument("--skip-web", action="store_true")
    parser.add_argument("--install", action="store_true", help="Install a private build in /Applications")
    args = parser.parse_args()
    if args.runtime_only and args.install:
        parser.error("--runtime-only does not install or replace the private app")
    if not args.runtime_only:
        for disc in ("cd1", "cd2"):
            if not (ROOT / "discs" / disc / "DATA").is_dir():
                raise SystemExit(f"Missing private {disc} data. Use --runtime-only for the public app.")
    prepare_engine()
    web_out = NATIVE / "Web" / ("dist-public" if args.runtime_only else "dist")
    if not args.skip_web:
        env = dict(os.environ, TITANIC_RUNTIME_ONLY="1" if args.runtime_only else "0")
        run(ENGINE / "node_modules/.bin/vite", "build", "--config", NATIVE / "Web/vite.config.ts", cwd=ROOT, env=env)
    if not (web_out / "index.html").is_file():
        raise SystemExit("The matching Web build is missing; omit --skip-web.")
    output = ROOT / "dist" / "public" if args.runtime_only else ROOT / "dist"
    output.mkdir(parents=True, exist_ok=True)
    staging = output / f"Titanic-build-{time.strftime('%Y%m%d-%H%M%S')}.app"
    contents = staging / "Contents"
    macos, resources = contents / "MacOS", contents / "Resources"
    macos.mkdir(parents=True)
    resources.mkdir()
    shutil.copytree(web_out, resources / "Web")
    manifest, assets, initial_saves = ({}, {}, {}) if args.runtime_only else bundle_private_game(resources)
    icon = NATIVE / "Resources/Titanic.icns"
    if icon.exists():
        shutil.copy2(icon, resources / "Titanic.icns")
    elif not args.runtime_only:
        icons = output / "Titanic.iconset"
        icons.mkdir(exist_ok=True)
        shutil.copy2(ENGINE / "taoot/public/taoot-mark.png", icons / "icon_128x128.png")
        run("iconutil", "-c", "icns", icons, "-o", resources / "Titanic.icns")
    licenses = resources / "Licenses"
    licenses.mkdir()
    shutil.copy2(ENGINE / "LICENSE", licenses / "dreamREfactory-GPL-3.0.txt")
    shutil.copy2(NATIVE / "THIRD_PARTY.md", licenses / "Third Party.md")
    upstream_files = engine_files()
    upstream_hashes = {p.relative_to(ENGINE).as_posix(): sha(p) for p in upstream_files}
    with tarfile.open(licenses / "dreamREfactory-source.tar.gz", "w:gz") as archive:
        for path in upstream_files:
            archive_bytes(archive, path.relative_to(ENGINE).as_posix(), path.read_bytes())
        archive_bytes(archive, "UPSTREAM_REVISION", (REVISION + "\n").encode())
        archive_bytes(archive, "UPSTREAM_SOURCE_SHA256.json", json.dumps(upstream_hashes, indent=2).encode())
    public_sources = restoration_files()
    with tarfile.open(licenses / "native-restoration-source.tar.gz", "w:gz") as archive:
        for path in public_sources:
            archive_bytes(archive, path.relative_to(ROOT).as_posix(), path.read_bytes(), path.suffix in {".py", ".sh"})
        archive_bytes(archive, "LICENSE", (ENGINE / "LICENSE").read_bytes())
    info = {
        "CFBundleName": "Titanic", "CFBundleDisplayName": "Titanic: Adventure Out of Time",
        "CFBundleIdentifier": "org.titanic.restoration", "CFBundleExecutable": "Titanic",
        "CFBundlePackageType": "APPL", "CFBundleShortVersionString": "0.1.0" if args.runtime_only else "1.0.0",
        "CFBundleVersion": "1", "LSMinimumSystemVersion": "14.0", "NSHighResolutionCapable": True,
        "NSSupportsAutomaticGraphicsSwitching": True,
        "NSHumanReadableCopyright": "Unofficial community runtime. Game data not included in the public release. Engine and integration GPL-3.0.",
        "CFBundleDocumentTypes": [{"CFBundleTypeName": "Titanic Saved Game", "CFBundleTypeRole": "Editor", "CFBundleTypeExtensions": ["ti"], "LSHandlerRank": "Alternate"}],
    }
    if (resources / "Titanic.icns").exists():
        info["CFBundleIconFile"] = "Titanic.icns"
    with (contents / "Info.plist").open("wb") as stream:
        plistlib.dump(info, stream)
    sources = sorted((NATIVE / "Sources").glob("*.swift"))
    builds = []
    for arch in ("arm64", "x86_64"):
        binary = output / f"Titanic-{arch}"
        run("xcrun", "swiftc", "-O", "-parse-as-library", "-module-name", "TitanicRestoration", "-file-prefix-map", f"{ROOT}=.", "-debug-prefix-map", f"{ROOT}=.", "-target", f"{arch}-apple-macos14.0", "-framework", "AppKit", "-framework", "WebKit", *sources, "-o", binary)
        builds.append(binary)
    run("lipo", "-create", *builds, "-output", macos / "Titanic")
    source_hashes = {p.relative_to(ROOT).as_posix(): sha(p) for p in public_sources}
    build_id = hashlib.sha256(json.dumps(source_hashes, sort_keys=True).encode()).hexdigest()
    record = {"mode": "runtime-only" if args.runtime_only else "private-bundled", "engine": REVISION,
              "restorationBuild": build_id, "sourceFiles": source_hashes, "engineSourceFiles": upstream_hashes,
              "builtAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "discFiles": len(assets),
              "manifestFiles": len(manifest), "assets": assets, "initialSaves": initial_saves,
              "saveFormat": "authored-neutral-envelope-with-versioned-state", "bundledGameData": not args.runtime_only}
    (resources / "Build.json").write_text(json.dumps(record, indent=2))
    run("codesign", "--force", "--sign", "-", staging)
    run("codesign", "--verify", "--deep", "--strict", staging)
    if args.runtime_only:
        run("python3", ROOT / "scripts/verify-restored-app.py", "--runtime-only", staging)
    app = output / "Titanic.app"
    if app.exists():
        app.rename(output / f"Titanic-previous-{time.strftime('%Y%m%d-%H%M%S')}.app")
    staging.rename(app)
    if args.install:
        installed = Path("/Applications/Titanic.app")
        if installed.exists():
            installed.rename(output / f"Titanic-installed-backup-{time.strftime('%Y%m%d-%H%M%S')}.app")
        run("ditto", app, installed)
        run("codesign", "--verify", "--deep", "--strict", installed)
        print(installed)
    print(app)


if __name__ == "__main__":
    main()
