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
from types import SimpleNamespace
from unittest.mock import patch

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
                       '--workers', '1']
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
            # A UI-only pack needs no GPU or model even through the serial MPS
            # path, and changing execution device can reuse completed images.
            subprocess.run(command+['--device', 'mps'], check=True, capture_output=True, timeout=30)
            self.assertEqual(output.stat().st_mtime_ns, modified)

    def test_unavailable_metal_is_reported_instead_of_silently_using_cpu(self):
        torch = SimpleNamespace(backends=SimpleNamespace(mps=SimpleNamespace(is_available=lambda: False)))
        with patch.dict(sys.modules, {'torch': torch}):
            with self.assertRaisesRegex(ValueError, 'Metal .* unavailable'):
                upscaler.check_device('mps', True)
            upscaler.check_device('cpu', True)
            upscaler.check_device('mps', False)

    def test_metal_rejects_parallel_gpu_model_copies(self):
        result = subprocess.run([sys.executable, str(SCRIPT), '--device', 'mps', '--workers', '4'],
                                capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 2)
        self.assertIn('uses one process', result.stderr)

    def test_selection_is_specific_and_rejects_mistakes(self):
        images = {
            'a'*64: {'sources': [{'file': 'LOCAL/HOUSE.SHP', 'name': 'life/light'}]},
            'b'*64: {'sources': [{'file': 'cd2/house.shp', 'name': 'bag/light'}]},
        }
        self.assertEqual(upscaler.nearest_keys(images, ['*/house.shp:life/*']), {'a'*64})
        self.assertEqual(upscaler.nearest_keys(images, ['B'*64]), {'b'*64})
        with self.assertRaisesRegex(ValueError, 'No artwork matches'):
            upscaler.nearest_keys(images, ['*/house.shp:typo/*'])

    def test_default_covers_ui_but_keeps_world_closeups_and_characters_on_ai(self):
        def asset(file, kind='ui'):
            return {'sources': [{'file': file, 'kind': kind, 'name': 'example'}]}
        images = {
            'house': asset('LOCAL/HOUSE.SHP'),
            'inventory': asset('cd2/inven.shp'),
            'panel': asset('LOCAL/main.stg'),
            'map': asset('LOCAL/map.stg'),
            'menu': asset('LOCAL/playmode.mov'),
            'puzzle': asset('LOCAL/enigma.stg'),
            'closeup': asset('LOCAL/gsdome.mov'),
            'room': asset('LOCAL/gstair3.set', 'room'),
            'character': asset('LOCAL/penny2.pup', 'character'),
        }
        self.assertEqual(upscaler.ui_keys(images), {'house', 'inventory', 'panel', 'map', 'menu'})

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
