#!/usr/bin/env python3
"""Validate locally supplied Titanic data and prepare two importable disc folders.

SPDX-License-Identifier: GPL-3.0-or-later
No network access or Windows executable execution is used by this script.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
INDEX = ROOT / "godot/required_files.json"
GOG_PROFILE = ROOT / "tools/gog-local-profile.json"
KEYS = {
    1: ("data/bootfile", "data/bedsit1.set", "data/main.stg", "data/ctl.stg"),
    2: ("data/a14.set", "data/deckbd.set", "data/cargo.set"),
}
EXCLUDED = {"install", "support", "shots", "sneak", ".git", "node_modules"}


class PreparationError(Exception):
    pass


def requirements() -> dict[int, tuple[str, ...]]:
    """Use the app's checked-in names-only index as the single source of truth."""
    result = {int(disc): tuple(paths) for disc, paths in json.loads(INDEX.read_text()).items()}
    if set(result) != {1, 2} or any(not result[d] for d in result):
        raise PreparationError("The app's required-file index is missing or invalid.")
    for paths in result.values():
        if len(paths) != len(set(paths)) or any(
            p != p.lower() or p.startswith("/") or ".." in Path(p).parts for p in paths
        ):
            raise PreparationError("The app's required-file index contains unsafe paths.")
    return result


def children(directory: Path) -> dict[str, Path]:
    result = {}
    for path in sorted(directory.iterdir()):
        if path.name.startswith("."):
            continue
        key = path.name.lower()
        if key in result:
            raise PreparationError(f"Ambiguous case-insensitive filename: {path}")
        result[key] = path
    return result


def resolve_file(root: Path, relative: str) -> Path | None:
    current = root
    for part in relative.split("/"):
        if not current.is_dir():
            return None
        found = children(current).get(part)
        if found is None:
            return None
        if found.is_symlink():
            raise PreparationError(f"Symbolic links are not supported in game data: {found}")
        current = found
    return current if current.is_file() else None


def checked_root(path: Path) -> Path:
    path = path.expanduser().absolute()
    if path.is_symlink() or not path.is_dir():
        raise PreparationError(f"Select an extracted folder, not a file or symbolic link: {path}")
    return path.resolve()


def discover(sources: list[Path]) -> dict[int, Path] | Path:
    """Recognize data content, not folder labels; never merge disc identities."""
    candidates = {1: set(), 2: set()}
    visited: set[Path] = set()
    digital_roots: set[Path] = set()
    for source in sources:
        root = checked_root(source)
        stack = [(root, 0)]
        while stack:
            directory, depth = stack.pop()
            if directory in visited:
                continue
            visited.add(directory)
            if len(visited) > 10000:
                raise PreparationError("Too many folders to inspect. Select the game's folder directly.")
            entries = children(directory)
            if directory.name.lower() == "local" and {"bootfile", "bedsit1.set"} <= entries.keys():
                digital_roots.add(directory)
            if resolve_file(directory, "data/bootfile") and resolve_file(directory, "data/bedsit1.set"):
                candidates[1].add(directory)
            if resolve_file(directory, "data/a14.set") and resolve_file(directory, "data/cargo.set"):
                candidates[2].add(directory)
            if depth < 6:
                stack.extend((p, depth + 1) for name, p in entries.items()
                             if name not in EXCLUDED and p.is_dir() and not p.is_symlink())
    if any(len(candidates[d]) > 1 for d in candidates):
        raise PreparationError("Multiple disc candidates found. Select exact roots with --disc1 and --disc2.")
    if not all(candidates.values()):
        if len(digital_roots) == 1:
            return next(iter(digital_roots))
        if digital_roots:
            raise PreparationError("Multiple LOCAL installations found. Select the intended game's folder directly.")
        raise PreparationError("Neither both English disc folders nor a recognized digital LOCAL folder were found. Disc images must first be extracted; installed digital files must retain their LOCAL folder.")
    result = {d: next(iter(paths)) for d, paths in candidates.items()}
    if result[1] == result[2]:
        raise PreparationError("One merged DATA folder cannot establish two distinct disc versions. Supply separate disc roots.")
    return result


def validate_header(path: Path) -> None:
    size = path.stat().st_size
    with path.open("rb") as file:
        header = file.read(32)
    valid = size >= 1024 and len(header) == 32 and any(
        struct.unpack_from(order + "I", header, 4)[0] == size
        and 0 < struct.unpack_from(order + "I", header, 20)[0] <= (size - 1024) // 4
        for order in ("<", ">")
    )
    if not valid:
        raise PreparationError(f"Damaged or truncated DreamFactory container: {path}")


def check_digital_file(path: Path, expected: dict) -> None:
    if path.stat().st_size != expected["size"]:
        raise PreparationError(f"Digital file differs from the supported GOG profile (size): {path.name}. This edition or modified file needs compatibility validation.")
    with path.open("rb") as stream:
        for chunk in expected["chunks"]:
            data = stream.read(chunk["size"])
            if len(data) != chunk["size"] or hashlib.md5(data).hexdigest() != chunk["md5"]:
                raise PreparationError(f"Digital file differs from the supported GOG profile (checksum): {path.name}. Restore unmodified purchased files or report the edition/version.")
        if stream.read(1):
            raise PreparationError(f"Unexpected trailing data in digital file: {path.name}")


def digital_inventory(root: Path) -> tuple[dict[int, dict[str, Path]], dict]:
    profile = json.loads(GOG_PROFILE.read_text())
    required = requirements()
    basenames = {Path(relative).name for paths in required.values() for relative in paths}
    if set(profile["files"]) != basenames:
        raise PreparationError("The checked-in digital profile does not match the app's file index.")
    entries = children(root)
    missing = sorted(basenames - entries.keys())
    if missing:
        raise PreparationError(f"Digital LOCAL folder is incomplete: {len(missing)} files missing: " + ", ".join(missing[:12]))
    for name in sorted(basenames):
        path = entries[name]
        if path.is_symlink() or not path.is_file():
            raise PreparationError(f"Digital game files must be regular files: {path}")
        check_digital_file(path, profile["files"][name])
    files = {disc: {relative: entries[Path(relative).name] for relative in paths}
             for disc, paths in required.items()}
    for disc, keys in KEYS.items():
        for relative in keys:
            validate_header(files[disc][relative])
    return files, {"status": "validated", "layout": "gog-local", "source": str(root),
                   "profileBuild": profile["buildId"], "profileVersion": profile["version"],
                   "checksumsVerified": True, "uniqueSourceFiles": len(basenames),
                   "files": sum(len(paths) for paths in files.values()),
                   "bytes": sum(p.stat().st_size for paths in files.values() for p in paths.values())}


def inventory(roots: dict[int, Path] | Path) -> tuple[dict[int, dict[str, Path]], dict]:
    if isinstance(roots, Path):
        return digital_inventory(roots)
    required = requirements()
    files: dict[int, dict[str, Path]] = {}
    discs = []
    for disc in (1, 2):
        files[disc] = {}
        missing = []
        for relative in required[disc]:
            path = resolve_file(roots[disc], relative)
            if path is None or path.stat().st_size == 0:
                missing.append(relative)
            else:
                files[disc][relative] = path
        if missing:
            raise PreparationError(f"Disc {disc} is incomplete: {len(missing)} required files missing or empty: "
                                   + ", ".join(missing[:12]) + (" …" if len(missing) > 12 else ""))
        for relative in KEYS[disc]:
            validate_header(files[disc][relative])
        discs.append({"disc": disc, "source": str(roots[disc]), "files": len(files[disc]),
                      "bytes": sum(p.stat().st_size for p in files[disc].values())})
    return files, {"status": "validated", "layout": "english-two-disc", "discs": discs,
                   "files": sum(d["files"] for d in discs), "bytes": sum(d["bytes"] for d in discs)}


def publish(files: dict[int, dict[str, Path]], output: Path, *, digital: bool = False) -> None:
    """Copy to our own staging directory and publish only after validation."""
    if output.exists() or output.is_symlink():
        raise PreparationError(f"Output already exists; choose a new folder: {output}")
    if not output.parent.is_dir():
        raise PreparationError(f"Output parent does not exist: {output.parent}")
    profile = json.loads(GOG_PROFILE.read_text())["files"] if digital else None
    with tempfile.TemporaryDirectory(prefix=".titanic-prepare-", dir=output.parent) as staging:
        stage = Path(staging) / "prepared"
        stage.mkdir()
        for disc, paths in files.items():
            for relative, source in paths.items():
                destination = stage / f"cd{disc}" / relative
                destination.parent.mkdir(parents=True, exist_ok=True)
                before = source.stat()
                shutil.copyfile(source, destination)
                after = source.stat()
                if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns) or destination.stat().st_size != before.st_size:
                    raise PreparationError(f"Source changed during preparation; try again: {source}")
                if profile is not None:
                    check_digital_file(destination, profile[Path(relative).name])
        inventory({1: stage / "cd1", 2: stage / "cd2"})
        if output.exists() or output.is_symlink():
            raise PreparationError(f"Output appeared during preparation; refusing to replace it: {output}")
        stage.rename(output)


def extraction_command(executable: str, installer: Path, output: Path) -> list[str]:
    return [executable, "--extract", "--test", "--no-extract-unknown", "--gog",
            "--output-dir", str(output), "--", str(installer)]


def run(args: argparse.Namespace) -> dict:
    output = args.output.expanduser().absolute() if args.output else None
    if output and (output.exists() or output.is_symlink()):
        raise PreparationError(f"Output already exists; choose a new folder: {output}")
    if output:
        output = output.parent.resolve() / output.name
    if not args.dry_run and output is None:
        raise PreparationError("Choose --output for preparation, or --dry-run to validate without writing.")
    if args.installer:
        installer = args.installer.expanduser().resolve()
        if not installer.is_file() or installer.suffix.lower() != ".exe":
            raise PreparationError("--installer requires the GOG offline setup .exe, with all matching .bin parts beside it.")
        if args.dry_run:
            return {"status": "extraction-not-run", "compatibility": "unverified",
                    "message": "Dry run does not extract or validate the installer's game files. Current store packaging may not provide the two-disc layout.",
                    "command": extraction_command("innoextract", installer, Path("<temporary-directory>"))}
        executable = shutil.which("innoextract")
        if not executable:
            raise PreparationError("innoextract is not installed. Install it yourself from https://constexpr.org/innoextract/ or supply already extracted disc folders. This tool never runs a Windows installer.")
        with tempfile.TemporaryDirectory(prefix="titanic-installer-") as temporary:
            extraction = Path(temporary)
            result = subprocess.run(extraction_command(executable, installer, extraction),
                                    stdout=sys.stderr, stderr=sys.stderr, check=False)
            if result.returncode:
                raise PreparationError(f"innoextract failed (exit {result.returncode}). Keep all installer parts together; this installer version may not be supported. No prepared output was published.")
            roots = discover([extraction])
            files, report = inventory(roots)
            publish(files, output, digital=report["layout"] == "gog-local")
            report["installer"] = str(installer)
    else:
        if args.disc1 or args.disc2:
            if not args.disc1 or not args.disc2 or args.source:
                raise PreparationError("Use both --disc1 and --disc2 together, or use --source.")
            roots = {1: checked_root(args.disc1), 2: checked_root(args.disc2)}
            if roots[1] == roots[2] or roots[1] in roots[2].parents or roots[2] in roots[1].parents:
                raise PreparationError("Disc roots must be separate folders.")
        else:
            roots = discover(args.source or [])
        source_roots = [roots] if isinstance(roots, Path) else roots.values()
        if output and any(output == p or p in output.parents or output in p.parents for p in source_roots):
            raise PreparationError("Output must be outside the supplied disc folders.")
        files, report = inventory(roots)
        if not args.dry_run:
            publish(files, output, digital=report["layout"] == "gog-local")
    if not args.dry_run:
        report.update(status="prepared", output=str(output), next_step="In the Godot player choose the prepared folder containing cd1 and cd2.")
    return report


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--source", type=Path, action="append", help="Extracted folder containing both discs or the supported digital LOCAL folder; repeat for two disc roots.")
    parser.add_argument("--disc1", type=Path, help="Exact original Disc 1 root (use with --disc2).")
    parser.add_argument("--disc2", type=Path, help="Exact original Disc 2 root (use with --disc1).")
    parser.add_argument("--installer", type=Path, help="Optionally extract a user-supplied GOG offline .exe using installed innoextract. Actual purchased package compatibility is unverified.")
    parser.add_argument("--output", type=Path, help="New destination folder; existing data is never replaced.")
    parser.add_argument("--dry-run", action="store_true", help="Validate folders without writing; installer mode only prints the extraction plan.")
    parser.add_argument("--json", action="store_true", help="Emit one machine-readable result object; errors exit 2.")
    args = parser.parse_args(argv)
    try:
        if args.installer and (args.source or args.disc1 or args.disc2):
            raise PreparationError("Choose --installer or extracted folder options, not both.")
        if not (args.installer or args.source or args.disc1 or args.disc2):
            raise PreparationError("Supply --source, both --disc1 and --disc2, or --installer.")
        report = run(args)
    except (PreparationError, OSError) as error:
        report = {"status": "error", "error": str(error)}
        print(json.dumps(report, indent=2) if args.json else f"Error: {error}")
        return 2
    if args.json:
        print(json.dumps(report, indent=2))
    elif "files" in report:
        print(f"{report['status'].capitalize()}: {report['files']} files, {report['bytes']:,} bytes across both discs.")
        print(report.get("next_step", "Validation passed. No files were written."))
        if output := report.get("output"):
            print(output)
    else:
        print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
