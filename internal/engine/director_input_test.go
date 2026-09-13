package engine

import (
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
)

func TestDirectorQueuedInputDoesNotLockItself(t *testing.T) {
	v, s := viewerFixture(t)
	d := NewScreenDirector(s, 512, 384, nil)
	v.Director = d
	d.SetRoom(v)
	calls := 0
	s.Interp.Register("record", func(*script.Interpreter, []script.Value, *script.Expr, *script.Frame) (script.Value, error) {
		calls++
		return script.Num(0), nil
	})
	s.BootScripts = []*script.Instance{dispatchInstance("boot", "mousedown", "exitcode")}
	blocker := s.Track("other script", false, func(task *Task) error { task.Sleep(100); return nil })
	s.Pump(0, false, 1000)
	if err := d.Click(1, 0); err != nil {
		t.Fatal(err)
	}
	s.Pump(0, false, 1000)
	if calls != 0 || s.Events.Len() != 1 {
		t.Fatal("input did not wait for another running script")
	}
	s.Pump(100, true, 1000)
	if !blocker.Done() {
		t.Fatal("blocking script did not finish")
	}
	if _, err := d.Tick(100); err != nil {
		t.Fatal(err)
	}
	s.Pump(100, true, 1000)
	if calls != 1 || s.Events.Len() != 0 || s.ScriptBusy() {
		t.Fatal("queued click counted its own dispatch as busy", calls, s.Events.Pending(), s.Pending())
	}
	// Navigation follows the same rule when dequeued from a moving room.
	s.Events.Post(QueuedEvent{Kind: "keydown", Key: "rightarrow"}, false)
	if _, err := d.Tick(150); err != nil {
		t.Fatal(err)
	}
	s.Pump(150, true, 1000)
	if !v.Animating() || s.Events.Len() != 0 {
		t.Fatal("queued direction kept requeuing")
	}
}
func TestDirectorPollingAndEventLockSuppressClicks(t *testing.T) {
	v, s := viewerFixture(t)
	d := NewScreenDirector(s, 512, 384, nil)
	v.Director = d
	d.SetRoom(v)
	calls := 0
	s.Interp.Register("record", func(*script.Interpreter, []script.Value, *script.Expr, *script.Frame) (script.Value, error) {
		calls++
		return script.Num(0), nil
	})
	s.BootScripts = []*script.Instance{dispatchInstance("boot", "mousedown", "")}
	s.Track("polling game script", false, func(task *Task) error { task.Sleep(100); return nil })
	s.Pump(0, false, 1000)
	s.InputPolled()
	d.Click(1, 0)
	s.Pump(0, false, 1000)
	if s.Events.Len() != 0 || calls != 0 {
		t.Fatal("polled input was also queued")
	}
	s.Pump(100, true, 1000)
	s.Interp.Globals.Set("lockevents", script.Num(1))
	d.Click(1, 0)
	s.Pump(100, false, 1000)
	if calls != 0 || s.Events.Len() != 0 {
		t.Fatal("locked events reached a handler")
	}
	if name, err := d.Hover(1, 0); err != nil || name != "watch" {
		t.Fatal("locked cursor differs", name, err)
	}
}
func TestDirectorStageFractionalHitAndKeyRouting(t *testing.T) {
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
	defer s.Close()
	d := NewScreenDirector(s, 512, 384, nil)
	s.StageCtrl.Name = "main"
	s.StageCtrl.CurrentFlat = "flat"
	s.StageCtrl.File = &df.Stage{File: &df.File{}}
	s.StageCtrl.regions["main:flat"] = []df.StageRegion{{Name: "button", Left: 10, Right: 20, Top: 30, Bottom: 40}}
	calls := 0
	s.Interp.Register("record", func(*script.Interpreter, []script.Value, *script.Expr, *script.Frame) (script.Value, error) {
		calls++
		return script.Num(0), nil
	})
	s.FlatScripts.Set("flat", dispatchInstance("flat", "keydown", "exitcode"))
	for _, x := range []float64{9.99, 10, 20, 20.01} {
		hit, err := d.HitTestAt(x, 35)
		if err != nil {
			t.Fatal(err)
		}
		want := "flat"
		if x >= 10 && x <= 20 {
			want = "button"
		}
		if hit.Type != want {
			t.Fatal("fractional stage coordinate truncated", x, hit)
		}
	}
	if _, err := d.KeyDown("x", false); err != nil {
		t.Fatal(err)
	}
	s.Pump(0, false, 1000)
	if calls != 1 || s.Events.Len() != 0 {
		t.Fatal("stage key dispatch locked itself")
	}
}
func TestDirectorPhotosOwnPixelsAndGammaRepaintsHeldMovie(t *testing.T) {
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
	defer s.Close()
	d := NewScreenDirector(s, 512, 384, nil)
	for i := range d.Screen.Frame {
		d.Screen.Frame[i] = byte(i * 7)
	}
	d.Screen.FrameValid = true
	photo := d.GrabPhoto(-100, 1000)
	if photo == nil || photo.Width != 320 || photo.Height != 240 || photo.RGBA[0] != d.Screen.Frame[(384-240)*512*4] {
		t.Fatal("photo crop was not clamped")
	}
	first := photo.RGBA[1]
	clear(d.Screen.Frame)
	if photo.RGBA[1] != first {
		t.Fatal("photo aliased live frame")
	}
	startMovieTest(t, d.Movies, movieFixture())
	ctx := NewDrawContext(512, 384)
	if err := d.Render(ctx); err != nil {
		t.Fatal(err)
	}
	version := ctx.Version
	if err := d.Render(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Version != version {
		t.Fatal("held movie repainted without visual change")
	}
	d.Gamma.Step(true, AllGammaChannels)
	if err := d.Render(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Version == version {
		t.Fatal("gamma change failed to repaint held movie")
	}
}
