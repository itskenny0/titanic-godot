package engine

import (
	"hash/maphash"
	"image"
	"math"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/hdpack"
)

// HDSurface is a display-only companion to the original screen. Scripts,
// occlusion, photos and savegames always use the untouched logical screen.
type HDSurface struct {
	Pack           *hdpack.Reader
	Pixels         []byte
	Width, Height  int
	Valid          bool
	Hits           uint64
	FrameStartHits uint64
	spriteKeys     map[spriteKeyID]string
	// pixelKeys remembers the artwork key of pictures already hashed, found
	// again by a cheap fingerprint instead of SHA-256 on every repaint.
	pixelKeys map[pixelKeyID]string
	seed      maphash.Seed
	// While the room is moving, pictures are only taken from the cache: one
	// that is not decoded yet would arrive after its frame has gone.
	moving, hashed    int
	late, shown, slow int
	motionOff         bool
	frameKeys         map[spriteKeyID]string
	keep              string
	keepW, keepH      int
	convert           []byte
}
type pixelKeyID struct {
	w, h int
	sum  uint64
}

// A sprite's artwork key depends only on its decoded pixels and the palette
// it is drawn with. Decoded sprites are immutable, so the SHA-256 is computed
// once per pairing instead of on every repaint.
type spriteKeyID struct {
	sprite  *df.Sprite
	palette uint64
}

func NewHDSurface(pack *hdpack.Reader, w, h int) *HDSurface {
	return &HDSurface{Pack: pack, Width: w, Height: h, Pixels: make([]byte, w*h*4*hdpack.Scale*hdpack.Scale), seed: maphash.MakeSeed()}
}

// pixelKey is hdpack.Key, remembered per distinct picture.
func (s *HDSurface) pixelKey(src []byte, w, h int) string {
	id := pixelKeyID{w, h, maphash.Bytes(s.seed, src)}
	if key, ok := s.pixelKeys[id]; ok {
		return key
	}
	if s.pixelKeys == nil || len(s.pixelKeys) >= 1024 {
		s.pixelKeys = map[pixelKeyID]string{}
	}
	key := hdpack.Key(w, h, src)
	s.hashed++
	s.pixelKeys[id] = key
	return key
}

// Service adopts artwork decoded in the background. It reports whether the
// screen should repaint to show it.
func (s *HDSurface) Service() bool {
	if s.moving > 0 {
		if s.moving--; s.moving == 0 {
			s.endMovement()
		}
	}
	return s.Pack.Collect()
}

// Moving is called by the room on every tick of a walk or turn.
func (s *HDSurface) Moving() { s.moving = 2 }

// A device that cannot decode movement artwork as fast as it is shown would
// flicker between original and replacement frames. After two movements that
// were mostly late, movement keeps the original artwork for this session.
func (s *HDSurface) endMovement() {
	if s.late > s.shown {
		if s.slow++; s.slow >= 2 {
			s.motionOff = true
		}
	} else if s.shown > 0 {
		s.slow = 0
	}
	s.late, s.shown = 0, 0
}
func (s *HDSurface) frameKey(f *CachedFrame, palette []byte) (string, bool) {
	if f == nil || f.Width < 1 || f.Height < 1 || f.Width > 512 || f.Height > 384 || len(f.Pixels) < f.Width*f.Height || !s.Pack.HasSize(f.Width, f.Height) {
		return "", false
	}
	id := spriteKeyID{palette: f.ID ^ maphash.Bytes(s.seed, palette)}
	if key, ok := s.frameKeys[id]; ok {
		return key, true
	}
	n := f.Width * f.Height * 4
	if len(s.convert) < n {
		s.convert = make([]byte, n)
	}
	if err := df.IndexedRGBA(f.Pixels[:f.Width*f.Height], palette, s.convert[:n]); err != nil {
		return "", false
	}
	if s.frameKeys == nil || len(s.frameKeys) >= 1024 {
		s.frameKeys = map[spriteKeyID]string{}
	}
	key := s.pixelKey(s.convert[:n], f.Width, f.Height)
	s.frameKeys[id] = key
	return key, true
}

// WarmLanding starts decoding the picture a walk or turn will stop on, and
// keeps it cached until it is shown. It reports false while every worker is
// busy, so the caller asks again.
func (s *HDSurface) WarmLanding(f *CachedFrame, palette []byte) bool {
	key, ok := s.frameKey(f, palette)
	if !ok {
		return true
	}
	_, state := s.Pack.Lookup(key, f.Width, f.Height, true)
	if state == hdpack.Busy {
		return false
	}
	s.keep, s.keepW, s.keepH = "", 0, 0
	if state == hdpack.Ready || state == hdpack.Pending {
		s.keep, s.keepW, s.keepH = key, f.Width, f.Height
	}
	return true
}

// WarmMotion starts decoding an upcoming movement frame. It reports whether
// asking for later frames is worthwhile.
func (s *HDSurface) WarmMotion(f *CachedFrame, palette []byte) bool {
	if s.motionOff {
		return false
	}
	key, ok := s.frameKey(f, palette)
	if !ok {
		return true
	}
	_, state := s.Pack.Lookup(key, f.Width, f.Height, true)
	return state != hdpack.Busy
}

// fillOpaqueBlack writes 0,0,0,255 pixels by doubling a seeded prefix.
func fillOpaqueBlack(pixels []byte) {
	if len(pixels) < 4 {
		return
	}
	pixels[0], pixels[1], pixels[2], pixels[3] = 0, 0, 0, 255
	for n := 4; n < len(pixels); n *= 2 {
		copy(pixels[n:], pixels[:n])
	}
}
func (s *HDSurface) Clear() {
	fillOpaqueBlack(s.Pixels)
	s.Valid = true
	s.FrameStartHits = s.Hits
	if s.keep != "" {
		// Later requests must not push the landing picture out of the cache.
		s.Pack.Lookup(s.keep, s.keepW, s.keepH, false)
	}
}
func (s *HDSurface) lookup(src []byte, w, h int) *image.NRGBA {
	if w < 1 || h < 1 || w > 512 || h > 384 || len(src) < w*h*4 || !s.Pack.HasSize(w, h) {
		return nil
	}
	img, state := s.Pack.Lookup(s.pixelKey(src[:w*h*4], w, h), w, h, s.moving == 0)
	if img != nil {
		s.Hits++
	}
	if s.moving > 0 && w == s.Width && h == s.Height {
		if img != nil {
			s.shown++
		} else if state == hdpack.Busy || state == hdpack.Pending {
			s.late++
		}
	}
	return img
}
func (s *HDSurface) spriteKey(f *df.Sprite, palette []byte) string {
	hash := uint64(14695981039346656037)
	for _, b := range palette {
		hash = (hash ^ uint64(b)) * 1099511628211
	}
	id := spriteKeyID{f, hash ^ uint64(len(palette))<<48}
	if key, ok := s.spriteKeys[id]; ok {
		return key
	}
	if s.spriteKeys == nil || len(s.spriteKeys) >= 4096 {
		s.spriteKeys = map[spriteKeyID]string{}
	}
	key := hdpack.Key(f.Width, f.Height, spriteRGBA(f, palette))
	s.spriteKeys[id] = key
	return key
}

// Sprite artwork keys must be identical in the classic frame and side panels.
func spriteRGBA(f *df.Sprite, palette []byte) []byte {
	rgba := make([]byte, f.Width*f.Height*4)
	for i, on := range f.Opaque {
		if on != 0 {
			p := int(f.Indexed[i]) * 4
			if p+2 < len(palette) {
				copy(rgba[i*4:i*4+3], palette[p:p+3])
				rgba[i*4+3] = 255
			}
		}
	}
	return rgba
}

// MaskedArtwork replaces a complete character pose over the logical fallback.
// Original alpha and subtitle boundaries remain authoritative.
func (s *HDSurface) MaskedArtwork(source *image.NRGBA, clipY int) {
	if !s.Valid || source == nil {
		return
	}
	w, h := source.Rect.Dx(), source.Rect.Dy()
	img := s.lookup(source.Pix, w, h)
	if img == nil {
		return
	}
	x0, y0 := max(0, source.Rect.Min.X)*2, max(0, source.Rect.Min.Y)*2
	x1, y1 := min(s.Width, source.Rect.Max.X)*2, min(s.Height, clipY, source.Rect.Max.Y)*2
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			sx, sy := x-source.Rect.Min.X*2, y-source.Rect.Min.Y*2
			if source.Pix[(sy/2)*source.Stride+(sx/2)*4+3] == 0 {
				continue
			}
			from, to := sy*img.Stride+sx*4, (y*s.Width*2+x)*4
			copy(s.Pixels[to:to+3], img.Pix[from:from+3])
			s.Pixels[to+3] = 255
		}
	}
}
func (s *HDSurface) Blit(src []byte, w, h, x, y int) {
	if !s.Valid || w <= 0 || h <= 0 || len(src) < w*h*4 {
		return
	}
	img := s.lookup(src, w, h)
	x0, y0 := max(0, x)*2, max(0, y)*2
	x1, y1 := min(s.Width, x+w)*2, min(s.Height, y+h)*2
	if x1 <= x0 || y1 <= y0 {
		return
	}
	rowBytes := (x1 - x0) * 4
	for dy := y0; dy < y1; dy++ {
		to := (dy*s.Width*2 + x0) * 4
		if img != nil {
			from := (dy-y*2)*img.Stride + (x0-x*2)*4
			copy(s.Pixels[to:to+rowBytes], img.Pix[from:from+rowBytes])
		} else if dy%2 == 1 {
			previous := to - s.Width*2*4
			copy(s.Pixels[to:to+rowBytes], s.Pixels[previous:previous+rowBytes])
		} else {
			from := (((dy/2)-y)*w + x0/2 - x) * 4
			in, out := src[from:from+rowBytes/2], s.Pixels[to:to+rowBytes]
			for len(in) >= 4 && len(out) >= 8 {
				r, g, b, a := in[0], in[1], in[2], in[3]
				out[0], out[1], out[2], out[3] = r, g, b, a
				out[4], out[5], out[6], out[7] = r, g, b, a
				in, out = in[4:], out[8:]
			}
		}
	}
}
func (s *HDSurface) Sprite(r *SpriteRect, palette []byte, clipW, clipH int, occ *Occlusion, level float64) {
	if !s.Valid || r == nil || r.K <= 0 || !r.intersects(min(s.Width, clipW), min(s.Height, clipH)) {
		return
	}
	// Preserve unusual fractional flat-buffer addressing exactly by presenting
	// the original frame for this draw, rather than approximating its geometry.
	if r.X != math.Trunc(r.X) || r.Y != math.Trunc(r.Y) {
		s.Valid = false
		return
	}
	var img *image.NRGBA
	artworkAlpha := false
	if r.F.Width > 0 && r.F.Height > 0 && r.F.Width <= 512 && r.F.Height <= 384 && s.Pack.HasSize(r.F.Width, r.F.Height) {
		key := s.spriteKey(r.F, palette)
		img = s.Pack.Get(key, r.F.Width, r.F.Height)
		if img != nil {
			s.Hits++
			artworkAlpha = r.screen && s.Pack.SpriteAlpha(key, r.F.Width, r.F.Height)
		}
	}
	x0, y0 := max(0, int(r.X)), max(0, int(r.Y))
	x1, y1 := min(s.Width, clipW, int(math.Ceil(r.X+float64(r.W)))), min(s.Height, clipH, int(math.Ceil(r.Y+float64(r.H))))
	stride := s.Width * 2 * 4
	unit, unitX, unitY, imgW, imgH := r.K == 1, int(r.X)*2, int(r.Y)*2, 0, 0
	if img != nil {
		imgW, imgH = img.Rect.Dx(), img.Rect.Dy()
	}
	// Mask and occlusion are logical: decide once per original pixel, then
	// fill its 2x2 block.
	for ly := y0; ly < y1; ly++ {
		for lx := x0; lx < x1; lx++ {
			i := r.sample(float64(lx), float64(ly))
			if (i < 0 && !artworkAlpha) || SceneryOccludes(occ, lx, ly, level) {
				continue
			}
			to := ly*2*stride + lx*2*4
			if img == nil {
				p := int(r.F.Indexed[i]) * 4
				if p+2 < len(palette) {
					cr, cg, cb := palette[p], palette[p+1], palette[p+2]
					for _, at := range [4]int{to, to + 4, to + stride, to + stride + 4} {
						px := s.Pixels[at : at+3 : at+3]
						px[0], px[1], px[2] = cr, cg, cb
					}
				}
				continue
			}
			for y := ly * 2; y < ly*2+2; y++ {
				// Unscaled sprites map exactly: (y/2-Y)*2 is y-2Y for integral Y.
				sy := y - unitY
				if !unit {
					sy = int((float64(y)/2 - r.Y) / r.K * 2)
				}
				sy = max(0, min(imgH-1, sy))
				for x := lx * 2; x < lx*2+2; x++ {
					sx := x - unitX
					if !unit {
						sx = int((float64(x)/2 - r.X) / r.K * 2)
					}
					sx = max(0, min(imgW-1, sx))
					from, at := sy*img.Stride+sx*4, y*stride+x*4
					if artworkAlpha {
						// Display-only alpha: logical rendering and hit tests still use
						// the original mask. Blend over the already drawn background.
						a := int(img.Pix[from+3])
						for c := 0; c < 3; c++ {
							s.Pixels[at+c] = byte((int(img.Pix[from+c])*a + int(s.Pixels[at+c])*(255-a) + 127) / 255)
						}
					} else {
						copy(s.Pixels[at:at+3], img.Pix[from:from+3])
					}
				}
			}
		}
	}
}
