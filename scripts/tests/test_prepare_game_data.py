"""Portable setup checks; fixtures are authored bytes, not game assets."""

import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import struct
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "prepare-game-data.py"
spec = importlib.util.spec_from_file_location("prepare_game_data", SCRIPT)
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class PreparationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.source = self.root / "purchased" / "app" / "game"
        self.output = self.root / "prepared"
        self.requirements = prepare.requirements()
        for disc, paths in self.requirements.items():
            for relative in paths:
                path = self.source / f"DISC{disc}" / relative.upper()
                path.parent.mkdir(parents=True, exist_ok=True)
                data = bytearray(1032)
                struct.pack_into("<I", data, 4, len(data))
                struct.pack_into("<I", data, 20, 1)
                data[-1] = disc  # Prove same-name disc variants are preserved.
                path.write_bytes(data)

    def tearDown(self):
        self.temporary.cleanup()

    def invoke(self, *arguments):
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = prepare.main([*map(str, arguments), "--json"])
        return code, json.loads(output.getvalue())

    def test_nested_case_insensitive_discovery_dry_run_and_exact_copy(self):
        self.assertEqual([len(self.requirements[d]) for d in (1, 2)], [213, 323])
        args = ["--source", self.root / "purchased", "--output", self.output]
        code, result = self.invoke(*args, "--dry-run")
        self.assertEqual((code, result["status"], result["files"]), (0, "validated", 536))
        self.assertFalse(self.output.exists())
        code, result = self.invoke(*args)
        self.assertEqual((code, result["status"]), (0, "prepared"))
        for disc, paths in self.requirements.items():
            for relative in paths:
                self.assertEqual((self.output / f"cd{disc}" / relative).read_bytes(),
                                 (self.source / f"DISC{disc}" / relative.upper()).read_bytes())
        self.assertNotEqual((self.output / "cd1/data/cafe.set").read_bytes(),
                            (self.output / "cd2/data/cafe.set").read_bytes())

    def test_missing_and_truncated_inputs_never_publish(self):
        source_file = self.source / "DISC2/FUSE/FUSE.SND"
        source_file.unlink()
        code, result = self.invoke("--source", self.source, "--output", self.output)
        self.assertEqual(code, 2)
        self.assertIn("fuse.snd", result["error"])
        self.assertFalse(self.output.exists())
        source_file.write_bytes(b"authored fixture")
        (self.source / "DISC1/DATA/MAIN.STG").write_bytes(b"truncated")
        code, result = self.invoke("--source", self.source, "--output", self.output)
        self.assertEqual(code, 2)
        self.assertIn("truncated", result["error"])
        self.assertFalse(self.output.exists())

    def test_merged_and_ambiguous_disc_roots_are_not_guessed(self):
        merged = self.root / "merged"
        shutil.copytree(self.source / "DISC1", merged)
        shutil.copytree(self.source / "DISC2", merged, dirs_exist_ok=True)
        code, result = self.invoke("--source", merged, "--dry-run")
        self.assertEqual(code, 2)
        self.assertIn("merged", result["error"])
        shutil.copytree(self.source / "DISC1", self.source / "ANOTHER_DISC1")
        code, result = self.invoke("--source", self.source, "--dry-run")
        self.assertEqual(code, 2)
        self.assertIn("Multiple", result["error"])
        code, result = self.invoke("--disc1", self.source / "DISC1", "--disc2", self.source / "DISC2", "--dry-run")
        self.assertEqual(code, 0)

    def test_symlink_and_existing_output_are_rejected(self):
        original = self.source / "DISC2/FUSE/FUSE.SND"
        original.unlink()
        original.symlink_to(self.source / "DISC1/DATA/MAIN.STG")
        code, result = self.invoke("--source", self.source, "--output", self.output)
        self.assertEqual(code, 2)
        self.assertIn("Symbolic", result["error"])
        self.output.mkdir()
        (self.output / "keep.txt").write_text("existing work")
        code, result = self.invoke("--source", self.source, "--output", self.output)
        self.assertEqual(code, 2)
        self.assertEqual((self.output / "keep.txt").read_text(), "existing work")

    def test_installer_dry_run_is_explicitly_unvalidated_and_never_executes(self):
        installer = self.root / "setup_titanic_fixture.exe"
        installer.write_bytes(b"not an executable")
        with patch.object(prepare.subprocess, "run", side_effect=AssertionError("must not execute")):
            code, result = self.invoke("--installer", installer, "--dry-run")
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "extraction-not-run")
        self.assertEqual(result["compatibility"], "unverified")
        self.assertEqual(result["command"][0], "innoextract")
        self.assertIn("--test", result["command"])
        self.assertIn("--no-extract-unknown", result["command"])
        self.assertIn("--gog", result["command"])
        self.assertEqual(result["command"][-2:], ["--", str(installer.resolve())])

    def test_digital_profile_maps_every_destination_and_rejects_modified_bytes(self):
        local = self.root / "digital" / "LOCAL"
        local.mkdir(parents=True)
        profile = {"buildId": "authored-test", "version": "fixture", "files": {}}
        basenames = {Path(p).name for paths in self.requirements.values() for p in paths}
        for name in basenames:
            data = bytearray(1032)
            struct.pack_into("<I", data, 4, len(data))
            struct.pack_into("<I", data, 20, 1)
            data[-1] = 7
            (local / name).write_bytes(data)
            profile["files"][name] = {"size": len(data), "chunks": [
                {"size": len(chunk), "md5": hashlib.md5(chunk).hexdigest()}
                for chunk in (data[:1000], data[1000:])
            ]}
        profile_file = self.root / "profile.json"
        profile_file.write_text(json.dumps(profile))
        with patch.object(prepare, "GOG_PROFILE", profile_file):
            real_copy = prepare.shutil.copyfile

            def damaged_copy(source, destination):
                result = real_copy(source, destination)
                if destination.name == "cafe.set":
                    damaged = bytearray(destination.read_bytes())
                    damaged[-1] ^= 1
                    destination.write_bytes(damaged)
                return result

            with patch.object(prepare.shutil, "copyfile", side_effect=damaged_copy):
                code, report = self.invoke("--source", local.parent, "--output", self.output)
            self.assertEqual(code, 2)
            self.assertIn("checksum", report["error"])
            self.assertFalse(self.output.exists())
            code, report = self.invoke("--source", local.parent, "--output", self.output)
            self.assertEqual((code, report["layout"], report["files"]), (0, "gog-local", 536))
            self.assertTrue(report["checksumsVerified"])
            for disc, paths in self.requirements.items():
                for relative in paths:
                    self.assertEqual((self.output / f"cd{disc}" / relative).read_bytes(),
                                     (local / Path(relative).name).read_bytes())
            changed = bytearray((local / "cafe.set").read_bytes())
            changed[-1] ^= 1
            (local / "cafe.set").write_bytes(changed)
            code, report = self.invoke("--source", local, "--dry-run")
            self.assertEqual(code, 2)
            self.assertIn("checksum", report["error"])

    def test_output_inside_source_is_rejected_even_with_system_path_alias(self):
        code, report = self.invoke("--source", self.source, "--output", self.source / "DISC1" / "prepared")
        self.assertEqual(code, 2)
        self.assertIn("outside", report["error"])


if __name__ == "__main__":
    unittest.main()
