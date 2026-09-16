package engine

import (
	"fmt"
	"math"
	"strings"
)

// This is a presentation hint, never a different game coordinate system.
// Unknown stages, scripts holding an inventory item and special effects retain
// the complete original frame. The frontend must also opt in explicitly.
type AdaptiveControl struct {
	Name  string  `json:"name"`
	Label string  `json:"label"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	AimX  float64 `json:"aim_x"`
	AimY  float64 `json:"aim_y"`
	Slot  int     `json:"slot"`
}
type AdaptiveLayout struct {
	Eligible bool              `json:"eligible"`
	Reason   string            `json:"reason"`
	Revision uint64            `json:"revision"`
	Controls []AdaptiveControl `json:"controls"`
}

func (p *Player) AdaptiveLayout() AdaptiveLayout {
	out := AdaptiveLayout{Reason: "special screen", Controls: []AdaptiveControl{}, Revision: p.Context.Version}
	if p.Host == nil || !p.Ready {
		return out
	}
	s, d := p.Host.Session, p.Host.Director
	// Movement fades, door transitions and world-sized movies (including
	// climbing) use the same world rectangle. They do not need the bottom bar.
	worldMovie := false
	if d.Movies.Playing() {
		movie, err := d.Movies.Frame()
		if err != nil || movie == nil || movie.MovieImage == nil || movie.OriginX < 0 || movie.OriginY < 0 || movie.OriginX+movie.Width > 512 || movie.OriginY+movie.Height > 264 {
			return out
		}
		worldMovie = true
	}
	transition := s.Fade.Active() || s.Fade.PendingReveal || s.Fade.Snapshot != nil || s.Wipe.Compositing()
	if (!s.ViewShowing() && !worldMovie && !transition) || d.Conversing() || s.PhotoOverlay != nil || s.XRay != nil && s.XRay.Aimed {
		return out
	}
	for _, label := range s.TextOverlay {
		if label.Y < 0 || label.Y > 264 {
			return out
		}
	}
	if s.StageCtrl.Name != "main.stg" || s.StageCtrl.CurrentFlat != "main 1" {
		return out
	}
	frame := d.roomFrame()
	flat := d.flatImage()
	if (frame == nil && !worldMovie && !transition) || (frame != nil && (frame.Width != 512 || frame.Height != 264)) || flat == nil || flat.Width != 512 || flat.Height != 384 {
		return out
	}
	if hand, _ := s.Interp.Globals.Get("handitem"); hand.IsString && hand.Text != "" {
		out.Reason = "held inventory item"
		return out
	}
	allowed := map[string]bool{"life": true, "bag": true, "map": true, "watch": true, "light": true, "navarrow": true}
	for _, prop := range s.Props.ScreenDrawList(true) {
		// The shared shop also contains open doors and location signs. These
		// remain in the world image and retain its coordinate mapping. They are
		// not toolbar controls, despite being persistent screen-space sprites.
		if prop.Shop.Name == "house.shp" && (prop.Name == "door" || prop.Name == "signs") {
			r, err := prop.ScreenRect()
			if err == nil && r != nil && r.K == 1 && r.Y < 264 && r.Y+float64(r.H) <= 264 {
				continue
			}
		}
		if !allowed[prop.Name] || prop.Shop.Name != "house.shp" || prop.Animating {
			out.Reason = "unrecognized or animated interface"
			return out
		}
	}
	// Only closed, idle controls are rearranged. Opening the watch, map, bag,
	// acquiring an item or a mod adding another interface object restores 4:3.
	for slot, spec := range [][2]string{{"life", "Voyage"}, {"bag", "Bag"}, {"watch", "Watch"}, {"map", "Map"}, {"navarrow", "Move"}} {
		prop := s.Props.Get(spec[0])
		if prop == nil || !prop.Visible || prop.Hidden {
			if spec[0] == "navarrow" || worldMovie || transition {
				continue
			}
			return out
		}
		if spec[0] == "bag" && !strings.HasSuffix(prop.StateName, "closed") {
			return out
		}
		r, err := prop.ScreenRect()
		if err != nil || r == nil || r.K != 1 || r.W > 128 || r.H > 128 || r.X < 0 || r.X+float64(r.W) > 512 || r.Y < 250 || r.Y+float64(r.H) > 384 {
			return out
		}
		target := AdaptiveControl{Name: spec[0], Label: spec[1], X: r.X, Y: r.Y, W: float64(r.W), H: float64(r.H), Slot: slot}
		found := false
		// A visible sprite can overlap another original control. Pick a point the
		// real hit tester resolves to this control; do not invent a new action.
		for i := 0; i < 50; i++ {
			x, y := r.X+math.Floor(float64(r.W)/2), r.Y+math.Floor(float64(r.H)/2)
			if i > 0 {
				x = r.X + math.Floor((float64((i-1)%7)+.5)*float64(r.W)/7)
				y = r.Y + math.Floor((float64((i-1)/7)+.5)*float64(r.H)/7)
			}
			if r.sample(x, y) < 0 {
				continue
			}
			hit, e := d.HitTestAt(x, y)
			if e == nil && hit.Type == "prop" && hit.Name == spec[0] {
				target.AimX, target.AimY = x, y
				found = true
				break
			}
		}
		if !found {
			return out
		}
		out.Controls = append(out.Controls, target)
	}
	out.Eligible, out.Reason = true, "exploration"
	return out
}

// The original sprites are presented separately, with their exact alpha and
// current palette. Interface HD artwork is integer scaled, so no second AI
// texture or game-data export is needed for these small controls.
func (p *Player) AdaptiveAtlas() []byte {
	layout := p.AdaptiveLayout()
	if !layout.Eligible {
		return nil
	}
	d := p.Host.Director
	flat := d.flatImage()
	palette := d.FlatPalette(flat.Palette)
	if d.room != nil {
		palette = d.room.BandPropPalette(flat.Palette)
	}
	const width, height = 640, 128
	out := make([]byte, width*height*4)
	for _, control := range layout.Controls {
		prop := p.Host.Session.Props.Get(control.Name)
		r, err := prop.ScreenRect()
		if err != nil || r == nil {
			return nil
		}
		for y := 0; y < r.H; y++ {
			for x := 0; x < r.W; x++ {
				i := r.sample(r.X+float64(x), r.Y+float64(y))
				if i < 0 {
					continue
				}
				pal := int(r.F.Indexed[i]) * 4
				if pal+2 >= len(palette) {
					continue
				}
				dst := (y*width + control.Slot*128 + x) * 4
				copy(out[dst:dst+3], palette[pal:pal+3])
				out[dst+3] = 255
			}
		}
	}
	return out
}

func (a AdaptiveLayout) String() string { return fmt.Sprintf("%t: %s", a.Eligible, a.Reason) }
