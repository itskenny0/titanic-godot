"""Hand-drawn SVG geometry for the small exploration controls.

Dimensions are in the original game's coordinates. Unmatched frames retain
the original artwork. These drawings contain no traced pixels or surface noise.
"""
import math
from xml.sax.saxutils import escape

def lettering(font, text, cx, baseline, width, height, color, arc=0):
    """Outline glyphs, optionally following a circular baseline (no SVG fonts)."""
    from fontTools.pens.svgPathPen import SVGPathPen
    glyphs, cmap = (font.getGlyphSet(), font.getBestCmap())
    names = [cmap[ord(c)] for c in text]
    total = sum((glyphs[n].width for n in names))
    cap = font['OS/2'].sCapHeight if hasattr(font['OS/2'], 'sCapHeight') else font['head'].unitsPerEm * 0.68
    sx, sy = (width / total, height / cap)
    cursor = -width / 2
    paths = []
    for name in names:
        advance = glyphs[name].width * sx
        pen = SVGPathPen(glyphs)
        glyphs[name].draw(pen)
        center = cursor + advance / 2
        if arc:
            angle = center / abs(arc)
            sign = 1 if arc > 0 else -1
            x = cx + abs(arc) * math.sin(angle)
            y = baseline + sign * abs(arc) * (1 - math.cos(angle))
            rotation = math.degrees(angle) * sign
        else:
            x, y, rotation = (cx + center, baseline, 0)
        paths.append(f'<path d="{pen.getCommands()}" transform="translate({x:.4f} {y:.4f}) rotate({rotation:.4f}) translate({-advance / 2:.4f} 0) scale({sx:.6f} {-sy:.6f})"/>')
        cursor += advance
    return f'<g aria-label="{escape(text)}" fill="{color}">' + ''.join(paths) + '</g>'

def gradient(name, colors, vertical=True):
    end = 'x2="0" y2="1"' if vertical else 'x2="1" y2="0"'
    stops = ''.join((f'<stop offset="{i / (len(colors) - 1):.4f}" stop-color="{c}"/>' for i, c in enumerate(colors)))
    return f'<linearGradient id="{name}" x1="0" y1="0" {end}>{stops}</linearGradient>'

def radial(name, stops, cx='.42', cy='.30'):
    points = ''.join((f'<stop offset="{at}" stop-color="{color}"/>' for at, color in stops))
    return f'<radialGradient id="{name}" cx="{cx}" cy="{cy}" r=".72">{points}</radialGradient>'

def life(font):
    ring = 'M60 13 C86 12 104 32 107 56 C111 81 91 105 65 108 C39 112 15 96 11 70 C6 44 30 16 60 13Z M60 33 C44 32 32 45 31 60 C29 77 41 89 57 90 C74 93 88 78 88 62 C88 46 77 33 60 33Z'
    defs = '<radialGradient id="cloth" cx=".5" cy=".5" r=".5"><stop offset=".50" stop-color="#667078"/><stop offset=".61" stop-color="#b5bdba"/><stop offset=".75" stop-color="#e2e2d7"/><stop offset=".86" stop-color="#e4e3d8"/><stop offset=".96" stop-color="#bcc2bf"/><stop offset="1" stop-color="#677270"/></radialGradient>'
    defs += gradient('clothlight', ['#fff9e2', '#d5d9d7', '#707c89'])
    defs += gradient('binding', ['#e9dcc2', '#c6bc9c', '#d0c6a9'])
    defs += f'<clipPath id="cloth-mask"><path d="{ring}" clip-rule="evenodd"/></clipPath>'
    defs += '<clipPath id="backdrop"><rect width="121" height="120"/></clipPath>'
    # The perimeter rope sags between its bindings. A detached, uniformly
    # highlighted circle looked like a pale halo against dark backgrounds.
    rope = ('M20 39 C27 18 43 8 60 8 C80 7 99 23 102 41 '
            'M103 49 C116 69 103 93 90 99 '
            'M81 103 C60 115 32 108 22 93 '
            'M13 80 C3 63 7 48 17 43')
    body = (f'<g id="rope" fill="none" stroke-linecap="round">'
            f'<path d="{rope}" stroke="#383529" stroke-width="2.1"/>'
            f'<path d="{rope}" stroke="#6a604a" stroke-width="1.05"/>'
            '<path d="M22 35 C32 16 47 9 60 8 C75 8 85 15 93 23" '
            'stroke="#8b7c5f" stroke-width=".35" opacity=".65"/></g>')

    body += f'<path d="{ring}" fill="url(#cloth)" fill-rule="evenodd"/>'
    body += f'<path d="{ring}" fill="url(#clothlight)" fill-rule="evenodd" opacity=".23"/>'
    body += '<g clip-path="url(#cloth-mask)">\n<path d="M8 43 L21 37 L36 47 L31 56 L9 53Z M93 36 L109 43 L109 53 L86 57 L80 46Z M10 79 L31 70 L42 84 L28 99Z M78 83 L89 71 L110 81 L97 99Z" fill="url(#binding)" opacity=".94"/>\n<path d="M12 44 L21 38 L35 48 M94 38 L108 45 M13 80 L32 72 M88 73 L106 82" fill="none" stroke="#f6e8c9" stroke-width=".4" opacity=".4"/>\n</g>\n<path d="M32 52 C27 70 38 89 57 91 C75 94 89 79 89 63" fill="none" stroke="#677580" stroke-width=".7" opacity=".65"/>\n'
    letters = lettering(font, 'R.M.S. TITANIC', 60, 27.8, 78, 7.7, '#ac422b', 39)
    letters += lettering(font, 'LIVERPOOL', 60, 100.5, 70, 7.8, '#a3422d', -43)
    body += letters.replace('fill="#ac422b"', 'fill="#ac422b" stroke="#ac422b" stroke-width="28"').replace('fill="#a3422d"', 'fill="#a3422d" stroke="#a3422d" stroke-width="28"')
    return (defs, body)

def bag():
    # A soft, full leather body around a narrow metal mouth. The right gusset
    # folds inward at the top, then rounds out towards the base.
    front = 'M8 18 C11 16 14 18 18 19 L58 31 C61 34 59 39 57 45 C55 58 52 72 48 80 C45 83 37 77 32 74 L5 60 C1 58 1 54 2 48 L5 27 Q5 21 8 18Z'
    side = 'M56 31 C63 31 64 36 64 43 C63 50 68 54 69 63 C70 68 66 71 62 73 L48 82 C45 81 48 76 49 70Z'
    defs = radial('leather', [(0, '#795235'), ('.34', '#634128'), ('.70', '#422918'), ('1', '#24180f')], '.41', '.35')
    defs += gradient('handle', ['#795037', '#593b27', '#2c1c12'])
    defs += gradient('handle-round', ['#b58b67', '#805638', '#51301c'])
    defs += gradient('gusset', ['#69492f', '#19130d', '#302116', '#19120c'], False)
    defs += gradient('mouth', ['#947152', '#68462f', '#352418'])
    defs += gradient('base', ['#563723', '#322014', '#24180f'])
    defs += radial('shoulder', [(0, '#b0855f'), ('.45', '#8b5e3b'), ('1', '#57351e')], '.40', '.15')
    defs += f'<clipPath id="leather-mask"><path d="{front}"/></clipPath>'
    defs += '<filter id="fold-soft" x="-20%" y="-30%" width="140%" height="160%"><feGaussianBlur stdDeviation="1.2"/></filter>'
    body = '''
<path d="M28 22 L27 10 C25 1 43 3 45 11 L45 28" fill="none" stroke="#302319" stroke-width="6.2"/>
<path d="M28 22 L27 10 C25 1 43 3 45 11 L45 28" fill="none" stroke="url(#handle)" stroke-width="4.5"/>
<path d="M27.3 22 L26.3 10 C24.3 1 42.3 3 44.3 11 L44.3 28" fill="none" stroke="url(#handle-round)" stroke-width="2.3"/>
<path d="M26.7 9 C26 4 37 4 41 8 Q44 10 44 15" fill="none" stroke="#c09a75" stroke-width=".6" opacity=".68"/>
<path d="M29 20 L28 11 Q27 8 30 8 M42 12 L43 23" fill="none" stroke="#291a11" stroke-width=".75" opacity=".7"/>
<path d="M10 18 L16 14 L61 29 Q65 33 63 38 L56 40Z" fill="url(#mouth)"/>
<path d="M14 18 L59 32" stroke="#21170f" stroke-width="1.3"/>
<path d="M15 16 L59 30" stroke="#a18160" stroke-width=".6"/>
<path d="M18 25 L17 12 C14 2 32 5 34 14 L35 29" fill="none" stroke="#2c2016" stroke-width="5.8"/>
<path d="M18 25 L17 12 C14 2 32 5 34 14 L35 29" fill="none" stroke="url(#handle)" stroke-width="4.1"/>
<path d="M17.3 25 L16.3 12 C13.3 2 31.3 5 33.3 14 L34.3 29" fill="none" stroke="url(#handle-round)" stroke-width="2.15"/>
<path d="M16 12 C13 5 22 5 28 9 Q31 11 32 14" fill="none" stroke="#c09a75" stroke-width=".55" opacity=".65"/>
<path d="M19 24 L18 13 Q17 10 20 10 M32 15 L33 27" fill="none" stroke="#281a11" stroke-width=".7" opacity=".7"/>
'''
    body += f'<path d="{side}" fill="url(#gusset)"/>'
    body += '''
<path d="M61 36 C61 43 57 48 59 58 Q62 66 62 72 C58 70 56 65 55 59Z" fill="#0f0c09" opacity=".65"/>
<path d="M64 48 C64 55 69 61 67 66 Q66 68 63 69" fill="none" stroke="#765033" stroke-width=".85" opacity=".5"/>
<path d="M50 78 L64 70 Q69 67 68 62" fill="none" stroke="#806141" stroke-width=".6" opacity=".65"/>
'''
    body += f'<path d="{front}" fill="url(#leather)"/>'
    body += '''
<g clip-path="url(#leather-mask)">
<path d="M4 42 C16 46 30 49 42 57 C47 61 48 70 47 80 L0 62Z" fill="url(#base)" opacity=".5"/>
<g filter="url(#fold-soft)" fill="none">
<path d="M8 26 C17 30 34 35 51 40" stroke="#bd8b5d" stroke-width="5" opacity=".17"/>
<path d="M6 37 C19 41 34 47 52 54" stroke="#b78759" stroke-width="4" opacity=".18"/>
<path d="M4 42 C22 46 38 52 51 58" stroke="#160f0b" stroke-width="3.1" opacity=".32"/>
<path d="M5 49 C8 43 9 35 8 28 M44 70 C43 65 46 59 50 54" stroke="#160e09" stroke-width="2" opacity=".3"/>
<path d="M10 51 C12 58 27 66 37 70" stroke="#a37248" stroke-width="6" opacity=".12"/>
</g>
<path d="M7 20 Q11 18 16 21 L56 32 Q60 34 57 39 C41 33 24 27 6 24Z" fill="url(#shoulder)" opacity=".6"/>
</g>
<path d="M8 20 C23 24 40 31 57 34" fill="none" stroke="#b68c64" stroke-width=".65" opacity=".65"/>
<path d="M8 23 C25 28 40 33 56 37" fill="none" stroke="#322116" stroke-width=".6" opacity=".6"/>
<path d="M5 28 C4 38 2 49 3 54 Q3 58 7 60 L44 79 Q47 80 48 77 C52 66 54 52 57 42" fill="none" stroke="#24180f" stroke-width=".95"/>
<path d="M5 30 C5 39 3 50 4 54 Q4 57 8 59 L44 78 M49 75 C52 63 53 52 56 43" fill="none" stroke="#9b7550" stroke-width=".4" opacity=".46"/>
<path d="M59 34 Q62 36 60 41 L57 52" fill="none" stroke="#332617" stroke-width="1.9"/>
<path d="M59 34 Q62 36 60 41 L57 52" fill="none" stroke="#a08453" stroke-width=".7"/>
<path d="M58 43 L57 50" stroke="#d0ad70" stroke-width=".45"/>
'''
    return defs, body


def watch():
    # Round hunter case in the original three-quarter view. Keep the lid seam
    # and left opening lip small; neither changes the case into a pointed shape.
    defs = radial('gold', [(0, '#c0bc80'), ('.35', '#aaa65f'), ('.7', '#969244'), ('1', '#737136')], '.36', '.27')
    defs += '''<linearGradient id="rim" x1=".13" y1="0" x2=".8" y2="1">
<stop offset="0" stop-color="#e0d9a5"/><stop offset=".23" stop-color="#b8b27a"/>
<stop offset=".47" stop-color="#6e7044"/><stop offset=".67" stop-color="#cac18b"/>
<stop offset=".85" stop-color="#8b8552"/><stop offset="1" stop-color="#514c2d"/>
</linearGradient>'''
    defs += gradient('crown', ['#b7ad7b', '#706d48', '#aca173', '#575337'])
    defs += gradient('case-side', ['#8f8758', '#c3b688', '#625b36'])
    body = '''
<ellipse cx="22" cy="31.8" rx="20.1" ry="20.6" fill="url(#case-side)"/>
<path d="M23 12 L24 8 L29 8 L30 13" fill="url(#crown)" stroke="#696642" stroke-width=".5"/>
<ellipse cx="26" cy="5.5" rx="4.3" ry="4.5" fill="none" stroke="#55533a" stroke-width="2.1"/>
<ellipse cx="26" cy="5.1" rx="3.5" ry="3.8" fill="none" stroke="url(#crown)" stroke-width="1.7"/>
<ellipse cx="21.5" cy="30.3" rx="19.7" ry="19.9" fill="url(#rim)"/>
<ellipse cx="21.5" cy="30.3" rx="18.8" ry="19" fill="url(#gold)"/>
<ellipse cx="21.5" cy="30.3" rx="18.25" ry="18.45" fill="none" stroke="#bdb875" stroke-width=".35" opacity=".65"/>
<path d="M3.1 29 C3.5 20 12 12 21.5 11.3" fill="none" stroke="#eee1b0" stroke-width=".65" opacity=".65"/>
<path d="M6.6 42 C11 48 19 50 26 48.5" fill="none" stroke="#cec18c" stroke-width=".6" opacity=".65"/>
<path d="M2 31.1 Q3 30 3.2 31.5 L3.2 34.5 Q2.5 35.2 2 34.7" fill="url(#rim)"/>
<path d="M40.5 29 L41.2 29.4 L41.4 33.8 L40.9 34.3" fill="url(#crown)"/>
<path d="M23 4.8 Q26 4.3 29 5 L28.8 8.2 Q26 8.7 23.3 8Z" fill="url(#crown)"/>
<path d="M24.3 5.3 L24.5 7.9 M26 5.2 L26 8 M27.8 5.3 L27.6 7.9" fill="none" stroke="#67603a" stroke-width=".28"/>
'''
    return defs, body


def scroll():
    # A straight paper cylinder with a loose overlapping sheet and a recessed
    # spiral end. Lighting follows its circumference, not its bounding box.
    defs = '''<linearGradient id="paper" gradientUnits="userSpaceOnUse" x1="0" y1="11.5" x2="0" y2="28.5">
<stop offset="0" stop-color="#bba17c"/><stop offset=".15" stop-color="#eddbbb"/>
<stop offset=".3" stop-color="#e3cfaa"/><stop offset=".52" stop-color="#c9ae86"/>
<stop offset=".72" stop-color="#9d8360"/><stop offset=".88" stop-color="#726248"/>
<stop offset="1" stop-color="#413d2e"/></linearGradient>
<linearGradient id="loose-sheet" x1="0" y1="0" x2="0" y2="1">
<stop offset="0" stop-color="#f4e2c4"/><stop offset=".45" stop-color="#d5bc94"/>
<stop offset="1" stop-color="#9a805b"/></linearGradient>
<radialGradient id="roll-end" cx=".6" cy=".37" r=".8">
<stop offset="0" stop-color="#463b2a"/><stop offset=".35" stop-color="#716046"/>
<stop offset=".72" stop-color="#b9a07a"/><stop offset="1" stop-color="#746247"/></radialGradient>
<linearGradient id="ribbon" x1="0" y1="0" x2="0" y2="1">
<stop offset="0" stop-color="#8a4b2e"/><stop offset=".2" stop-color="#663221"/>
<stop offset=".65" stop-color="#3b2318"/><stop offset="1" stop-color="#211910"/></linearGradient>'''
    body = '''
<g transform="rotate(23 28 19)">
<path d="M2 12.8 C13 12.2 17 12.7 23 12.4 C34 12.2 45 11.6 54.3 11.5 C57.8 11.5 59.1 24.9 54.8 28.1 C44 27.9 32 27.4 22 26.8 L2 25.8 C-.8 22.8 -.6 15.1 2 12.8Z" fill="url(#paper)"/>
<path d="M2.3 13.2 C1.1 15 1.2 18.4 1.4 20 Q2.4 18.1 3.3 17.1 L54.8 16.1 L55.2 13.1Z" fill="#665037" opacity=".35"/>
<path d="M2.3 11.8 C18 13.1 37 12 54.5 11.1 L55.8 13.1 C36 14.2 19 14.8 3.1 13.7 L1.4 15.3Z" fill="url(#loose-sheet)"/>
<path d="M3 11.8 C18 13 37 12 54.5 11.1" fill="none" stroke="#f5e4c8" stroke-width=".55"/>
<path d="M3.3 14.3 C19 15.4 37 14.6 54.8 13.5" fill="none" stroke="#725735" stroke-width=".38" opacity=".65"/>
<path d="M1.8 14.9 C.8 18 .9 22 2 24.3 L3.1 25.5 C1.8 21.8 2.1 19.3 3.6 16.1Z" fill="#c8ae85" opacity=".6"/>
<path d="M22 12.5 C20.7 16.2 21 23.1 22.7 26.8 L25.5 27 C23.5 22.7 23.5 16.2 24.7 12.5Z" fill="#2c2015" opacity=".4"/>
<path d="M21.5 12.5 C20 15.7 20.3 22.6 21.7 26.8 L24.2 26.9 C22.5 22 22.7 16 24 12.5Z" fill="url(#ribbon)"/>
<path d="M22.1 13 C21.1 15.9 21.1 19.1 21.6 21.7" fill="none" stroke="#b67b50" stroke-width=".6" opacity=".65"/>
<ellipse cx="54.5" cy="19.8" rx="3.05" ry="8.15" fill="url(#roll-end)"/>
<path d="M54 11.8 C50.6 13.8 51.4 27 54.5 27.9 C58.2 28.8 58.5 11.7 54.3 11.8 C51.7 11.9 52.1 25.3 54.5 25.7 C57.2 26.1 57 14 54.5 14 C52.8 14 53.2 23.3 54.6 23.7 C56.3 24 56.1 16.1 54.7 16 C53.8 16 54 21.7 54.9 21.8" fill="none" stroke="#cab18a" stroke-width=".55"/>
<path d="M54.6 14.3 C56.7 14.6 56.7 24.3 54.7 25.3 M54.8 16.5 C55.8 17.4 55.9 22.7 54.9 23.1" fill="none" stroke="#4c402d" stroke-width=".45"/>
<path d="M54.3 11.7 C56 11.3 57.1 14.1 57.3 17" fill="none" stroke="#ead5b0" stroke-width=".5"/>
</g>
'''
    return defs, body


def arrow(color):
    defs = gradient('arrow', [color, color, '#434431'])
    body = '<path d="M2 16 L11 13 L20 7 L32 3 L45 7 L54 13 L62 16Z" fill="#282923" stroke="#52564a" stroke-width=".8"/><path d="M15 13 L32 5 L49 13Z" fill="url(#arrow)"/><path d="M19 11 L32 6 L43 11" fill="none" stroke="#f5f0ae" opacity=".5" stroke-width=".65"/>'
    return (defs, body)

def redraw(source, serif):
    """Only verified idle sprites; animations and palette variants use the export."""
    recipes = {'13b16e27cdbeef4e5c51e2470bd9e91f59554ef4d249c3f3d47d6b82c80d91e9': ('life', 121, 120, False), '6e3ba6481cc08d8cb0818d934d0f8014faba9ad2f126d9e3aa2897f63db6776f': ('life', 114, 118, True), '8d2ca16610d0b37e4916f8ce823109def391da1a69c3ff040981001d3e111be5': ('bag', 70, 86, False), 'fbdde50ef60fc9669df130b202e552f2d60247db8e4866d0fe2db769d8b415dd': ('bag', 71, 90, False), '006f12962404cc17b31090096eb402894d12418ad31c47c331a24f8e1e32e5b3': ('watch', 43, 54, False), 'daa19707618b1e238fc087ef7ad6d6ff637c3c7c2aeb87356d1539b128d492b6': ('watch', 45, 55, False), '470b6d0a212843123b174d9318b8fc7f3add3e2076c5badf2ecc1b2181edf0b5': ('scroll', 59, 40, False), '48ad0802a2137fdce714f4bb45ceca16c499870573d37982d993ad1811cfdaa1': ('scroll', 59, 40, False), 'f2f7f5d17df261f9e3c1405125a5dd047e4de2731c041a79ae3e0aff7e119863': ('scroll', 63, 43, False), '65d3abb400c01349c4629b7b8b47687c6b45c2b93dacdb462d80c7ab562c5cd6': ('scroll', 62, 39, False)}
    recipe = recipes.get(source)
    if not recipe:
        return None
    name, w, h, light = recipe
    defs, body = life(serif) if name == 'life' else globals()[name]()
    if name == 'life' and light:
        body = '<g transform="translate(-3 -1) scale(.99)">' + body + '</g>'
    if name == 'bag':
        body = f'<g transform="scale({w / 70:.6f} {h / 86:.6f})">{body}</g>'
    if name == 'scroll':
        body = f'<g transform="scale({w / 59:.6f} {h / 40:.6f})">{body}</g>'
    if name == 'watch':
        body = f'<g transform="scale({w / 43:.6f} {h / 54:.6f})">{body}</g>'
    return (w, h, defs, body)

SUPPORTED = {'006f12962404cc17b31090096eb402894d12418ad31c47c331a24f8e1e32e5b3', '6e3ba6481cc08d8cb0818d934d0f8014faba9ad2f126d9e3aa2897f63db6776f', '8d2ca16610d0b37e4916f8ce823109def391da1a69c3ff040981001d3e111be5', '470b6d0a212843123b174d9318b8fc7f3add3e2076c5badf2ecc1b2181edf0b5', 'f2f7f5d17df261f9e3c1405125a5dd047e4de2731c041a79ae3e0aff7e119863', '13b16e27cdbeef4e5c51e2470bd9e91f59554ef4d249c3f3d47d6b82c80d91e9', '65d3abb400c01349c4629b7b8b47687c6b45c2b93dacdb462d80c7ab562c5cd6', 'fbdde50ef60fc9669df130b202e552f2d60247db8e4866d0fe2db769d8b415dd', '48ad0802a2137fdce714f4bb45ceca16c499870573d37982d993ad1811cfdaa1', 'daa19707618b1e238fc087ef7ad6d6ff637c3c7c2aeb87356d1539b128d492b6'}
