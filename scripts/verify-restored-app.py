#!/usr/bin/env python3
"""Verify a Mac app's signature, architecture, source archives and asset boundary."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import plistlib
import re
import subprocess
import tarfile

GAME_SUFFIXES = {".ti", ".exe", ".dll", ".iso", ".bin", ".pup", ".trk", ".11k", ".set", ".stg", ".mov", ".shp", ".cst", ".sbk", ".wav", ".aif", ".bmp"}
PRIVATE_PARTS = {"collection", "gamefiles", "preserved-saves", "initial saves", "screenshots", "logs", "wineprefix", "wineprefix-staging-11.9"}
PERSONAL_PATH = re.compile(rb"/Users/[A-Za-z0-9_.@-]+/")


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def public_content(name, data):
    path = PurePosixPath(name)
    assert not path.is_absolute() and ".." not in path.parts, f"Unsafe path: {name}"
    assert path.suffix.lower() not in GAME_SUFFIXES, f"Game/save payload: {name}"
    assert not PRIVATE_PARTS.intersection(p.lower() for p in path.parts), f"Private/game directory: {name}"
    assert path.name not in {"RESTORATION.md", "cursor-art.ts", "save-roundtrip-check.test.ts", "save-extension.test.ts"}, f"Private or extracted fixture source: {name}"
    assert not PERSONAL_PATH.search(data), f"Personal absolute path in {name}"
    assert data[32:40] not in {b"LPPALPPA", b"ODTRTRFD"}, f"Original game/save container disguised as {name}"
    if path.suffix.lower() in {".png", ".jpg", ".jpeg", ".gif", ".icns", ".pdf"}:
        assert name in {"Contents/Resources/Titanic.icns", "native/restoration/Resources/Titanic.icns", "native/restoration/Resources/Titanic.png"}, f"Unexpected image/manual asset: {name}"


def source_archive(path, expected, public):
    with tarfile.open(path, "r:gz") as archive:
        members = archive.getmembers()
        assert all(member.isfile() for member in members), f"Links/non-files in {path.name}"
        names = [member.name for member in members]
        assert len(names) == len(set(names)), f"Duplicate archive paths in {path.name}"
        if public:
            metadata = {"UPSTREAM_REVISION", "UPSTREAM_SOURCE_SHA256.json"} if path.name.startswith("dreamREfactory") else {"LICENSE"}
            assert set(names) == set(expected) | metadata, f"Unexpected or missing files in {path.name}"
        for member in members:
            data = archive.extractfile(member).read()
            if public:
                public_content(member.name, data)
                assert "public" not in PurePosixPath(member.name).parts and "assets" not in PurePosixPath(member.name).parts, f"Upstream media directory in source archive: {member.name}"
            if member.name in expected:
                assert hashlib.sha256(data).hexdigest() == expected[member.name], f"Source hash mismatch: {member.name}"
        assert set(expected).issubset(names), f"Missing corresponding sources in {path.name}"
    return len(names)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("app", nargs="?", default="/Applications/Titanic.app")
    parser.add_argument("--runtime-only", action="store_true", help="Require an entirely game-data-free public bundle")
    args = parser.parse_args()
    app = Path(args.app).resolve()
    resources = app / "Contents/Resources"
    record = json.loads((resources / "Build.json").read_text())
    public = record.get("mode") == "runtime-only"
    if args.runtime_only:
        assert public, "This is not a runtime-only public build"
    for path in app.rglob("*"):
        assert not path.is_symlink(), f"Bundle contains symlink: {path}"
        if public and path.is_file():
            public_content(path.relative_to(app).as_posix(), path.read_bytes())
    if public:
        assert record["assets"] == {} and record["initialSaves"] == {}, "Public build records game data"
        assert record["discFiles"] == 0 and record["manifestFiles"] == 0
        assert record["bundledGameData"] is False
        assert not (resources / "Game").exists() and not (resources / "Initial Saves").exists()
        manifest = {}
    else:
        for relative, expected in record["assets"].items():
            assert digest(resources / "Game" / relative) == expected, f"Modified disc asset: {relative}"
        for name, expected in record["initialSaves"].items():
            assert digest(resources / "Initial Saves" / name) == expected, f"Modified initial save: {name}"
        manifest = json.loads((resources / "Game/gamefiles.json").read_text())
        assert any("titanic1/" in name for name in manifest) and any("titanic2/" in name for name in manifest)
        for name, size in manifest.items():
            assert (resources / "Game" / name).stat().st_size == size
    info = plistlib.loads((app / "Contents/Info.plist").read_bytes())
    assert info["CFBundleIdentifier"] == "org.titanic.restoration"
    binary = app / "Contents/MacOS" / info["CFBundleExecutable"]
    subprocess.run(["codesign", "--verify", "--deep", "--strict", str(app)], check=True)
    dependency_output = subprocess.check_output(["otool", "-L", str(binary)], text=True)
    dependencies = [line.strip().split(" (", 1)[0] for line in dependency_output.splitlines() if line.startswith("\t")]
    assert dependencies and all(name.startswith(("/System/Library/", "/usr/lib/")) for name in dependencies), dependencies
    architectures = subprocess.check_output(["lipo", "-archs", str(binary)], text=True).strip()
    assert "arm64" in architectures and "x86_64" in architectures
    web_entry = (resources / "Web/index.html").read_text()
    assert "http://" not in web_entry and "https://" not in web_entry
    licenses = resources / "Licenses"
    for name in ("dreamREfactory-GPL-3.0.txt", "dreamREfactory-source.tar.gz", "native-restoration-source.tar.gz", "Third Party.md"):
        assert (licenses / name).is_file()
    archive_counts = {
        "engine": source_archive(licenses / "dreamREfactory-source.tar.gz", record.get("engineSourceFiles", {}), public),
        "restoration": source_archive(licenses / "native-restoration-source.tar.gz", record.get("sourceFiles", {}), public),
    }
    print(json.dumps({"app": str(app), "mode": record.get("mode", "private-bundled"), "architectures": architectures,
                      "discFilesVerified": len(record["assets"]), "runtimeFilesVerified": len(manifest),
                      "savedGameSeedsVerified": len(record["initialSaves"]), "sourceFilesVerified": archive_counts,
                      "signature": "valid ad-hoc signature", "externalDependencies": "system frameworks only",
                      "engine": record["engine"], "publicAssetBoundary": "passed" if public else "private bundle"}, indent=2))


if __name__ == "__main__":
    main()
