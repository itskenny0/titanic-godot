package engine

import "slices"

const ScreenWidth = 512
const ScreenHeight = 384
const repaintEvery = 60

type ScreenRect struct{ X, Y, W, H float64 }
type ScreenPresenter struct {
	Width, Height  int
	Frame          []byte
	FrameValid     bool
	presented      bool
	scratch        []byte
	lastLo, lastHi uint32
	haveSignature  bool
	skippedFrames  int
}

func NewScreenPresenter(w, h int) *ScreenPresenter {
	return &ScreenPresenter{Width: w, Height: h, Frame: make([]byte, w*h*4)}
}
func (s *ScreenPresenter) ScratchFor(n int) []byte {
	if len(s.scratch) < n {
		s.scratch = make([]byte, n)
	}
	return s.scratch
}
func (s *ScreenPresenter) ClearFrame() {
	clear(s.Frame)
	for i := 3; i < len(s.Frame); i += 4 {
		s.Frame[i] = 255
	}
}
func (s *ScreenPresenter) Capture() *RGBAFrame {
	if !s.FrameValid {
		return nil
	}
	return &RGBAFrame{RGBA: slices.Clone(s.Frame), Width: s.Width, Height: s.Height}
}
func (s *ScreenPresenter) ShouldPaint(sig *DrawSignature) bool {
	if s.FrameValid && s.presented && s.haveSignature && sig.Lo == s.lastLo && sig.Hi == s.lastHi {
		s.skippedFrames++
		if s.skippedFrames < repaintEvery {
			return false
		}
	}
	s.skippedFrames = 0
	s.lastLo, s.lastHi = sig.Lo, sig.Hi
	s.haveSignature = true
	return true
}
func (s *ScreenPresenter) BlitAt(src []byte, w, h, x, y int) {
	x0, y0 := max(0, x), max(0, y)
	x1, y1 := min(s.Width, x+w), min(s.Height, y+h)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	for row := y0; row < y1; row++ {
		from := ((row-y)*w + x0 - x) * 4
		to := (row*s.Width + x0) * 4
		n := (x1 - x0) * 4
		copy(s.Frame[to:to+n], src[from:from+n])
	}
}
func (s *ScreenPresenter) BlitTop(src []byte, w, h int) { s.BlitAt(src, w, h, 0, 0) }
func (s *ScreenPresenter) Blit(c *DrawContext) {
	c.Width, c.Height = s.Width, s.Height
	c.Commands = nil
	c.Version++
	s.presented = true
}
func (s *ScreenPresenter) DrawTextOverlay(c *DrawContext, overlay []TextOverlay) {
	if len(overlay) == 0 {
		return
	}
	c.Save()
	defer c.Restore()
	for _, e := range overlay {
		c.Font = OverlayFont(e.Size)
		c.Fill = "#e8e8e8"
		if e.Color == 0 {
			c.Fill = "#000"
		}
		c.FillText(e.Text, e.X, e.Y)
	}
}
func (s *ScreenPresenter) ApplyFade(c *DrawContext, level float64) {
	if level <= 0 {
		return
	}
	c.Save()
	defer c.Restore()
	c.Fill = fadeColor(level)
	c.FillRect(0, 0, float64(c.Width), float64(c.Height))
}
func (s *ScreenPresenter) ApplyFadeExcept(c *DrawContext, level float64, keep ScreenRect) {
	if level <= 0 {
		return
	}
	w, h := float64(c.Width), float64(c.Height)
	x0, y0 := max(0, min(w, keep.X)), max(0, min(h, keep.Y))
	x1, y1 := max(0, min(w, keep.X+keep.W)), max(0, min(h, keep.Y+keep.H))
	c.Save()
	defer c.Restore()
	c.Fill = fadeColor(level)
	if x1 <= x0 || y1 <= y0 {
		c.FillRect(0, 0, w, h)
	} else {
		if y0 > 0 {
			c.FillRect(0, 0, w, y0)
		}
		if y1 < h {
			c.FillRect(0, y1, w, h-y1)
		}
		if x0 > 0 {
			c.FillRect(0, y0, x0, y1-y0)
		}
		if x1 < w {
			c.FillRect(x1, y0, w-x1, y1-y0)
		}
	}
}
