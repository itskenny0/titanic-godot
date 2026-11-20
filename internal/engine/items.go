package engine

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"sort"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/script"
)

// Items the player can carry, with the names an item notice shows. The ids
// are the original inventory props; the baby lives in the house shop but is
// carried the same way.
var itemNames = map[string]string{
	"antidote": "Antidote", "baby": "Baby", "binoculars": "Binoculars", "boatpass": "Boat Pass",
	"bombkey": "Bomb Key", "carkeys": "Car Keys", "conkletter": "Conkling Letter", "cufflink": "Cufflink",
	"fakeneck": "Diamond Necklace", "gaspen": "Gas Pen", "hackclue": "Scrap of Paper", "icewarning": "Ice Warning",
	"notebook": "Willie's Notebook", "oldreds": "Old Reds Cigarettes", "painting": "Painting", "photo1": "Photograph",
	"photo2": "Photograph", "photo3": "Photograph", "pipe": "Pipe", "prayercard": "Prayer Card",
	"realneck": "Diamond Necklace", "ring": "Willie's Ring", "rubaiyat": "The Rubaiyat", "rubiclue": "Scrap of Paper",
	"shawl": "Claris's Shawl", "thayergram": "Thayer's Telegram", "trunkkey": "Trunk Key", "vladfood": "Package for Vlad",
	"zeitgram": "Zeitel's Telegram",
}

// itemWatch notices items as they come into the player's possession. With
// ToBag on, a newly received item goes straight into the bag once the
// scripts are idle, as if the player had dropped it there, so exploration
// can stay in the side-panel layout. Items taken out of the bag on purpose
// were already owned and stay in hand.
type itemWatch struct {
	ToBag   bool
	Off     bool
	owned   map[string]bool
	primed  bool
	pending map[string]bool
}

func (p *Player) serviceItems() {
	s := p.Host.Session
	w := &p.items
	if !w.primed && s.ScriptBusy() {
		return
	}
	owned := map[string]bool{}
	for name := range itemNames {
		if prop := s.Props.Get(name); prop != nil && strings.EqualFold(prop.Owner.Text, "frank") {
			owned[name] = true
		}
	}
	if !w.primed {
		w.owned, w.primed, w.pending = owned, true, map[string]bool{}
		return
	}
	touring := achTouring(s)
	var fresh []string
	for name := range owned {
		if !w.owned[name] {
			fresh = append(fresh, name)
		}
	}
	sort.Strings(fresh)
	w.owned = owned
	for _, name := range fresh {
		if touring || w.Off {
			continue
		}
		fields := map[string]any{"id": name, "name": itemNames[name], "bag": w.ToBag}
		if icon := p.itemIcon(name); icon != "" {
			fields["icon"] = icon
		}
		p.emit("item", fields)
		if w.ToBag {
			w.pending[name] = true
		}
	}
	if len(w.pending) == 0 || s.ScriptBusy() {
		return
	}
	hand, _ := s.Interp.Globals.Get("handitem")
	for name := range w.pending {
		delete(w.pending, name)
		prop := s.Props.Get(name)
		// Only an item the game placed in the player's hand, and still there.
		if prop == nil || !hand.IsString || !strings.EqualFold(hand.Text, name) || !owned[name] || prop.StateName != "large" {
			continue
		}
		prop.Visible = false
		s.Interp.Globals.Set("handitem", script.Str(""))
		p.Context.Version++
	}
}

// itemIcon renders the item as it appears in the player's hand, as a PNG.
func (p *Player) itemIcon(name string) string {
	prop := p.Host.Session.Props.Get(name)
	d := p.Host.Director
	flat := d.flatImage()
	if prop == nil || flat == nil {
		return ""
	}
	for i := range prop.Group.States {
		st := &prop.Group.States[i]
		if strings.ToLower(st.Identifier) != "large" || len(st.Frames) == 0 {
			continue
		}
		f, err := prop.frame(st, 0)
		if err != nil || f == nil || f.Width <= 0 || f.Height <= 0 {
			return ""
		}
		img := image.NewNRGBA(image.Rect(0, 0, f.Width, f.Height))
		copy(img.Pix, spriteRGBA(f, d.FlatPalette(flat.Palette)))
		var out bytes.Buffer
		if png.Encode(&out, img) != nil {
			return ""
		}
		return base64.StdEncoding.EncodeToString(out.Bytes())
	}
	return ""
}
