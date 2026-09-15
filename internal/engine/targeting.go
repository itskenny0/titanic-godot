package engine

import (
	"fmt"
	"math"
	"sort"
)

// Search only on press, never on every hover or during a drag. These offsets
// are in original game pixels, independent of display resolution and HD packs.
var targetOffsets = func() [][3]int {
	var offsets [][3]int
	for y := -14; y <= 14; y++ {
		for x := -14; x <= 14; x++ {
			if d := x*x + y*y; d > 0 && d <= 14*14 {
				offsets = append(offsets, [3]int{x, y, d})
			}
		}
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i][2] < offsets[j][2] })
	return offsets
}()

func (p *Player) assistedPoint(x, y, radius float64) (float64, float64) {
	if radius <= 0 || math.IsNaN(radius) || p.Host == nil || !p.Ready || p.Paused || x < 0 || y < 0 || x >= ScreenWidth || y >= ScreenHeight {
		return x, y
	}
	d, s := p.Host.Director, p.Host.Session
	// Flat puzzle screens keep their exact coordinates. Movies with clickable
	// regions and dialogue use their own authored hit tests, ahead of room hits.
	if !d.Movies.Playing() && !d.AwaitingChoice() && (d.InputLocked() || !s.ViewShowing()) {
		return x, y
	}
	hit := func(px, py float64) string {
		if px < 0 || py < 0 || px >= ScreenWidth || py >= ScreenHeight {
			return ""
		}
		if d.Movies.Playing() {
			if d.Movies.ClickableAt(px, py) {
				seg := d.Movies.active.seg
				for i, r := range d.Movies.WaitingRegions() {
					if px >= float64(seg.OriginX+min(r.X0, r.X1)) && px <= float64(seg.OriginX+max(r.X0, r.X1)) && py >= float64(seg.OriginY+min(r.Y0, r.Y1)) && py <= float64(seg.OriginY+max(r.Y0, r.Y1)) {
						return fmt.Sprint("movie:", i)
					}
				}
			}
			return ""
		}
		if d.AwaitingChoice() {
			if i := d.PuppetView.BevelAt(px, py); i >= 0 {
				return fmt.Sprint("choice:", i)
			}
			return ""
		}
		h, err := d.HitTestAt(px, py)
		if err == nil && h.Type != "none" && h.Type != "flat" && h.Type != "scene" {
			return h.Type + ":" + h.Name
		}
		return ""
	}
	// An exact hit always wins, including a larger target beside a small one.
	if hit(x, y) != "" {
		return x, y
	}
	radius = math.Min(radius, 14)
	nearest, identity, ambiguous := -1, "", false
	ax, ay := x, y
	for _, offset := range targetOffsets {
		if float64(offset[2]) > radius*radius || nearest >= 0 && offset[2] > nearest {
			break
		}
		px, py := x+float64(offset[0]), y+float64(offset[1])
		if id := hit(px, py); id != "" {
			if nearest < 0 {
				nearest, identity, ax, ay = offset[2], id, px, py
			} else if identity != id {
				ambiguous = true
			}
		}
	}
	// Don't choose arbitrarily between equally close neighboring objects.
	if ambiguous {
		return x, y
	}
	return ax, ay
}
