package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"testing"
)

func TestAssistedTargetsPreserveExactHitsAndAvoidAmbiguousNeighbors(t *testing.T) {
	p, v, s := controllerFixture(t)
	s.SetVisible, s.SetName = true, "room"
	v.current.Width, v.current.Height = 512, 264
	v.View().Objects = []df.ObjectEntry{
		{Identifier: "left", StartRegionX: 100, EndRegionX: 102, StartRegionY: 100, EndRegionY: 102},
		{Identifier: "right", StartRegionX: 110, EndRegionX: 112, StartRegionY: 100, EndRegionY: 102},
	}
	for _, c := range []struct{ x, y, r, wx, wy float64 }{
		{99, 101, 14, 100, 101}, {106, 101, 14, 106, 101}, // equal distance: don't guess
		{101, 101, 14, 101, 101}, {99, 101, 0, 99, 101}, // exact and mouse unchanged
		{80, 101, 14, 80, 101}, {-1, 101, 14, -1, 101},
	} {
		x, y := p.assistedPoint(c.x, c.y, c.r)
		if x != c.wx || y != c.wy {
			t.Errorf("%+v -> %g,%g", c, x, y)
		}
	}
	// Corrected clicks and drags keep the same offset until released.
	for _, c := range []PlayerCommand{
		{Action: "pointer", Kind: "press", X: 99, Y: 101, Radius: 14},
		{Action: "pointer", Kind: "move", X: 103, Y: 105},
		{Action: "pointer", Kind: "release", X: 103, Y: 105},
	} {
		if err := p.Command(c); err != nil {
			t.Fatal(err)
		}
		if p.Host.Session.PointerX != c.X+1 || p.Host.Session.PointerY != c.Y {
			t.Fatal("drag offset lost")
		}
	}
	p.Command(PlayerCommand{Action: "pointer", Kind: "move", X: 20, Y: 30})
	if p.Host.Session.PointerX != 20 {
		t.Fatal("offset leaked into next movement")
	}
	p.Paused = true
	if x, _ := p.assistedPoint(99, 101, 14); x != 99 {
		t.Fatal("paused target selected")
	}
}

func TestAssistedMovieRegionsUseOrigin(t *testing.T) {
	p, _, _ := controllerFixture(t)
	m := movieFixture()
	m.Segments[0].Frames[0].Regions = []df.MovieRegion{{Type: 2, X0: 8, Y0: 6, X1: 0, Y1: 0, Target: "first"}}
	startMovieTest(t, p.Host.Director.Movies, m)
	x, y := p.assistedPoint(8, 23, 14)
	if x != 10 || y != 23 {
		t.Fatal("movie target origin lost", x, y)
	}
}

func TestAssistedDialogueAndInputLocks(t *testing.T) {
	p, v, s := controllerFixture(t)
	v.View().Objects = []df.ObjectEntry{{Identifier: "tiny", StartRegionX: 100, EndRegionX: 102, StartRegionY: 100, EndRegionY: 102}}
	if err := v.Turn(RightTurns); err != nil {
		t.Fatal(err)
	}
	if x, _ := p.assistedPoint(99, 101, 14); x != 99 {
		t.Fatal("moving room gained targets")
	}
	fixture, _ := puppetFixture(t)
	s.PuppetCtrl.open("test.pup", fixture.Puppet.Pup)
	s.PuppetCtrl.Bevel("First", 42)
	s.Track("answer", false, func(task *Task) error { s.PuppetCtrl.Event(task); return nil })
	s.Pump(0, false, 1000)
	r := p.Host.Director.ChoiceRects()[0]
	x, y := p.assistedPoint(r.X+r.W/2, r.Y-2, 14)
	if p.Host.Director.PuppetView.BevelAt(x, y) != 0 {
		t.Fatal("nearby reply not selected", x, y, r)
	}
}

func TestAssistedTargetsRespectSpriteMaskAndPuzzleCoordinates(t *testing.T) {
	p, v, s := controllerFixture(t)
	s.SetVisible, s.SetName = true, "room"
	v.current.Width, v.current.Height = 512, 264
	shop := s.Props.AddShop("test", testShop())
	seedSprites(shop)
	shop.Persistent = true
	prop := s.Props.Get("door")
	prop.Visible = true
	prop.AnchorX, prop.AnchorY = 100, 100
	r, err := prop.ScreenRect()
	if err != nil {
		t.Fatal(err)
	}
	// The top right sprite pixel is transparent; assisted input must land on
	// an actual opaque pixel, not the sprite's bounding rectangle.
	x, y := p.assistedPoint(r.X+1, r.Y, 14)
	hit, err := p.Host.Director.HitTestAt(x, y)
	if err != nil || hit.Type != "prop" || r.sample(x, y) < 0 {
		t.Fatal("did not find opaque sprite pixel", hit, x, y, err)
	}
	prop.Visible = false
	x, y = p.assistedPoint(r.X+1, r.Y, 14)
	if x != r.X+1 || y != r.Y {
		t.Fatal("hidden sprite remained selectable")
	}
	s.SetVisible = false
	s.StageCtrl.Name = "puzzle"
	s.StageCtrl.CurrentFlat = "flat"
	s.StageCtrl.File = &df.Stage{File: &df.File{}}
	s.StageCtrl.regions["puzzle:flat"] = []df.StageRegion{{Name: "button", Left: 100, Right: 110, Top: 100, Bottom: 110}}
	x, y = p.assistedPoint(99, 105, 14)
	if x != 99 || y != 105 {
		t.Fatal("assistance changed precise puzzle coordinates")
	}
}
