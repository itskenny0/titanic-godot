"""Check exact-pixel overrides and safe resuming without downloading an AI model."""
import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest

from PIL import Image

SCRIPT = Path(__file__).resolve().parents[1] / 'tools/upscale-hd.py'
spec = importlib.util.spec_from_file_location('upscale_hd', SCRIPT)
upscaler = importlib.util.module_from_spec(spec)
spec.loader.exec_module(upscaler)


class NearestTests(unittest.TestCase):
    def test_cli_preserves_every_rgba_pixel_and_needs_no_weights(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root/'originals').mkdir()
            # An enclosed transparent pixel, hidden RGB, and partial alpha.
            pixels = bytes([200, 120, 5, 255] * 4 + [17, 29, 31, 0] +
                           [60, 90, 120, 128] + [200, 120, 5, 255] * 3)
            source = Image.frombytes('RGBA', (3, 3), pixels)
            key = hashlib.sha256(struct.pack('<II', 3, 3)+pixels).hexdigest()
            source.save(root/'originals'/f'{key}.png')
            catalog = {'version': 1, 'images': {key: {
                'width': 3, 'height': 3, 'sources': [
                    {'file': 'CD1/HOUSE.SHP', 'name': 'life/light', 'kind': 'ui'}]}}}
            (root/'catalog.json').write_text(json.dumps(catalog))
            command = [sys.executable, str(SCRIPT), '--input', str(root),
                       '--output', str(root/'pack'), '--models', str(root/'models'),
                       '--workers', '1', '--nearest', '*/house.shp:life/*']
            subprocess.run(command, check=True, capture_output=True, timeout=30)
            manifest = json.loads((root/'pack/manifest.json').read_text())
            entry = manifest['images'][key]
            output = root/'pack/images'/f'{key}.{entry["format"]}'
            with Image.open(output) as result:
                self.assertEqual(result.size, (6, 6))
                self.assertEqual(result.convert('RGBA').tobytes(),
                                 source.resize((6, 6), Image.Resampling.NEAREST).tobytes())
            self.assertEqual(entry['sha256'], hashlib.sha256(output.read_bytes()).hexdigest())
            self.assertEqual(manifest['nearest'], [key])
            self.assertEqual(list((root/'models').iterdir()), [])
            modified = output.stat().st_mtime_ns
            subprocess.run(command, check=True, capture_output=True, timeout=30)
            self.assertEqual(output.stat().st_mtime_ns, modified)

    def test_selection_is_specific_and_rejects_mistakes(self):
        images = {
            'a'*64: {'sources': [{'file': 'LOCAL/HOUSE.SHP', 'name': 'life/light'}]},
            'b'*64: {'sources': [{'file': 'cd2/house.shp', 'name': 'bag/light'}]},
        }
        self.assertEqual(upscaler.nearest_keys(images, ['*/house.shp:life/*']), {'a'*64})
        self.assertEqual(upscaler.nearest_keys(images, ['B'*64]), {'b'*64})
        with self.assertRaisesRegex(ValueError, 'No artwork matches'):
            upscaler.nearest_keys(images, ['*/house.shp:typo/*'])

    def test_changing_selection_invalidates_only_affected_png_and_webp(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            original = {'model': 'example', 'scale': 2}
            upscaler.prepare_output(root, original)
            for key in ('a'*64, 'b'*64):
                for suffix in ('.png', '.webp'):
                    (root/'images'/(key+suffix)).write_bytes(b'previous output')
            (root/'manifest.json').write_text('{}')
            nearest = dict(original, nearest=['a'*64])
            upscaler.prepare_output(root, nearest)
            self.assertFalse((root/'manifest.json').exists())
            for suffix in ('.png', '.webp'):
                self.assertFalse((root/'images'/('a'*64+suffix)).exists())
                self.assertTrue((root/'images'/('b'*64+suffix)).exists())
                (root/'images'/('a'*64+suffix)).write_bytes(b'nearest output')
            upscaler.prepare_output(root, original)
            for suffix in ('.png', '.webp'):
                self.assertFalse((root/'images'/('a'*64+suffix)).exists())
                self.assertTrue((root/'images'/('b'*64+suffix)).exists())
            with self.assertRaisesRegex(ValueError, 'different generation settings'):
                upscaler.prepare_output(root, dict(original, model='other'))


if __name__ == '__main__':
    unittest.main()
