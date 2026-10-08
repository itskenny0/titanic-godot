"""Reduce palette noise while retaining the original control artwork.

Flat material can tolerate more filtering than a watch rim or a bag handle.
Connected narrow contours therefore use a gentler pass. Neither pass adds
texture, lettering, geometry or AI detail; the original alpha stays unchanged.
"""


def _bilateral(colours, alpha, colour_sigma, spatial_sigma, passes):
    import numpy as np

    h, w = alpha.shape
    opaque = np.pad(alpha, 1, mode='edge')
    for _ in range(passes):
        padded = np.pad(colours, ((1, 1), (1, 1), (0, 0)), mode='edge')
        result = np.zeros_like(colours)
        weights = np.zeros((h, w, 1), dtype=np.float32)
        for dy in range(-1, 2):
            for dx in range(-1, 2):
                neighbour = padded[1+dy:1+dy+h, 1+dx:1+dx+w]
                valid = opaque[1+dy:1+dy+h, 1+dx:1+dx+w] > 0
                difference = np.sum((colours-neighbour)**2, axis=2)
                weight = (np.exp(-difference/(2*colour_sigma**2)
                                 - (dx*dx+dy*dy)/(2*spatial_sigma**2))*valid)[:, :, None]
                result += neighbour*weight
                weights += weight
        colours = np.divide(result, weights, out=colours.copy(), where=weights > 0)
    return colours


def _contours(colours, alpha):
    """Find thin continuous features, rather than isolated palette speckles."""
    import numpy as np
    from PIL import Image, ImageFilter

    h, w = alpha.shape
    padded = np.pad(colours, ((2, 2), (2, 2), (0, 0)), mode='edge')

    def shifted(dx, dy):
        return padded[2+dy:2+dy+h, 2+dx:2+dx+w]

    def distance(other):
        return np.sqrt(np.mean((colours-other)**2, axis=2))

    features = np.zeros((h, w), dtype=np.float32)
    for dx, dy in ((1, 0), (0, 1), (1, 1), (1, -1)):
        # A ridge has similar colours along its length and different colours
        # on BOTH sides. Looking two pixels across also catches thicker rims.
        along = np.maximum(distance(shifted(dx, dy)), distance(shifted(-dx, -dy)))
        across = np.minimum(distance(shifted(-2*dy, 2*dx)), distance(shifted(2*dy, -2*dx)))
        continuity = np.clip((15-along)/10, 0, 1)
        contrast = np.clip((across-8)/12, 0, 1)
        features = np.maximum(features, continuity*contrast)

    occupied = np.pad(alpha > 0, 1, mode='edge')
    edge = np.zeros((h, w), dtype=bool)
    for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
        edge |= (alpha > 0) != occupied[1+dy:1+dy+h, 1+dx:1+dx+w]
    features = np.maximum(features, edge)
    # Protect a contour's neighbours too, avoiding a sharp/soft seam around it.
    expanded = Image.fromarray((features*255).astype('uint8'))
    expanded = expanded.filter(ImageFilter.MaxFilter(3)).filter(ImageFilter.GaussianBlur(.45))
    return np.maximum(np.array(expanded, dtype=np.float32)/255, edge)[:, :, None]


def control_details(image, group):
    """Hand-placed preservation regions; these never draw replacement geometry.

    Coordinates follow the source controls and scale with their hover/opening
    frames. The source still supplies every colour, stroke and transparent hole.
    """
    import numpy as np
    from PIL import Image, ImageDraw, ImageFilter

    w, h = image.size
    mask = Image.new('L', (w*4, h*4))
    numeral_band = None
    draw = ImageDraw.Draw(mask)

    def polygon(points):
        draw.polygon([(x*w*4, y*h*4) for x, y in points], fill=255)

    def line(points, width):
        draw.line([(x*w*4, y*h*4) for x, y in points], fill=255,
                  width=max(1, round(width*min(w, h)*4)), joint='curve')

    if group in {'watch', 'watch-face'}:
        # Bow, winding stem and the narrow metal rim around the case.
        polygon([(0, 0), (1, 0), (1, .24), (0, .24)])
        if group == 'watch-face':
            # The open watch has a different perspective. Preserve the case
            # bevel, dial border, minute marks and Roman numeral band, while
            # still cleaning the empty centre of the dial.
            draw.ellipse((w*.04*4, h*.24*4, w*.87*4, h*.96*4),
                         outline=255, width=max(4, round(w*.055*4)))
            draw.ellipse((w*.115*4, h*.275*4, w*.81*4, h*.885*4),
                         outline=255, width=max(4, round(w*.025*4)))
            numeral_band = Image.new('L', mask.size)
            ImageDraw.Draw(numeral_band).ellipse(
                (w*.16*4, h*.31*4, w*.77*4, h*.835*4),
                outline=255, width=max(4, round(w*.085*4)))
        else:
            draw.ellipse((w*.01*4, h*.17*4, w*.99*4, h*.97*4),
                         outline=255, width=max(4, round(w*.065*4)))
    elif group == 'bag':
        # Both handles, their attachments, the top seam and the side fastening.
        polygon([(0, 0), (1, 0), (1, .32), (0, .32)])
        line([(.03, .22), (.27, .28), (.55, .34), (.85, .4)], .06)
        line([(.83, .39), (.81, .61)], .065)
        line([(.25, .22), (.29, .3)], .08)
        line([(.52, .24), (.57, .34)], .065)
    elif group == 'map':
        # Rolled ends, binding and the thin highlight along the paper fold.
        polygon([(0, 0), (.2, 0), (.23, .46), (.02, .5)])
        polygon([(.78, .46), (1, .44), (1, 1), (.85, .92)])
        line([(.25, .21), (.23, .55)], .14)
        line([(.1, .16), (.42, .39), (.89, .63)], .08)
    elif group == 'life':
        # Rope and the narrow inner rim. Broad fabric panels retain denoising.
        draw.ellipse((w*.025*4, h*.015*4, w*.98*4, h*.98*4),
                     outline=255, width=max(4, round(w*.075*4)))
        draw.ellipse((w*.255*4, h*.25*4, w*.775*4, h*.795*4),
                     outline=255, width=max(4, round(w*.025*4)))
    mask = mask.resize((w, h), Image.Resampling.LANCZOS)
    if numeral_band is not None:
        # The normal dial uses neutral grey ink. Preserve the actual marks
        # and their antialiasing, not all the palette noise between numerals.
        rgb = np.array(image.convert('RGB'), dtype=np.float32)
        dial = Image.new('L', (w, h))
        ImageDraw.Draw(dial).ellipse((w*.12, h*.28, w*.82, h*.89), fill=255)
        ink = ((rgb.max(axis=2)-rgb.min(axis=2) < 8)
               & (rgb.mean(axis=2) < 130) & (np.array(dial) > 0))
        if ink.sum() >= 8:
            numerals = Image.fromarray((ink*255).astype('uint8')).filter(ImageFilter.MaxFilter(3))
        else:
            # Alternate palettes may tint the ink. Preserve the whole numeral
            # band instead of mistaking those marks for material texture.
            numerals = numeral_band.resize((w, h), Image.Resampling.LANCZOS)
        mask = Image.fromarray(np.maximum(np.array(mask), np.array(numerals)))
    if group == 'life':
        # Locate actual ink, including its serifs and antialiasing. No font
        # substitution, redrawn letters or hard-coded text positions.
        rgba = np.array(image.convert('RGBA'), dtype=np.float32)
        red = ((rgba[:, :, 0] > rgba[:, :, 1]*1.4)
               & (rgba[:, :, 0] > rgba[:, :, 2]*1.55)
               & (rgba[:, :, 0] > 70) & (rgba[:, :, 3] > 0))
        ink = Image.fromarray((red*255).astype('uint8')).filter(ImageFilter.MaxFilter(3))
        mask = Image.fromarray(np.maximum(np.array(mask), np.array(ink)))
    elif group == 'lid':
        alpha = image.convert('RGBA').getchannel('A')
        rim = np.array(alpha).astype('int16') - np.array(alpha.filter(ImageFilter.MinFilter(5)))
        mask = Image.fromarray(np.maximum(np.array(mask), rim).astype('uint8'))
    return np.array(mask.filter(ImageFilter.GaussianBlur(.35)), dtype=np.float32)[:, :, None]/255


def soften_dither(image, group=None):
    import numpy as np
    from PIL import Image

    rgba = np.array(image.convert('RGBA'), dtype=np.float32)
    colours, alpha = rgba[:, :, :3], rgba[:, :, 3]
    fine = _bilateral(colours, alpha, colour_sigma=38, spatial_sigma=.7, passes=1)
    material = _bilateral(colours, alpha, colour_sigma=48, spatial_sigma=1, passes=2)
    contours = _contours(colours, alpha)
    refined = material*(1-contours) + fine*contours
    if group:
        # The generic contour detector misses curved handle attachments and
        # small fittings. Preserve them with an almost pixel-exact fine pass.
        detail = _bilateral(colours, alpha, colour_sigma=18, spatial_sigma=.45, passes=1)
        protected = control_details(image, group)
        refined = refined*(1-protected) + detail*protected
    rgba[:, :, :3] = refined
    return Image.fromarray(np.rint(rgba).astype('uint8'))
