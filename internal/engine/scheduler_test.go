package engine

import (
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/df"
	"os"
	"reflect"
	"strings"
	"testing"
)

type recordedPlay struct {
	Channel                  AudioChannel
	Seconds, Volume, Pan     float64
	Loop, Stopped, Displaced bool
}

func (p *recordedPlay) Done() bool { return !p.Loop || p.Stopped || p.Displaced }
func (p *recordedPlay) Stop()      { p.Stopped = true }

type recordingAudio struct {
	Calls   []*recordedPlay
	holding map[AudioChannel]*recordedPlay
}

func (a *recordingAudio) Play(c AudioChannel, d *df.Audio, o PlayOptions) PlayHandle {
	p := &recordedPlay{Channel: c, Seconds: float64(len(d.Samples)) / float64(d.SampleRate), Volume: playVolume(o), Pan: o.Pan, Loop: o.Loop}
	if !o.Overlap {
		a.Halt(c)
		if a.holding == nil {
			a.holding = map[AudioChannel]*recordedPlay{}
		}
		a.holding[c] = p
	}
	a.Calls = append(a.Calls, p)
	return p
}
func (a *recordingAudio) Halt(c AudioChannel) {
	if p := a.holding[c]; p != nil && !p.Stopped {
		p.Displaced = true
	}
	delete(a.holding, c)
}
func (*recordingAudio) IsDone(AudioChannel) bool               { return true }
func (*recordingAudio) SetChannelVolume(AudioChannel, float64) {}
func (*recordingAudio) SetSuspended(bool)                      {}

type schedulerTestHost struct {
	executor        *Executor
	set             string
	busy            bool
	active          int
	events          [][]any
	nav, fromScript bool
	global          bool
	hook            func(*Task, string) error
	clock           float64
}

func (h *schedulerTestHost) CurrentSet() string        { return h.set }
func (*schedulerTestHost) CurrentFlatName() string     { return "main" }
func (h *schedulerTestHost) ScriptBusy() bool          { return h.busy || h.active > 0 }
func (*schedulerTestHost) AmbientRandom() float64      { return .25 }
func (*schedulerTestHost) ListenerPosition() *Listener { return &Listener{} }
func (*schedulerTestHost) HasHandler(kind, name, handler string) bool {
	return kind == "actor" && handler == "endwalk"
}
func (h *schedulerTestHost) HasGlobal(string) bool { return h.global }
func (h *schedulerTestHost) RunGlobal(task *Task, name string) error {
	h.events = append(h.events, []any{"global", name})
	task.Wait(func() bool { return true })
	return nil
}
func (h *schedulerTestHost) SendEvent(task *Task, cmd, target, handler, caller string) error {
	h.events = append(h.events, []any{cmd, target, handler, caller, h.nav})
	if h.hook != nil {
		if err := h.hook(task, handler); err != nil {
			return err
		}
	}
	task.Wait(func() bool { return true })
	return nil
}
func (h *schedulerTestHost) Track(name string, idle bool, run func(*Task) error) {
	if !idle {
		h.active++
	}
	h.executor.Start(name, func(task *Task) error {
		if !idle {
			defer func() { h.active-- }()
		}
		return run(task)
	})
}
func (h *schedulerTestHost) WithNavigation(run func() error) error {
	nav, from := h.nav, h.fromScript
	h.nav, h.fromScript = true, true
	defer func() { h.nav, h.fromScript = nav, from }()
	return run()
}
func (h *schedulerTestHost) AdvanceClock(now float64) { h.clock = now }
func (*schedulerTestHost) Log(string)                 {}
func normalizedJSON(v any) any {
	b, _ := json.Marshal(v)
	var out any
	json.Unmarshal(b, &out)
	var lower func(any) any
	lower = func(v any) any {
		switch v := v.(type) {
		case map[string]any:
			m := map[string]any{}
			for k, x := range v {
				m[strings.ToLower(k[:1])+k[1:]] = lower(x)
			}
			return m
		case []any:
			for i, x := range v {
				v[i] = lower(x)
			}
			return v
		}
		return v
	}
	return lower(out)
}
func schedulerFixture() (*Scheduler, *schedulerTestHost, *recordingAudio) {
	actors := new(ActorRuntime)
	cast := &df.Cast{}
	for _, name := range []string{"alice", "bob", "claire"} {
		cast.Members = append(cast.Members, df.CastMember{Name: name, Poses: []df.CastPose{{Name: "stand", Play: []int{0, 1}}, {Name: "walk", Play: []int{0, 1, 2}}}})
	}
	actors.AddCast("test", cast)
	for _, a := range actors.Actors.All() {
		a.PoseName = "walk"
		a.WorldX = -3
		a.WorldY = 2
		a.WorldZ = -1
		a.Speed = 7
		a.Turn = 16
		a.Deg = 250
	}
	lib := NewAudioLibrary()
	lib.order = []string{"test"}
	lib.banks["test"] = audioBankEntry{bank: &df.AudioBank{Singles: map[string]df.BankChunk{}}}
	// Populate the same decoded samples for each authored sound; codec accuracy is
	// checked separately against the owned-data corpus.
	for _, name := range []string{"rumble", "single"} {
		lib.banks["test"].bank.Singles[name] = df.BankChunk{}
		lib.cache["test|"+name] = &df.Audio{SampleRate: 100, Samples: make([]float32, 20)}
	}
	audio := &recordingAudio{Calls: []*recordedPlay{}}
	host := &schedulerTestHost{executor: NewExecutor(), set: "room", global: true, events: [][]any{}}
	return NewScheduler(host, actors, lib, audio), host, audio
}
func TestSchedulerReference(t *testing.T) {
	b, err := os.ReadFile("../../tests/fixtures/scheduler.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected []map[string]any
	if err = json.Unmarshal(b, &expected); err != nil {
		t.Fatal(err)
	}
	s, h, audio := schedulerFixture()
	defer h.executor.Close()
	finish, routeEnd := "finish", "routeEnd"
	s.StartWalk("alice", -20, 35, -3, &finish)
	s.StartWalkPath("bob", []RoutePoint{{-3, 2, -1, 0}, {12, -7, 3, 20}, {24, 10, 8, 30}}, &routeEnd)
	s.StartTurn("claire", 128)
	s.MakeLoop("scene", "scene2", " Animate ( ) ", 3)
	s.MakeLoop("prop", "counter", "once()", 1)
	s.MakeLoop("flat", "missing", "flatTick", 2)
	s.MakeCricket("rumble", 3, 4, 10, 2, 3)
	s.MakeCricket("single", 100, 100, 10, 1, -1)
	s.SoundLoop("rumble", true)
	for step, want := range expected {
		h.busy = step >= 2 && step <= 5
		if step == 4 {
			if err = s.PlaySound("rumble", true); err != nil {
				t.Fatal(err)
			}
			if err = s.PlaySound("rumble", true); err != nil {
				t.Fatal(err)
			}
		}
		if step == 5 {
			s.SoundLoop("rumble", false)
		}
		if step == 7 {
			s.PauseWalk("bob", true)
		}
		if step == 8 {
			h.set = "other"
		}
		if step == 10 {
			h.set = "room"
			s.PauseWalk("bob", false)
		}
		if step == 12 {
			s.HaltSounds()
		}
		now := float64((step + 1) * 50)
		if err = s.TickTime(now); err != nil {
			t.Fatal(err)
		}
		s.ServiceFrameLoops()
		for _, done := range h.executor.Pump(now, true, 10000) {
			if done.Err != nil {
				t.Fatal(done.Err)
			}
		}
		actors := []any{}
		for _, a := range s.Actors.Actors.All() {
			actors = append(actors, map[string]any{"name": a.Name, "x": a.WorldX, "y": a.WorldY, "z": a.WorldZ, "deg": a.Deg, "pose": a.PoseName, "step": a.Step, "star": a.StarName})
		}
		crickets := []any{}
		for _, c := range s.Crickets {
			crickets = append(crickets, map[string]any{"name": c.Name, "setName": c.SetName, "x": c.X, "y": c.Y, "radius": c.Radius, "base": c.Base, "jitter": c.Jitter, "count": c.Count, "paused": c.Paused, "done": c.Handle == nil || c.Handle.Done()})
		}
		walks := append([]string{}, s.Walks.keys...)
		loops := append([]*GameLoop{}, s.Loops...)
		got := normalizedJSON(map[string]any{"step": step, "actors": actors, "walks": walks, "loops": loops, "crickets": crickets, "events": h.events, "calls": audio.Calls, "navigation": []any{"oldNav", "oldScene", "oldView", h.nav, h.fromScript}}).(map[string]any)
		for k, v := range want {
			if !reflect.DeepEqual(got[k], v) {
				a, _ := json.Marshal(got[k])
				b, _ := json.Marshal(v)
				t.Fatalf("step %d %s differs\ngot %s\nwant %s", step, k, a, b)
			}
		}
	}
}

func TestSchedulerClockCatchupAndCancellation(t *testing.T) {
	s, h, _ := schedulerFixture()
	defer h.executor.Close()
	s.MakeLoop("prop", "slow", "later", 100)
	if err := s.TickTime(50); err != nil {
		t.Fatal(err)
	}
	if err := s.TickTime(10000); err != nil {
		t.Fatal(err)
	}
	h.executor.Pump(10000, true, 10000)
	if s.Loops[0].Count != 36 || len(h.events) != 20 {
		t.Fatal("catch-up caps differ", s.Loops[0].Count, len(h.events))
	}
	h.busy = true
	if err := s.TickTime(15000); err != nil {
		t.Fatal(err)
	}
	h.busy = false
	if err := s.TickTime(15049); err != nil {
		t.Fatal(err)
	}
	h.executor.Pump(15049, true, 10000)
	if len(h.events) != 20 {
		t.Fatal("busy time leaked into the game clock", len(h.events))
	}
	// Neither the timed loop nor calctime runs before the next 50 ms boundary.
	h.hook = func(task *Task, handler string) error {
		if handler == "wait" {
			task.NextFrame()
		}
		return nil
	}
	s.MakeLoop("scene", "scene2", "wait", 1)
	s.ServiceFrameLoops()
	h.executor.Pump(15050, true, 100)
	if !h.nav || !h.fromScript || h.active == 0 {
		t.Fatal("scene loop did not keep navigation armed across a wait")
	}
	h.executor.CancelAll()
	if h.nav || h.fromScript || h.active != 0 {
		t.Fatal("cancelled loop left navigation or busy state active")
	}
}
