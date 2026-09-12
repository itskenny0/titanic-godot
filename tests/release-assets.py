import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("release_assets", Path(__file__).parents[1] / "tools/release-assets.py")
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseAssetsTests(unittest.TestCase):
    def test_only_complete_downloads_are_selected(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for name in release.package_names():
                (directory / name).write_bytes(b"package")
            (directory / "runtime-linux-x86_64").write_bytes(b"internal runtime")
            (directory / "titanic.pck").write_bytes(b"internal pack")
            self.assertEqual(len(release.collect(directory)), 17)
            (directory / "titanic-portmaster.zip").unlink()
            with self.assertRaisesRegex(ValueError, "Missing packages: titanic-portmaster.zip"):
                release.collect(directory)

    def test_conflicting_duplicates_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for name in release.package_names():
                (directory / name).write_bytes(b"package")
            other = directory / "other"
            other.mkdir()
            (other / "titanic-source.tar.gz").write_bytes(b"different source")
            with self.assertRaisesRegex(ValueError, "Conflicting packages"):
                release.collect(directory)

    def test_annotated_tag_is_resolved_to_commit(self):
        with patch.object(release, "gh_json", side_effect=[
            {"isDraft": False, "tagName": "v0.1.0"},
            {"object": {"type": "tag", "sha": "annotation"}},
            {"object": {"type": "commit", "sha": "source"}},
        ]):
            release.verify_release("owner/repo", "v0.1.0", "source")

    def test_wrong_source_cannot_be_published(self):
        with patch.object(release, "gh_json", side_effect=[
            {"isDraft": False, "tagName": "v0.1.0"},
            {"object": {"type": "commit", "sha": "old-source"}},
        ]):
            with self.assertRaisesRegex(ValueError, "does not match"):
                release.verify_release("owner/repo", "v0.1.0", "new-source")


if __name__ == "__main__":
    unittest.main()
