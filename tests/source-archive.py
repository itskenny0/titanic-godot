import io
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

TOOL = Path(__file__).resolve().parents[1] / 'tools/verify-source-archive.py'

class SourceArchiveTests(unittest.TestCase):
    def test_tagged_source_is_required(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            (root / 'source.go').write_bytes(b'package main\n')
            subprocess.run(['git', '-C', str(root), 'add', 'source.go'], check=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Test', '-c', 'user.email=test@example.com', 'commit', '-qm', 'fixture'], check=True)
            for content, extra, expected in [(b'package main\n', False, 0), (b'different source\n', False, 1), (b'package main\n', True, 1)]:
                with tarfile.open(root / 'source.tar.gz', 'w:gz') as archive:
                    for name, data in [('source.go', content)] + ([('extra.go', b'extra')] if extra else []):
                        member = tarfile.TarInfo('titanic-godot/' + name)
                        member.size = len(data)
                        archive.addfile(member, io.BytesIO(data))
                result = subprocess.run(['python3', str(TOOL), 'source.tar.gz', '--ref', 'HEAD'], cwd=root, capture_output=True)
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)

if __name__ == '__main__':
    unittest.main()
