package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"testing"
)

func controllerFixture(t *testing.T) (*Player, *SetViewer, *Session) {
	v, s := viewerFixture(t)
	d := NewScreenDirector(s, ScreenWidth, ScreenHeight, nil)
	v.Director = d
	d.SetRoom(v)
	p := NewPlayer(new(testPlayerBridge))
	p.Ready = true
	p.Host = &GameHost{Session: s, Director: d, Viewer: v}
	return p, v, s
}
func TestControllerDialogueUsesAuthoredChoiceRects(t *testing.T) {
	p, _, s := controllerFixture(t)
	fixture, _ := puppetFixture(t)
	c := s.PuppetCtrl
	c.open("test.pup", fixture.Puppet.Pup)
	c.Bevel("Ask about the ship", 42)
	c.Bevel("Goodbye", 71)
	answer := -99.0
	s.Track("answer", false, func(task *Task) error { answer = c.Event(task); return nil })
	s.Pump(0, false, 1000)
	surface := p.ControllerSurface(false)
	if surface.Context != "dialogue" || len(surface.Targets) != 2 {
		t.Fatal(surface)
	}
	target := surface.Targets[1]
	if p.Host.Director.PuppetView.BevelAt(target.AimX, target.AimY) != 1 {
		t.Fatal("target is not on the authored reply", target)
	}
	if err := p.Host.Director.Click(target.AimX, target.AimY); err != nil {
		t.Fatal(err)
	}
	s.Pump(0, false, 1000)
	if answer != 71 {
		t.Fatal("controller click selected wrong reply", answer)
	}
	if len(p.ControllerSurface(false).Targets) != 0 {
		t.Fatal("stale replies remain after answering")
	}
}
func TestControllerMovieOriginAndReversedRegion(t *testing.T) {
	p, _, _ := controllerFixture(t)
	m := movieFixture()
	m.Segments[0].Frames[0].Regions = []df.MovieRegion{{Type: 2, X0: 8, Y0: 6, X1: 0, Y1: 0, Target: "first"}}
	startMovieTest(t, p.Host.Director.Movies, m)
	surface := p.ControllerSurface(false)
	if surface.Context != "movie" || len(surface.Targets) != 1 {
		t.Fatal(surface)
	}
	target := surface.Targets[0]
	if target.X != 10 || target.Y != 20 || target.W != 9 || target.H != 7 || !p.Host.Director.Movies.ClickableAt(target.AimX, target.AimY) {
		t.Fatal("movie origin or inclusive region bounds lost", target)
	}
}
func TestControllerBusyAndPausedSuppressTargets(t *testing.T) {
	p, v, _ := controllerFixture(t)
	if err := v.Turn(RightTurns); err != nil {
		t.Fatal(err)
	}
	if got := p.ControllerSurface(true); got.Context != "busy" || len(got.Targets) != 0 {
		t.Fatal("moving room has targets", got)
	}
	p.Paused = true
	if got := p.ControllerSurface(true); got.Context != "busy" || len(got.Targets) != 0 {
		t.Fatal("paused game has targets", got)
	}
}
