#!/usr/bin/env python3
"""Build editable SVG interface masters and an ordinary personal HD pack.

The faithful conversion outlines the original colours and lettering. Optional
control refinement reduces palette noise in that trace. Clean controls instead
use hand-drawn curves and display alpha. Logical coordinates and masks stay
original. Not used by release CI.
"""
import argparse
from concurrent.futures import ProcessPoolExecutor
import hashlib
import io
import json
from pathlib import Path
import re
import struct
import sys
import tempfile
import xml.etree.ElementTree as ET

NS = 'http://www.w3.org/2000/svg'
ET.register_namespace('', NS)
ROOT = Path(__file__).resolve().parents[1]
HOUSE = set('life bag watch lid map hrs min sec credits ok invenctl invenhelp '
            'themetoggle buttons navtoggle save help quit navarrow navblank '
            'subtoggle volume open new keynorth keyeast keywest disable'.split())
STAGES = {'main.stg', 'ctl.stg', 'inven1.stg', 'inven2.stg', 'map.stg', 'tour.stg'}
MOVIES = {'menu.mov', 'playmode.mov', 'playmore.mov', 'credits.mov', 'history.mov',
          'notebook.mov', 'help1m.mov', 'help1w.mov', 'help2m.mov', 'help2w.mov',
          'helptm.mov', 'helptw.mov'}


def is_interface(source):
    if source.get('kind') != 'ui':
        return False
    filename = source['file'].replace('\\', '/').rsplit('/', 1)[-1].lower()
    if filename == 'house.shp':
        return source['name'].split('/')[0].lower() in HOUSE
    return filename == 'inven.shp' or filename in STAGES or filename in MOVIES


def source_key(image):
    return hashlib.sha256(struct.pack('<II', *image.size) + image.convert('RGBA').tobytes()).hexdigest()


def mask_path(alpha):
    """Exact authored cutouts, including enclosed holes, as merged vector runs."""
    w, h = alpha.size
    pixels = alpha.tobytes()
    active, rectangles = {}, []
    for y in range(h + 1):
        runs = set()
        x = 0
        while y < h and x < w:
            if pixels[y*w+x] == 0:
                x += 1
                continue
            start = x
            while x < w and pixels[y*w+x] != 0:
                x += 1
            runs.add((start, x))
        for run in active.keys() - runs:
            start, end = run
            top = active.pop(run)
            rectangles.append(f'M{start} {top}h{end-start}v{y-top}h{start-end}Z')
        for run in runs - active.keys():
            active[run] = y
    return ''.join(rectangles)


def trace(image, temporary, preserve_tones=False, colour_reference=None):
    import vtracer
    # Trace enlarged *original* colours, not AI output or a quantised palette.
    # Speckle filtering would erase punctuation, watch hands and small glyphs.
    from PIL import Image
    if preserve_tones:
        return trace_tones(image, temporary, colour_reference)
    rgb = image.convert('RGB').resize((image.width*4, image.height*4), Image.Resampling.NEAREST)
    rgb.save(temporary / 'source.png')
    vtracer.convert_image_to_svg_py(str(temporary / 'source.png'), str(temporary / 'trace.svg'),
        colormode='color', hierarchical='stacked', mode='spline', filter_speckle=0,
        color_precision=8, layer_difference=1, corner_threshold=90,
        length_threshold=4.0, max_iterations=10, splice_threshold=45, path_precision=2)
    root = ET.parse(temporary / 'trace.svg').getroot()
    group = ET.Element(f'{{{NS}}}g', {'transform': 'scale(.25)'})
    group.extend(root)
    return group


def tone_palette(image, colour_reference=None):
    """Reserve source pigments so tiny details survive material quantisation."""
    import numpy as np
    from PIL import Image

    rgb = image.convert('RGB')
    if colour_reference is None:
        return rgb.quantize(colors=256, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.NONE)
    source = colour_reference.convert('RGB')
    colours = source.getcolors(maxcolors=96)
    if colours is None:
        source = source.quantize(colors=96, method=Image.Quantize.MEDIANCUT,
                                 dither=Image.Dither.NONE).convert('RGB')
        colours = source.getcolors(maxcolors=96)
    reserved = [c for _, c in sorted(colours, reverse=True)]
    material = rgb.quantize(colors=256-len(reserved), method=Image.Quantize.MEDIANCUT,
                            dither=Image.Dither.NONE).convert('RGB')
    palette = list(dict.fromkeys(reserved + [c for _, c in material.getcolors(maxcolors=256)]))
    # Pillow's RGB->palette lookup can map nearby exact source colours to the
    # same cache bucket. Explicit nearest-colour matching preserves pigments.
    unique, inverse = np.unique(np.array(rgb).reshape(-1, 3), axis=0, return_inverse=True)
    colours = np.array(palette, dtype=np.int32)
    nearest = np.empty(len(unique), dtype='uint8')
    for start in range(0, len(unique), 1024):
        delta = unique[start:start+1024, None, :].astype(np.int32)-colours[None, :, :]
        nearest[start:start+1024] = np.sum(delta*delta, axis=2).argmin(axis=1)
    indexed = Image.frombytes('P', rgb.size, nearest[inverse].tobytes())
    indexed.putpalette([v for colour in palette for v in colour] + [0]*(768-len(palette)*3))
    return indexed


def trace_tones(image, temporary, colour_reference=None):
    """Outline each tone independently; colour clustering loses subtle shading.

    VTracer's colour hierarchy can merge a whole low-contrast fold or bevel,
    even at maximum colour precision. An undithered 256-colour palette followed
    by binary contours retains those tones and the original lettering. The tiny
    shared-edge overlap prevents transparent hairlines between adjacent paths.
    """
    import numpy as np
    import vtracer
    from PIL import Image

    indexed = tone_palette(image, colour_reference)
    labels, palette = np.array(indexed), indexed.getpalette()
    group = ET.Element(f'{{{NS}}}g', {'transform': 'scale(.25)'})
    for colour in map(int, np.unique(labels)):
        mask = Image.fromarray(np.where(labels == colour, 0, 255).astype('uint8'))
        mask.resize((image.width*4, image.height*4), Image.Resampling.NEAREST).convert('RGB').save(temporary / 'source.png')
        vtracer.convert_image_to_svg_py(str(temporary / 'source.png'), str(temporary / 'trace.svg'),
            colormode='binary', mode='spline', filter_speckle=0,
            corner_threshold=90, length_threshold=4.0, max_iterations=10,
            splice_threshold=45, path_precision=2)
        fill = '#' + ''.join(f'{c:02x}' for c in palette[colour*3:colour*3+3])
        for path in ET.parse(temporary / 'trace.svg').getroot():
            path.set('fill', fill)
            path.set('stroke', fill)
            path.set('stroke-width', '.4')
            path.set('stroke-linejoin', 'round')
            group.append(path)
    return group


def make_svg(image, entry, key, refine_controls=False):
    w, h = image.size
    root = ET.Element(f'{{{NS}}}svg', {'version': '1.1', 'width': str(w), 'height': str(h),
                      'viewBox': f'0 0 {w} {h}', 'data-source-sha256': key})
    names = sorted({s['file'] + ':' + s['name'] for s in entry['sources'] if is_interface(s)})
    ET.SubElement(root, f'{{{NS}}}title').text = '; '.join(names)
    ET.SubElement(root, f'{{{NS}}}desc').text = 'Outlined artwork and baked-in lettering. Original coordinates and cutouts retained.'
    defs = ET.SubElement(root, f'{{{NS}}}defs')
    clip = ET.SubElement(defs, f'{{{NS}}}clipPath', {'id': 'authored-mask'})
    ET.SubElement(clip, f'{{{NS}}}path', {'d': mask_path(image.getchannel('A'))})
    visible = ET.SubElement(root, f'{{{NS}}}g', {'clip-path': 'url(#authored-mask)'})
    control_groups = sorted({
        s['name'].split('/')[0].lower() for s in entry['sources'] if
        s['file'].replace('\\', '/').rsplit('/', 1)[-1].lower() == 'house.shp'
        and s['name'].split('/')[0].lower() in {'life', 'bag', 'watch', 'lid', 'map'}
    })
    refine = refine_controls and bool(control_groups)
    if refine:
        sys.path.insert(0, str(ROOT / 'tools/hd'))
        from ui_reference import soften_dither
        profile = control_groups[0]
        if profile == 'watch' and any(s['name'].lower() == 'watch/run' for s in entry['sources']):
            profile = 'watch-face'
        reference = soften_dither(image, profile)
        root.set('data-style', 'refined-original')
        root.set('data-detail-profile', profile)
        # A very small blend joins contours without smearing fine lettering,
        # watch rims or handle highlights. Alpha is clipped separately.
        blend = ET.SubElement(defs, f'{{{NS}}}filter', {
            'id': 'contour-blend', 'x': '-2%', 'y': '-2%', 'width': '104%', 'height': '104%',
            'color-interpolation-filters': 'sRGB'})
        ET.SubElement(blend, f'{{{NS}}}feGaussianBlur', {'stdDeviation': '.12'})
        layer = ET.SubElement(visible, f'{{{NS}}}g', {'filter': 'url(#contour-blend)'})
    else:
        root.set('data-style', 'outlined-original')
        reference, layer = image, visible
    with tempfile.TemporaryDirectory() as directory:
        layer.append(trace(reference, Path(directory), preserve_tones=refine,
                           colour_reference=image if refine else None))
    return ET.tostring(root, encoding='utf-8', xml_declaration=True)


def render_svg(data, image):
    import resvg_py
    from PIL import Image
    root = ET.fromstring(data)
    # Edited masters remain self-contained. Never resolve remote fonts/images,
    # scripts or local file references while rendering a downloaded master.
    for node in root.iter():
        tag = node.tag.rsplit('}', 1)[-1]
        if tag not in {'svg', 'g', 'defs', 'title', 'desc', 'metadata', 'path', 'rect',
                       'ellipse', 'circle', 'line', 'polyline', 'polygon', 'clipPath',
                       'linearGradient', 'radialGradient', 'stop', 'filter', 'feGaussianBlur'}:
            raise ValueError('Unsupported SVG element: ' + tag)
        for attr, value in node.attrib.items():
            if attr.rsplit('}', 1)[-1].lower().startswith('on') or 'href' in attr.lower():
                raise ValueError('External or executable SVG attribute')
            if 'url(' in value.lower() and not re.fullmatch(r'url\(#[A-Za-z0-9_-]+\)', value):
                raise ValueError('SVG resource must reference a local definition')
    size = (image.width*2, image.height*2)
    png = resvg_py.svg_to_bytes(svg_string=data.decode('utf-8'), width=size[0], height=size[1], skip_system_fonts=True)
    rendered = Image.open(io.BytesIO(png)).convert('RGBA')
    if root.get('data-style') == 'clean-vector':
        return rendered
    # Runtime sprites deliberately retain original alpha/hit masks. Fill any
    # antialiased clip boundary with source colour to avoid dark edge fringes.
    original = image.resize(size, Image.Resampling.NEAREST)
    fallback = original.copy()
    fallback.putalpha(255)
    rendered = Image.alpha_composite(fallback, rendered)
    rendered.putalpha(original.getchannel('A'))
    return rendered


def convert(task):
    key, entry, source, output, render_only, refine_controls, *options = task
    clean_font = options[0] if options else None
    from PIL import Image
    output = Path(output)
    with Image.open(Path(source) / 'originals' / (key + '.png')) as original:
        image = original.convert('RGBA')
    if image.size != (entry['width'], entry['height']) or source_key(image) != key:
        raise ValueError('Source pixels do not match catalog: ' + key)
    svg = output / 'svg' / (key + '.svg')
    if render_only:
        data = svg.read_bytes()
        root = ET.fromstring(data)
        if root.get('data-source-sha256') != key or root.get('viewBox') != f'0 0 {image.width} {image.height}':
            raise ValueError('Edited SVG has changed source or coordinates: ' + key)
    else:
        if clean_font:
            sys.path.insert(0, str(ROOT / 'tools/hd'))
            from ui_vectors import redraw
            from fontTools.ttLib import TTFont
            with TTFont(clean_font) as font:
                w, h, defs, body = redraw(key, font)
            if (w, h) != image.size:
                raise ValueError('Vector recipe has different source dimensions')
            data = (f'<svg xmlns="{NS}" width="{w}" height="{h}" viewBox="0 0 {w} {h}" '
                    f'data-source-sha256="{key}" data-style="clean-vector">'
                    f'<title>Clean exploration control</title><defs>{defs}</defs>{body}</svg>').encode()
        else:
            data = make_svg(image, entry, key, refine_controls)
        svg.write_bytes(data)
    rendered = render_svg(data, image)
    destination = output / 'images' / (key + '.png')
    rendered.save(destination, compress_level=9)
    blob = destination.read_bytes()
    result = {'width': rendered.width, 'height': rendered.height, 'format': 'png',
              'sha256': hashlib.sha256(blob).hexdigest()}
    if ET.fromstring(data).get('data-style') == 'clean-vector':
        result['sprite_alpha'] = True
    return key, result


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--input', type=Path, required=True, help='hd-pack export with catalog.json and originals')
    p.add_argument('--output', type=Path, required=True, help='new SVG/UI pack directory')
    p.add_argument('--workers', type=int, default=2)
    p.add_argument('--limit', type=int, help='only generate this many images for a preview')
    p.add_argument('--select', action='append', default=[], help='source file:name wildcard, repeatable')
    p.add_argument('--render-only', action='store_true', help='render edited SVGs without retracing')
    p.add_argument('--refine-controls', action='store_true',
                   help='reduce palette noise in traced controls, including their animation frames')
    p.add_argument('--clean-controls', type=Path, metavar='SERIF_TTF',
                   help='draw the supported idle controls with curves and gradients; outline this font for lettering')
    a = p.parse_args()
    if a.clean_controls and (a.refine_controls or not a.clean_controls.is_file()):
        p.error('--clean-controls needs a serif TTF file and cannot be combined with --refine-controls')
    if a.workers < 1 or (a.limit is not None and a.limit < 1):
        p.error('workers and limit must be positive')
    source, output = a.input.resolve(), a.output.resolve()
    # Do not accidentally replace source exports, another HD pack, or game data.
    if output == source or output in source.parents or source in output.parents:
        p.error('use an output directory separate from the source export')
    if output.exists() and not a.render_only:
        p.error('output already exists; choose a new folder, or use --render-only for edited masters')
    if a.render_only and not (output / 'vector-ui.json').is_file():
        p.error('--render-only needs a previous vector-ui output')
    catalog = json.loads((source / 'catalog.json').read_text())['images']
    selected = {k: v for k, v in catalog.items() if any(is_interface(s) for s in v['sources'])}
    if a.select:
        from fnmatch import fnmatchcase
        chosen = set()
        for selector in a.select:
            matches = {k for k, v in selected.items() if k == selector or any(
                fnmatchcase((s['file'] + ':' + s['name']).lower(), selector.lower()) for s in v['sources'])}
            if not matches:
                p.error('no interface artwork matches ' + selector)
            chosen.update(matches)
        selected = {k: v for k, v in selected.items() if k in chosen}
    if a.clean_controls:
        sys.path.insert(0, str(ROOT / 'tools/hd'))
        from ui_vectors import SUPPORTED
        selected = {k: v for k, v in selected.items() if k in SUPPORTED}
    if a.limit:
        selected = dict(list(sorted(selected.items()))[:a.limit])
    if not selected:
        p.error('no interface artwork in this export')
    if a.render_only:
        previous = json.loads((output / 'vector-ui.json').read_text())
        selected = {k: catalog[k] for k in previous['images']}
    for name in ('svg', 'images'):
        (output / name).mkdir(parents=True, exist_ok=True)
    # Invalidate the old pack before a rerender: a failed run must not leave a
    # manifest pointing at a mixture of old and newly rendered checksums.
    (output / 'manifest.json').unlink(missing_ok=True)
    jobs = [(k, v, str(source), str(output), a.render_only,
             a.refine_controls, str(a.clean_controls.resolve()) if a.clean_controls else None) for k, v in selected.items()]
    manifest = {'version': 1, 'scale': 2, 'model': 'SVG interface', 'images': {}}
    with ProcessPoolExecutor(max_workers=a.workers) as pool:
        for n, (key, entry) in enumerate(pool.map(convert, jobs), 1):
            manifest['images'][key] = entry
            if n % 25 == 0 or n == len(jobs):
                print(f'Rendered {n}/{len(jobs)} SVGs', flush=True)
    style = previous['style'] if a.render_only else ('clean-vector' if a.clean_controls else 'refined-original' if a.refine_controls else 'outlined-original')
    if any(v.get('sprite_alpha') for v in manifest['images'].values()):
        manifest['version'] = 2
    index = {'version': 1, 'style': style,
             'images': selected}
    (output / 'vector-ui.json').write_text(json.dumps(index, indent=2) + '\n')
    (output / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(f'SVG masters: {output}/svg\nGame-ready UI pack: {output}', flush=True)


if __name__ == '__main__':
    main()
