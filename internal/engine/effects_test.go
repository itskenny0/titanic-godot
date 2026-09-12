package engine

import "testing"

func TestFadeRevealWaitsForScripts(t *testing.T) {
	f := FadeState{Level: 1, Blanked: true, PendingReveal: true}
	f.Tick(50, true)
	if f.Level != 1 || !f.PendingReveal {
		t.Fatal("stage revealed before its opening script completed")
	}
	f.Tick(100, false)
	if f.Level != 0 || f.Blanked || f.PendingReveal {
		t.Fatal("stage did not reveal after script completion")
	}
	f.Snapshot = &RGBAFrame{}
	f.Queue = []FadeRamp{{To: 1, Steps: 3}, {To: 0, Steps: 3}}
	f.Tick(100, false)
	if f.Level != 1.0/3 || f.Snapshot == nil {
		t.Fatal("fade lost its captured frame")
	}
	f.Tick(200, false)
	if f.Level != 0 || f.Snapshot != nil || f.Active() {
		t.Fatal("fade-in did not return to the live frame")
	}
}
func TestPartialTurnWipeHoldsCompositor(t *testing.T) {
	w := NewWipeState()
	w.Dir = "turnleft"
	w.Span = .5
	w.Steps = 2
	w.From = &RGBAFrame{}
	w.Tick(50)
	w.Tick(100)
	if w.Active() || !w.Compositing() || !w.Settled {
		t.Fatal("partial turn dropped its interpolation frame")
	}
	w.End()
	if w.Compositing() || w.Span != 1 || w.From != nil || w.To != nil {
		t.Fatal("wipe cleanup left stale frame state")
	}
	w.Dir = "left"
	w.From = &RGBAFrame{}
	w.Steps = 1
	w.Tick(200)
	w.Tick(217)
	if w.Compositing() {
		t.Fatal("completed full wipe retained its captured frame")
	}
}
func TestFrozenClockDoesNotCatchUp(t *testing.T) {
	audio := new(HostAudio)
	clock := NewGameClock(audio)
	clock.AdvanceFrames(clock.GameTime(100))
	clock.AdvanceFrames(clock.GameTime(150))
	if clock.FrameCounter != 1 {
		t.Fatal(clock.FrameCounter)
	}
	clock.Freeze()
	clock.Freeze()
	clock.AdvanceFrames(clock.GameTime(10000))
	if clock.FrameCounter != 1 || !clock.Frozen() {
		t.Fatal("frozen frame clock advanced")
	}
	clock.Thaw()
	clock.Thaw()
	if now := clock.GameTime(10050); now != 200 {
		t.Fatal("frozen time leaked into game time", now)
	}
	clock.AdvanceFrames(clock.GameTime(10050))
	if clock.FrameCounter != 2 {
		t.Fatal("thaw caught up frozen frames")
	}
	events := audio.DrainEvents()
	if len(events) != 2 || !events[0].On || events[1].On {
		t.Fatal("nested freeze toggled audio twice")
	}
	clock.AdvanceFrames(100000)
	if clock.FrameCounter != 66 {
		t.Fatal("frame catch-up was not capped")
	}
}
