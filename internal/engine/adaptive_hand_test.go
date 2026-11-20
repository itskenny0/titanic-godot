package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func handSave(t *testing.T) string {
	t.Helper()
	save := filepath.Join("..", "..", "originalgame", "lounge1c 2026-09-12-23-35-02.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no lounge save with Trask in originalgame/")
	}
	return save
}

// heldItem takes an item out the original way, into the hand beside the bag.
func heldItem(t *testing.T, name string) (*achRun, AdaptiveControl) {
	t.Helper()
	r, _ := startItemRun(t, handSave(t), true)
	r.give(name)
	r.play(60, nil)
	l := r.p.AdaptiveLayout()
	if !l.Eligible || l.AtlasSlots != handSlot+1 {
		t.Fatalf("an item in hand should keep the side panels: %+v", l)
	}
	for _, c := range l.Controls {
		if c.Hand {
			if c.Name != name || c.Dragged || c.Slot != handSlot {
				t.Fatalf("hand control %+v", c)
			}
			hit, err := r.p.Host.Director.HitTestAt(c.AimX, c.AimY)
			if err != nil || hit.Type != "prop" || hit.Name != name {
				t.Fatalf("pressing the hand control must press the item itself, got %+v", hit)
			}
			return r, c
		}
	}
	t.Fatalf("no hand control in %+v", l.Controls)
	return nil, AdaptiveControl{}
}

func handAtlasPixels(atlas []byte, scale int) int {
	width := (handSlot + 1) * 128 * scale
	n := 0
	for y := 0; y < 128*scale; y++ {
		for x := handSlot * 128 * scale; x < width; x++ {
			if atlas[(y*width+x)*4+3] != 0 {
				n++
			}
		}
	}
	return n
}

// drag sends the same pointer commands the classic screen would, in game
// coordinates; the side panels only change where the player touches.
func (r *achRun) drag(fromX, fromY, toX, toY float64, during func()) {
	p := r.p
	p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: fromX, Y: fromY})
	r.play(2, nil)
	for i := 1; i <= 10; i++ {
		p.Command(PlayerCommand{Action: "pointer", Kind: "move", X: fromX + (toX-fromX)*float64(i)/10, Y: fromY + (toY-fromY)*float64(i)/10})
		r.play(1, nil)
		if during != nil {
			during()
		}
	}
	p.Command(PlayerCommand{Action: "pointer", Kind: "release", X: toX, Y: toY})
}

func TestHeldItemOfferedFromSidePanels(t *testing.T) {
	r, hand := heldItem(t, "notebook")
	l := r.p.AdaptiveLayout()
	atlas := r.p.AdaptiveAtlas()
	if len(atlas) != (handSlot+1)*128*128*4*l.AtlasScale*l.AtlasScale || handAtlasPixels(atlas, l.AtlasScale) == 0 {
		t.Fatal("the item in hand is missing from the side panel artwork")
	}
	d := r.p.Host.Director
	dragged := 0
	r.drag(hand.AimX, hand.AimY, 214, 124, func() {
		l := r.p.AdaptiveLayout()
		if !l.Eligible {
			t.Fatalf("dragging the item must keep the side panels: %s", l)
		}
		for _, c := range l.Controls {
			if c.Hand && c.Dragged {
				dragged++
				if handAtlasPixels(r.p.AdaptiveAtlas(), l.AtlasScale) != 0 {
					t.Fatal("a dragged item is still drawn in hand")
				}
			}
		}
	})
	if dragged < 5 {
		t.Fatalf("the item followed the pointer in only %d of 10 steps", dragged)
	}
	talking := false
	r.play(600, func() bool { talking = d.Conversing() || d.AwaitingChoice(); return talking })
	if !talking {
		t.Fatal("offering the notebook to Trask should start the conversation")
	}
}

func TestHeldItemReturnsToBagFromSidePanels(t *testing.T) {
	r, hand := heldItem(t, "notebook")
	s := r.p.Host.Session
	// Dropped where nothing takes it, the item goes back into the hand.
	r.drag(hand.AimX, hand.AimY, 40, 40, nil)
	r.play(30, nil)
	if h, _ := s.Interp.Globals.Get("handitem"); h.Text != "notebook" {
		t.Fatalf("hand %q after dropping on nothing", h.Text)
	}
	var bag AdaptiveControl
	found := false
	for _, c := range r.p.AdaptiveLayout().Controls {
		if c.Hand && c.Dragged {
			t.Fatal("the item did not come back to the hand")
		}
		if c.Hand {
			hand, found = c, true
		}
		if c.Name == "bag" {
			bag = c
		}
	}
	if !found || bag.Name == "" {
		t.Fatalf("controls %+v", r.p.AdaptiveLayout().Controls)
	}
	r.drag(hand.AimX, hand.AimY, bag.AimX, bag.AimY, nil)
	r.play(300, func() bool {
		h, _ := s.Interp.Globals.Get("handitem")
		return h.Text == "" && r.p.AdaptiveLayout().Eligible
	})
	if h, _ := s.Interp.Globals.Get("handitem"); h.Text != "" || s.Props.Get("notebook").Visible {
		t.Fatalf("hand %q: dropping on the bag control should put the item away", h.Text)
	}
	if l := r.p.AdaptiveLayout(); !l.Eligible || l.AtlasSlots != handSlot {
		t.Fatalf("after putting the item away: %+v", l)
	}
}
