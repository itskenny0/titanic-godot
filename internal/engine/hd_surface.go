package engine

import (
	"image"
	"math"

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
}

func NewHDSurface(pack *hdpack.Reader, w, h int) *HDSurface {
	return &HDSurface{Pack: pack, Width: w, Height: h, Pixels: make([]byte, w*h*4*hdpack.Scale*hdpack.Scale)}
}
func (s *HDSurface) Clear() {
	clear(s.Pixels)
	for i := 3; i < len(s.Pixels); i += 4 {
		s.Pixels[i] = 255
	}
	s.Valid = true
	s.FrameStartHits = s.Hits
}
func (s *HDSurface) lookup(src []byte, w, h int) *image.NRGBA {
	if w < 1 || h < 1 || w > 512 || h > 384 || len(src) < w*h*4 {
		return nil
	}
	img := s.Pack.Get(hdpack.Key(w, h, src[:w*h*4]), w, h)
	if img != nil {
		s.Hits++
	}
	return img
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
			for dx := x0; dx < x1; dx += 2 {
				from := (((dy/2)-y)*w + dx/2 - x) * 4
				at := to + (dx-x0)*4
				copy(s.Pixels[at:at+4], src[from:from+4])
				copy(s.Pixels[at+4:at+8], src[from:from+4])
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
	if r.F.Width <= 512 && r.F.Height <= 384 {
		rgba := make([]byte, r.F.Width*r.F.Height*4)
		for i, on := range r.F.Opaque {
			if on != 0 {
				p := int(r.F.Indexed[i]) * 4
				if p+2 < len(palette) {
					copy(rgba[i*4:i*4+3], palette[p:p+3])
					rgba[i*4+3] = 255
				}
			}
		}
		img = s.lookup(rgba, r.F.Width, r.F.Height)
	}
	x0, y0 := max(0, int(r.X))*2, max(0, int(r.Y))*2
	x1, y1 := min(s.Width, clipW, int(math.Ceil(r.X+float64(r.W))))*2, min(s.Height, clipH, int(math.Ceil(r.Y+float64(r.H))))*2
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			lx, ly := x/2, y/2
			i := r.sample(float64(lx), float64(ly))
			if i < 0 || SceneryOccludes(occ, lx, ly, level) {
				continue
			}
			to := (y*s.Width*2 + x) * 4
			if img != nil {
				sx := max(0, min(img.Rect.Dx()-1, int((float64(x)/2-r.X)/r.K*2)))
				sy := max(0, min(img.Rect.Dy()-1, int((float64(y)/2-r.Y)/r.K*2)))
				from := sy*img.Stride + sx*4
				// Original masks retain exact hit and occlusion boundaries.
				copy(s.Pixels[to:to+3], img.Pix[from:from+3])
			} else {
				p := int(r.F.Indexed[i]) * 4
				if p+2 < len(palette) {
					copy(s.Pixels[to:to+3], palette[p:p+3])
				}
			}
		}
	}
}
