#!/usr/bin/env python3
"""Publish the complete package set from one successful workflow run."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
from urllib.parse import quote


def package_names():
    names = {"titanic-source.tar.gz", "titanic-portmaster.zip", "titanic-android-arm64-release.apk"}
    for arch in ("x86_64", "arm64"):
        names.add(f"titanic-windows-{arch}-portable.zip")
        names.add(f"titanic-mac-{arch}.zip")
        for suffix in (".deb", ".tar.gz", "-static.tar.gz", ".AppImage", ".flatpak"):
            names.add(f"titanic-linux-{arch}{suffix}")
    return names


def digest(path):
    result = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            result.update(block)
    return result.hexdigest()


def collect(directory):
    wanted = package_names()
    found = {}
    # Linux format jobs add standard icon sizes to the already-built debs.
    # Only these explicit replacements supersede the original desktop artifact.
    superseded = set()
    for arch in ("x86_64", "arm64"):
        name = f"titanic-linux-{arch}.deb"
        replacement = directory / f"titanic-linux-formats-{arch}" / name
        if replacement.is_file() and not replacement.is_symlink():
            superseded.add(directory / f"titanic-linux-{arch}" / name)
    for path in directory.rglob("*"):
        if path in superseded or path.is_symlink() or not path.is_file() or path.name not in wanted:
            continue
        if path.stat().st_size == 0:
            raise ValueError(f"Empty package: {path.name}")
        if path.name in found and digest(path) != digest(found[path.name]):
            raise ValueError(f"Conflicting packages: {path.name}")
        found[path.name] = path
    missing = wanted - found.keys()
    if missing:
        raise ValueError("Missing packages: " + ", ".join(sorted(missing)))
    return found


def gh_json(*args):
    return json.loads(subprocess.check_output(["gh", *args], text=True))


def verify_release(repo, tag, source_sha):
    release = gh_json("release", "view", tag, "--repo", repo, "--json", "isDraft,tagName")
    if release["isDraft"] or release["tagName"] != tag:
        raise ValueError("Expected an existing published release")
    obj = gh_json("api", f"repos/{repo}/git/ref/tags/{quote(tag, safe='')}")["object"]
    while obj["type"] == "tag":
        obj = gh_json("api", f"repos/{repo}/git/tags/{obj['sha']}")["object"]
    if obj["type"] != "commit" or obj["sha"] != source_sha:
        raise ValueError("Release tag does not match the source used for these builds")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("artifacts", type=Path)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--source-sha", required=True)
    args = parser.parse_args()
    packages = collect(args.artifacts)
    repo = os.environ["GH_REPO"]
    verify_release(repo, args.tag, args.source_sha)
    with tempfile.TemporaryDirectory(prefix="titanic-release-") as temporary:
        stage = Path(temporary)
        for name, path in packages.items():
            shutil.copyfile(path, stage / name)
        info = {
            "tag": args.tag,
            "source_commit": args.source_sha,
            "workflow_commit": os.environ.get("GITHUB_WORKFLOW_SHA"),
            "workflow_run": f"https://github.com/{repo}/actions/runs/{os.environ['GITHUB_RUN_ID']}",
            "artifact_run": os.environ.get("ARTIFACT_RUN", os.environ["GITHUB_RUN_ID"]),
        }
        (stage / "BUILD-INFO.json").write_text(json.dumps(info, indent=2) + "\n")
        assets = sorted(stage.iterdir())
        checksums = stage / "SHA256SUMS"
        checksums.write_text("".join(f"{digest(path)}  {path.name}\n" for path in assets))
        subprocess.run([
            "gh", "release", "upload", args.tag, "--repo", repo, "--clobber",
            *(str(path) for path in assets), str(checksums),
        ], check=True)
        print(f"Attached {len(packages)} packages, build information and checksums to {args.tag}")


if __name__ == "__main__":
    main()
