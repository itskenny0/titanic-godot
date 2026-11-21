package engine

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/hdpack"
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
	// Hand marks an item taken out of the bag; it has no fixed panel place.
	Hand bool `json:"hand,omitempty"`
	// Dragged marks the item while it follows the pointer through the world.
	Dragged bool `json:"dragged,omitempty"`
}
type AdaptiveLayout struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
	// Hold marks a moment that needs nothing outside the world picture: the
	// set closed between movies, a fade, the bag opening, an item in hand.
	// The frontend may keep the side panels through it instead of flashing
	// the classic layout for a few frames. Without Hold the original
	// screen is needed now: a full-screen movie, a close-up, a conversation.
	Hold       bool   `json:"hold"`
	Revision   uint64 `json:"revision"`
	AtlasScale int    `json:"atlas_scale"`
	// AtlasSlots is the atlas width in 128-pixel control slots.
	AtlasSlots int               `json:"atlas_slots"`
	Controls   []AdaptiveControl `json:"controls"`
}

// handSlot is the atlas slot after the five toolbar controls.
const handSlot = 5

// handControl describes an item taken out of the bag. The original keeps it
// beside the bag; the player drags it onto someone or something, or back into
// the bag, and the item's own script does the rest. The side panels show it
// as one more control, pressed at a point the hit tester resolves to the
// item. While it is dragged the item is drawn in the world and its place in
// hand stays empty until the button is released.
func (p *Player) handControl(name string) (AdaptiveControl, bool) {
	s, d := p.Host.Session, p.Host.Director
	prop := s.Props.Get(name)
	if prop == nil || !prop.Visible || prop.Hidden || prop.StateName != "large" || prop.Shop == nil || prop.Shop.Name != "inven.shp" {
		return AdaptiveControl{}, false
	}
	r, err := prop.ScreenRect()
	if err != nil || r == nil || r.K != 1 || r.W > 128 || r.H > 128 {
		return AdaptiveControl{}, false
	}
	control := AdaptiveControl{Name: name, Label: "In hand", X: r.X, Y: r.Y, W: float64(r.W), H: float64(r.H), Slot: handSlot, Hand: true}
	if r.X < 0 || r.X+float64(r.W) > 512 || r.Y < 250 || r.Y+float64(r.H) > 384 {
		if !s.PointerDown {
			return AdaptiveControl{}, false
		}
		control.Dragged = true
		control.AimX, control.AimY = math.Floor(r.X+float64(r.W)/2), math.Floor(r.Y+float64(r.H)/2)
		return control, true
	}
	if x, y, ok := aimAt(d, name, r); ok {
		control.AimX, control.AimY = x, y
		return control, true
	}
	return AdaptiveControl{}, false
}

// aimAt finds a point of a screen sprite the real hit tester resolves to that
// prop, so pressing the panel control does exactly what the original does.
// A visible sprite can overlap another control; no new action is invented.
func aimAt(d *ScreenDirector, name string, r *SpriteRect) (float64, float64, bool) {
	for i := 0; i < 50; i++ {
		x, y := r.X+math.Floor(float64(r.W)/2), r.Y+math.Floor(float64(r.H)/2)
		if i > 0 {
			x = r.X + math.Floor((float64((i-1)%7)+.5)*float64(r.W)/7)
			y = r.Y + math.Floor((float64((i-1)/7)+.5)*float64(r.H)/7)
		}
		if r.sample(x, y) < 0 {
			continue
		}
		if hit, e := d.HitTestAt(x, y); e == nil && hit.Type == "prop" && hit.Name == name {
			return x, y, true
		}
	}
	return 0, 0, false
}

func (p *Player) AdaptiveLayout() AdaptiveLayout {
	out := AdaptiveLayout{Reason: "special screen", Controls: []AdaptiveControl{}, Revision: p.Context.Version, AtlasScale: 1, AtlasSlots: handSlot}
	if p.Host == nil || !p.Ready {
		return out
	}
	if p.Host.Screen().HD != nil {
		out.AtlasScale = hdpack.Scale
	}
	s, d := p.Host.Session, p.Host.Director
	// Movement fades, door transitions and world-sized movies (including
	// climbing) use the same world rectangle. They do not need the bottom bar.
	worldMovie := false
	if d.Movies.Playing() {
		movie, err := d.Movies.Frame()
		if err != nil || movie == nil || movie.MovieImage == nil || movie.OriginX < 0 || movie.OriginY < 0 || movie.OriginX+movie.Width > 512 || movie.OriginY+movie.Height > 264 {
			out.Reason = "full-screen movie"
			return out
		}
		worldMovie = true
	}
	if d.Conversing() {
		out.Reason = "conversation"
		return out
	}
	if s.PhotoOverlay != nil || s.XRay != nil && s.XRay.Aimed {
		out.Reason = "picture overlay"
		return out
	}
	for _, label := range s.TextOverlay {
		if label.Y < 0 || label.Y > 264 {
			out.Reason = "text outside the world"
			return out
		}
	}
	if s.StageCtrl.Name != "main.stg" || s.StageCtrl.CurrentFlat != "main 1" {
		return out
	}
	flat := d.flatImage()
	if flat == nil || flat.Width != 512 || flat.Height != 384 {
		return out
	}
	// From here on the screen is the ordinary exploration picture, only not
	// ready for side panels at this moment.
	out.Hold = true
	transition := s.Fade.Active() || s.Fade.PendingReveal || s.Fade.Snapshot != nil || s.Wipe.Compositing()
	if !s.ViewShowing() && !worldMovie && !transition {
		out.Reason = "room not showing"
		return out
	}
	frame := d.roomFrame()
	if (frame == nil && !worldMovie && !transition) || (frame != nil && (frame.Width != 512 || frame.Height != 264)) {
		out.Reason = "room not showing"
		return out
	}
	var hand *AdaptiveControl
	if held, _ := s.Interp.Globals.Get("handitem"); held.IsString && held.Text != "" {
		c, ok := p.handControl(held.Text)
		if !ok {
			out.Reason = "held inventory item"
			return out
		}
		hand = &c
	}
	allowed := map[string]bool{"invenhelp": true, "life": true, "bag": true, "map": true, "watch": true, "light": true, "navarrow": true}
	for _, prop := range s.Props.ScreenDrawList(true) {
		// The shared shop also contains open doors and location signs. These
		// remain in the world image and retain its coordinate mapping. They are
		// not toolbar controls, despite being persistent screen-space sprites.
		// The baby Frank carries is drawn the same way, in the picture.
		if prop.Shop.Name == "house.shp" && (prop.Name == "door" || prop.Name == "signs" || prop.Name == "baby") {
			r, err := prop.ScreenRect()
			if err == nil && r != nil && r.K == 1 && r.Y < 264 && r.Y+float64(r.H) <= 264 {
				continue
			}
		}
		if hand != nil && prop.Name == hand.Name {
			continue
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
		// The bag, watch and map lie in the bedsit until Frank picks them up;
		// until then they are no toolbar controls.
		if prop != nil && spec[0] != "life" && spec[0] != "navarrow" && prop.Owner.Text != "frank" {
			continue
		}
		if prop == nil || !prop.Visible || prop.Hidden {
			if spec[0] == "navarrow" || worldMovie || transition {
				continue
			}
			out.Reason = "interface hidden"
			return out
		}
		if spec[0] == "bag" && !strings.HasSuffix(prop.StateName, "closed") {
			out.Reason = "bag open"
			return out
		}
		r, err := prop.ScreenRect()
		if err != nil || r == nil || r.K != 1 || r.W > 128 || r.H > 128 || r.X < 0 || r.X+float64(r.W) > 512 || r.Y < 250 || r.Y+float64(r.H) > 384 {
			out.Reason = "interface moved"
			return out
		}
		target := AdaptiveControl{Name: spec[0], Label: spec[1], X: r.X, Y: r.Y, W: float64(r.W), H: float64(r.H), Slot: slot}
		x, y, found := aimAt(d, spec[0], r)
		if !found {
			out.Reason = "interface covered"
			return out
		}
		target.AimX, target.AimY = x, y
		out.Controls = append(out.Controls, target)
	}
	// Controls that come and go take the slots after the toolbar's five.
	extra := func(c AdaptiveControl) {
		c.Slot = out.AtlasSlots
		out.AtlasSlots++
		out.Controls = append(out.Controls, c)
	}
	if hand != nil {
		extra(*hand)
	}
	// A new voyage shows a Help button in the toolbar until Frank is given
	// his first item. It plays the original help movie.
	if help := s.Props.Get("invenhelp"); help != nil && help.Visible && !help.Hidden && help.Shop != nil && help.Shop.Name == "house.shp" {
		r, err := help.ScreenRect()
		if err != nil || r == nil || r.K != 1 || r.W > 128 || r.H > 128 || r.X < 0 || r.X+float64(r.W) > 512 || r.Y < 250 || r.Y+float64(r.H) > 384 {
			out.Reason = "interface moved"
			return out
		}
		x, y, found := aimAt(d, "invenhelp", r)
		if !found {
			out.Reason = "interface covered"
			return out
		}
		extra(AdaptiveControl{Name: "invenhelp", Label: "Help", X: r.X, Y: r.Y, W: float64(r.W), H: float64(r.H), AimX: x, AimY: y})
	}
	out.Eligible, out.Reason, out.Hold = true, "exploration", false
	return out
}

// Side panels use the same source-keyed HD artwork as the classic toolbar.
// Logical geometry remains original. Explicit vector replacements may supply
// smooth display alpha; a missing replacement uses the original pixels and mask.
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
	scale := layout.AtlasScale
	width, height := layout.AtlasSlots*128*scale, 128*scale
	out := make([]byte, width*height*4)
	for _, control := range layout.Controls {
		prop := p.Host.Session.Props.Get(control.Name)
		r, err := prop.ScreenRect()
		if err != nil || r == nil {
			return nil
		}
		// A dragged item is drawn in the world; its place in hand stays empty.
		if control.Dragged {
			continue
		}
		var replacement *image.NRGBA
		var ramp *[3][256]byte
		artworkAlpha := false
		if hd := p.Host.Screen().HD; hd != nil {
			// Artwork is keyed at the default brightness, like the classic frame.
			rgba := spriteRGBA(r.F, d.Gamma.DefaultPalette(palette))
			key := hdpack.Key(r.F.Width, r.F.Height, rgba)
			replacement = hd.Pack.Get(key, r.F.Width, r.F.Height)
			artworkAlpha = replacement != nil && hd.Pack.SpriteAlpha(key, r.F.Width, r.F.Height)
			if replacement != nil {
				ramp = d.Gamma.ArtworkRamp()
			}
		}
		shade := func(dst, from []byte) {
			copy(dst, from)
			if ramp != nil {
				for c := 0; c < 3; c++ {
					dst[c] = ramp[c][dst[c]]
				}
			}
		}
		for y := 0; y < r.H*scale; y++ {
			for x := 0; x < r.W*scale; x++ {
				i := r.sample(r.X+float64(x/scale), r.Y+float64(y/scale))
				if i < 0 && !artworkAlpha {
					continue
				}
				dst := (y*width + control.Slot*128*scale + x) * 4
				if artworkAlpha {
					from := y*replacement.Stride + x*4
					shade(out[dst:dst+4], replacement.Pix[from:from+4])
					continue
				}
				pal := int(r.F.Indexed[i]) * 4
				if pal+2 >= len(palette) {
					continue
				}
				if replacement != nil {
					from := y*replacement.Stride + x*4
					shade(out[dst:dst+3], replacement.Pix[from:from+3])
				} else {
					copy(out[dst:dst+3], palette[pal:pal+3])
				}
				out[dst+3] = 255
			}
		}
	}
	return out
}

func (a AdaptiveLayout) String() string { return fmt.Sprintf("%t: %s", a.Eligible, a.Reason) }
