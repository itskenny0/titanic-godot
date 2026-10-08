#!/usr/bin/env python3
"""SVG pipeline checks using synthetic art, never commercial game fixtures."""
import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest
import xml.etree.ElementTree as ET

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('vector_ui', ROOT / 'tools/vector-ui.py')
ui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ui)


class VectorUITest(unittest.TestCase):
    def test_clean_vectors_keep_smooth_transparency_and_real_cutouts(self):
        # An opaque source must not reappear under a vector's transparent edge
        # or hole. That fallback was responsible for the old grey/pixel fringe.
        source = Image.new('RGBA', (16, 16), (90, 95, 90, 255))
        svg = (f'<svg xmlns="{ui.NS}" width="16" height="16" viewBox="0 0 16 16" data-style="clean-vector">'
               '<path d="M8 1.3 A6.7 6.7 0 1 1 7.99 1.3Z M8 4 A4 4 0 1 0 8.01 4Z" '
               'fill="#b7462e" fill-rule="evenodd"/></svg>').encode()
        rendered = ui.render_svg(svg, source)
        alpha = rendered.getchannel('A')
        self.assertEqual(rendered.getpixel((16, 16))[3], 0, 'source filled the ring hole')
        self.assertEqual(rendered.getpixel((0, 0))[3], 0, 'source matte returned')
        self.assertTrue(any(0 < a < 255 for a in alpha.tobytes()), 'curved edge lost antialiasing')
        legacy = ui.render_svg(svg.replace(b' data-style="clean-vector"', b''), source)
        self.assertEqual(set(legacy.getchannel('A').tobytes()), {255}, 'legacy mask semantics changed')

    def test_refinement_removes_dither_without_inventing_texture(self):
        import sys
        import numpy as np
        sys.path.insert(0, str(ROOT / 'tools/hd'))
        from ui_reference import soften_dither
        flat = Image.new('RGBA', (24, 24), (120, 105, 90, 255))
        self.assertEqual(soften_dither(flat).tobytes(), flat.tobytes())
        pixels = flat.load()
        for y in range(24):
            for x in range(24):
                v = 105 if (x+y) % 2 else 135
                pixels[x, y] = (v, v, v, 255)
        # Include a hole and isolated dark punctuation beside the dither field.
        draw = ImageDraw.Draw(flat)
        draw.rectangle((3, 3, 7, 7), fill=(0, 0, 0, 0))
        draw.point((19, 3), fill=(0, 0, 0, 255))
        refined = soften_dither(flat)
        self.assertEqual(refined.getchannel('A').tobytes(), flat.getchannel('A').tobytes())
        self.assertLess(max(refined.getpixel((19, 3))[:3]), 5)
        self.assertLess(np.array(refined)[12:22, 2:22, 0].var(), np.array(flat)[12:22, 2:22, 0].var()/4)

    def test_ui_scope_excludes_shared_shop_world_artwork(self):
        def source(file, name, kind='ui'):
            return {'file': file, 'name': name, 'kind': kind}
        self.assertTrue(ui.is_interface(source('LOCAL/HOUSE.SHP', 'Life/dark')))
        self.assertTrue(ui.is_interface(source('LOCAL/map.stg', 'Map 1')))
        self.assertTrue(ui.is_interface(source('LOCAL/help1w.mov', 'segment0/frame0')))
        self.assertFalse(ui.is_interface(source('LOCAL/house.shp', 'door/open')))
        self.assertFalse(ui.is_interface(source('LOCAL/house.shp', 'signs/boatdeck')))
        self.assertFalse(ui.is_interface(source('LOCAL/enigma.stg', 'main')))
        self.assertFalse(ui.is_interface(source('LOCAL/house.shp', 'life/dark', 'world')))

    def test_refinement_preserves_thin_rims_and_handle_highlights(self):
        import sys
        import numpy as np
        sys.path.insert(0, str(ROOT / 'tools/hd'))
        from ui_reference import soften_dither
        image = Image.new('RGBA', (32, 24), (65, 47, 32, 255))
        draw = ImageDraw.Draw(image)
        # A narrow, low-contrast brass rim and a curved handle. These are real
        # structures, unlike alternating isolated dither pixels.
        draw.line((3, 18, 28, 18), fill=(85, 67, 52, 255), width=1)
        draw.arc((8, 2, 24, 16), 180, 360, fill=(90, 72, 57, 255), width=2)
        before = np.array(image, dtype=float)
        after = np.array(soften_dither(image), dtype=float)
        rim = after[18, 5:27, :3].mean() - after[21, 5:27, :3].mean()
        self.assertGreater(rim, 12, 'a thin continuous rim was washed out')
        highlights = before[:, :, 0] == 90
        self.assertLess(abs(after[:, :, :3][highlights] - before[:, :, :3][highlights]).mean(), 6)

    def test_vector_contours_keep_continuous_shading(self):
        import numpy as np
        # The colour tracer used to merge most of this subdued leather-like
        # gradient into flat panels, even with layer_difference=0.
        pixels = np.empty((24, 64, 4), dtype='uint8')
        for x in range(64):
            pixels[:, x] = (40+x, 25+x//2, 18+x//3, 255)
        image = Image.fromarray(pixels)
        root = ET.Element(f'{{{ui.NS}}}svg', {'width': '64', 'height': '24', 'viewBox': '0 0 64 24'})
        with tempfile.TemporaryDirectory() as directory:
            root.append(ui.trace(image, Path(directory), preserve_tones=True))
        result = ui.render_svg(ET.tostring(root), image).resize(image.size, Image.Resampling.BOX)
        error = abs(np.array(result, dtype=float)[2:-2, 2:-2, :3] - pixels[2:-2, 2:-2, :3])
        self.assertLess(error.mean(), 1.5, 'vector conversion flattened the shading')
        self.assertLess(np.quantile(error, .95), 3)

    def test_small_watch_fitting_keeps_its_hole_and_highlight(self):
        import sys
        import numpy as np
        sys.path.insert(0, str(ROOT / 'tools/hd'))
        from ui_reference import soften_dither
        image = Image.new('RGBA', (43, 54), (70, 65, 42, 255))
        draw = ImageDraw.Draw(image)
        draw.ellipse((19, 1, 28, 8), outline=(105, 100, 64, 255), width=1)
        draw.line((22, 1, 25, 1), fill=(132, 126, 82, 255))
        draw.line((23, 8, 23, 12), fill=(116, 109, 70, 255), width=2)
        refined = soften_dither(image, 'watch')
        before = np.array(image, dtype=float)[0:13, 18:30, :3]
        after = np.array(refined, dtype=float)[0:13, 18:30, :3]
        self.assertLess(abs(after-before).mean(), 1.5, 'the small metal fitting was softened')
        self.assertLess(max(refined.getpixel((23, 4))[:3]), 75, 'the loop hole filled in')
        self.assertGreater(refined.getpixel((23, 1))[0], 127, 'the tiny highlight faded')

    def test_source_pigments_survive_a_complex_material_palette(self):
        import numpy as np
        # Lots of new material shades should not consume the colours needed
        # for a few small, subtly different engraving/lettering pixels.
        material = np.empty((64, 64, 3), dtype='uint8')
        for y in range(64):
            for x in range(64):
                material[y, x] = (70+x, 45+y, 30+(x+y)//2)
        source = Image.new('RGB', (64, 64), (86, 71, 42))
        for x, colour in enumerate(((84, 60, 38), (86, 62, 39), (89, 63, 41))):
            source.putpixel((x, 0), colour)
            material[0, x] = colour
        converted = ui.tone_palette(Image.fromarray(material), source).convert('RGB')
        for x in range(3):
            self.assertEqual(converted.getpixel((x, 0)), source.getpixel((x, 0)))

    def test_watch_dial_keeps_ink_while_cleaning_empty_material(self):
        import sys
        sys.path.insert(0, str(ROOT / 'tools/hd'))
        from ui_reference import soften_dither
        for ink in ((104, 104, 104, 255), (100, 70, 60, 255)):
            with self.subTest(ink=ink):
                image = Image.new('RGBA', (104, 109), (152, 156, 134, 255))
                draw = ImageDraw.Draw(image)
                draw.line((43, 37, 43, 43), fill=ink)
                # Isolated pale dither in the dial centre is not engraving.
                draw.point((50, 55), fill=(173, 177, 176, 255))
                result = soften_dither(image, 'watch-face')
                self.assertLess(max(abs(a-b) for a, b in zip(result.getpixel((43, 40)), ink)), 3)
                self.assertLess(result.getpixel((50, 55))[2], 155)

    def test_cutouts_small_lettering_and_editable_paths(self):
        image = Image.new('RGBA', (32, 24), '#dedbc9')
        draw = ImageDraw.Draw(image)
        draw.rectangle((2, 2, 11, 14), fill='#91532b')
        draw.rectangle((4, 4, 9, 12), fill=(0, 0, 0, 0))
        # A tiny capital H and punctuation must not disappear as speckles.
        draw.line((17, 3, 17, 10), fill='black')
        draw.line((21, 3, 21, 10), fill='black')
        draw.line((17, 6, 21, 6), fill='black')
        draw.point((24, 10), fill='black')
        key = ui.source_key(image)
        entry = {'width': 32, 'height': 24, 'sources': [
            {'file': 'LOCAL/house.shp', 'kind': 'ui', 'name': 'life/dark'}]}
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            source, output = base/'source', base/'pack'
            for path in (source/'originals', output/'svg', output/'images'):
                path.mkdir(parents=True)
            image.save(source/'originals'/f'{key}.png')
            _, manifest = ui.convert((key, entry, str(source), str(output), False, True))
            data = (output/'svg'/f'{key}.svg').read_bytes()
            root = ET.fromstring(data)
            tags = {n.tag.rsplit('}', 1)[-1] for n in root.iter()}
            self.assertNotIn('image', tags)
            self.assertNotIn('text', tags)
            self.assertIn('path', tags)
            rendered = Image.open(output/'images'/f'{key}.png').convert('RGBA')
            self.assertEqual(rendered.size, (64, 48))
            self.assertEqual(rendered.getchannel('A').tobytes(), image.getchannel('A').resize((64, 48), Image.Resampling.NEAREST).tobytes())
            self.assertLess(max(rendered.getpixel((48, 20))[:3]), 70, 'punctuation was erased')
            self.assertLess(max(rendered.getpixel((34, 8))[:3]), 70, 'letter stroke was erased')
            self.assertEqual(manifest['sha256'], hashlib.sha256((output/'images'/f'{key}.png').read_bytes()).hexdigest())
            # Edited SVGs rerender, but cannot silently switch source images.
            root.set('data-source-sha256', '0'*64)
            (output/'svg'/f'{key}.svg').write_bytes(ET.tostring(root))
            with self.assertRaisesRegex(ValueError, 'changed source'):
                ui.convert((key, entry, str(source), str(output), True, None))
            image.putpixel((0, 0), (1, 2, 3, 255))
            image.save(source/'originals'/f'{key}.png')
            with self.assertRaisesRegex(ValueError, 'do not match catalog'):
                ui.convert((key, entry, str(source), str(output), False, None))

    def test_svg_has_no_external_resources(self):
        image = Image.new('RGBA', (2, 2))
        for body in ('<image href="file:///tmp/secret"/>', '<script/>',
                     '<path fill="url(https://example.test/x)"/>', '<text>font-dependent</text>'):
            with self.assertRaises(ValueError):
                ui.render_svg(f'<svg xmlns="{ui.NS}">{body}</svg>'.encode(), image)


if __name__ == '__main__':
    unittest.main()
