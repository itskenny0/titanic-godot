package engine

import "math"

func (d *ScreenDirector) pushTurn(dir string) {
	w, screen := &d.Session.Wipe, d.Screen
	from, to := w.From, w.To
	if from == nil || to == nil || from.Width != screen.Width || from.Height != screen.Height || to.Width != from.Width || to.Height != from.Height {
		return
	}
	// Turning wipes compose saved logical frames directly.
	if screen.HD != nil {
		screen.HD.Valid = false
	}
	width, height := from.Width, from.Height
	travel := math.Max(1, jsRound(float64(float64(width)*w.Span)))
	per := math.Floor(travel/math.Max(1, w.Steps)) + 1
	off := int(math.Max(0, math.Min(travel, float64(w.Step*per))))
	keep := width - off
	quarter := 0
	if w.Span < 1 {
		quarter = width >> 2
	}
	for y := 0; y < height; y++ {
		row := y * width * 4
		if dir == "turnright" {
			copy(screen.Frame[row+keep*4:], to.RGBA[row+quarter*4:row+(quarter+off)*4])
			for x := 0; x < keep; x++ {
				dest, src := row+x*4, row+(off+x)*4
				copy(screen.Frame[dest:dest+3], from.RGBA[src:src+3])
			}
		} else {
			end := width - quarter
			copy(screen.Frame[row:], to.RGBA[row+(end-off)*4:row+end*4])
			for x := 0; x < keep; x++ {
				dest, src := row+(off+x)*4, row+x*4
				copy(screen.Frame[dest:dest+3], from.RGBA[src:src+3])
			}
		}
	}
}
func (d *ScreenDirector) coverWithWipe() {
	w := &d.Session.Wipe
	if !w.Compositing() || w.From == nil {
		return
	}
	if w.Dir == "turnleft" || w.Dir == "turnright" {
		d.pushTurn(w.Dir)
		return
	}
	if !w.Active() {
		return
	}
	from := w.From
	width, height := from.Width, from.Height
	kept := int(math.Max(0, math.Min(float64(width), jsRound(float64(float64(width)*(w.Steps-w.Step))/w.Steps))))
	if kept <= 0 {
		return
	}
	put := func(srcX, cols int) {
		if cols <= 0 {
			return
		}
		strip := d.Screen.ScratchFor(cols * height * 4)
		for y := 0; y < height; y++ {
			fromAt := (y*width + srcX) * 4
			copy(strip[y*cols*4:], from.RGBA[fromAt:fromAt+cols*4])
		}
		d.Screen.BlitAt(strip, cols, height, srcX, 0)
	}
	switch w.Dir {
	case "open":
		half := kept >> 1
		put(0, half)
		put(width-(kept-half), kept-half)
	case "close":
		put((width-kept)>>1, kept)
	case "left":
		put(0, kept)
	default:
		put(width-kept, kept)
	}
}
