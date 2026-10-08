#!/usr/bin/env python3
"""Validate bundled SVG controls and refresh their manifest after editing."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1] / 'godot/artwork/ui'
ALLOWED = {'svg', 'defs', 'g', 'path', 'rect', 'ellipse', 'circle', 'line',
           'polyline', 'polygon', 'linearGradient', 'radialGradient', 'stop',
           'title', 'desc', 'metadata'}


def validate(root=ROOT, check=False):
    path = root / 'manifest.json'
    manifest = json.loads(path.read_text())
    assert manifest['version'] == 2 and manifest['scale'] == 2
    expected = set()
    for key, entry in manifest['images'].items():
        assert re.fullmatch('[0-9a-f]{64}', key), 'Invalid source key'
        assert entry['format'] == 'svg' and entry['sprite_alpha']
        file = root / 'images' / (key + '.svg')
        expected.add(file)
        setting = file.with_suffix('.svg.import')
        expected.add(setting)
        assert setting.read_text() == '[remap]\n\nimporter="keep"\n', 'Keep SVG source in exported packs'
        data = file.read_bytes()
        assert len(data) < 1048576, 'SVG exceeds runtime limit'
        svg = ET.fromstring(data)
        assert svg.get('data-source-sha256') == key, 'Source key changed'
        assert float(svg.get('width')) == entry['width']
        assert float(svg.get('height')) == entry['height']
        assert [float(v) for v in svg.get('viewBox').split()] == [0, 0, entry['width'] / 2, entry['height'] / 2]
        for node in svg.iter():
            name = node.tag.rsplit('}', 1)[-1]
            assert name in ALLOWED, 'Unsupported Godot 3 SVG element: ' + name
            for attr, value in node.attrib.items():
                assert not attr.lower().startswith('on') and 'href' not in attr.lower()
                assert attr not in {'filter', 'clip-path', 'mask'}, 'Bake effects into vector paths'
                assert attr != 'style', 'Use explicit portable SVG attributes'
                if 'url(' in value:
                    assert re.fullmatch(r'url\(#[A-Za-z0-9_-]+\)', value), 'Nonlocal SVG reference'
            if name in {'linearGradient', 'radialGradient'} and node.get('gradientUnits') != 'userSpaceOnUse':
                for attr in ['x1', 'x2', 'y1', 'y2', 'cx', 'cy', 'r', 'fx', 'fy']:
                    assert attr not in node.attrib or node.get(attr).endswith('%'), 'Use percentages for NanoSVG gradients'
        digest = hashlib.sha256(data).hexdigest()
        if check:
            assert entry['sha256'] == digest, 'Run python3 tools/update-svg-ui.py after editing ' + file.name
        entry['sha256'] = digest
    assert set((root / 'images').iterdir()) == expected, 'Only source-matched SVGs belong in images'
    if not check:
        path.write_text(json.dumps(manifest, indent=2) + '\n')
    return len(manifest['images'])


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Validate without writing')
    args = parser.parse_args()
    print(f'{validate(check=args.check)} SVG controls verified')
