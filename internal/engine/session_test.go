package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"reflect"
	"testing"
)

func TestSessionPollingUsesHostFramesAndLogicalClock(t *testing.T) {
	s := builtinTestSession(t)
	s.HasRealFrames = true
	s.PointerDown = true
	var result script.Value
	task := s.Track("button", false, func(*Task) error {
		var err error
		result, err = s.Interp.Builtins["button"](s.Interp, nil, nil, nil)
		return err
	})
	s.Pump(100, false, 100)
	if task.Done() || !s.ScriptBusy() || !s.PollingInput() || s.RealYieldSeq != 1 {
		t.Fatal("button did not yield to a real frame")
	}
	s.PointerDown = false
	s.Pump(101, false, 100)
	if task.Done() {
		t.Fatal("clock-only pump released frame wait")
	}
	s.Pump(102, true, 100)
	if !task.Done() || task.Err() != nil || result.Num() != 0 || s.ScriptBusy() {
		t.Fatal("button did not read the updated pointer after frame")
	}
	if err := s.TickTime(1000); err != nil {
		t.Fatal(err)
	}
	delayed := s.Track("delay", false, func(*Task) error {
		_, err := s.Interp.Builtins["delay"](s.Interp, []script.Value{script.Num(3)}, nil, nil)
		return err
	})
	s.Pump(1000, false, 100)
	s.Pump(1049, false, 100)
	if delayed.Done() {
		t.Fatal("delay used stale scheduler time")
	}
	s.Pump(1050, false, 100)
	if !delayed.Done() || delayed.Err() != nil {
		t.Fatal("delay missed its deadline")
	}
}

// The original idle handler completes before room-entry events. JavaScript's
// implicit awaits can lose a clock increment when both handlers read the same
// value. Keep each script assignment synchronous, including at a clock boundary.
func TestClockAndRoomEntryPreserveBothIncrements(t *testing.T) {
	s := builtinTestSession(t)
	increment := script.Stmt{Kind: "assign", Name: "seconds", Value: &script.Expr{
		Kind: "binary", Text: "+", Left: &script.Expr{Kind: "variable", Text: "seconds"}, Right: &script.Expr{Kind: "number", Number: 1},
	}}
	inst := &script.Instance{Name: "clock", Script: &script.Script{Handlers: map[string]*script.Handler{}}}
	for _, name := range []string{"calctime", "openscene"} {
		inst.Script.Handlers[name] = &script.Handler{Name: name, Body: []script.Stmt{{Kind: "global", Names: []string{"seconds"}}, increment}}
	}
	s.Interp.Fallbacks = []*script.Instance{inst}
	s.Interp.Globals.Set("seconds", script.Num(39))
	if err := s.TickTime(50); err != nil {
		t.Fatal(err)
	}
	if err := s.TickTime(100); err != nil {
		t.Fatal(err)
	}
	if s.ScriptBusy() {
		t.Fatal("idle clock blocked player input")
	}
	entry := s.Track("room entry", false, func(*Task) error {
		_, err := s.RunGlobal("openscene", nil)
		return err
	})
	s.Pump(100, true, 100)
	if !entry.Done() || entry.Err() != nil {
		t.Fatal("room entry did not complete", entry.Err())
	}
	value, _ := s.Interp.Globals.Get("seconds")
	if value.Num() != 41 {
		t.Fatalf("lost an increment: got %g, want 41", value.Num())
	}
}
func TestSessionNestedDegreeEventsAndAbandon(t *testing.T) {
	s := builtinTestSession(t)
	call := func(name string, args ...script.Value) script.Stmt {
		expr := &script.Expr{Kind: "call", Text: name, Opcode: 1}
		for _, v := range args {
			e := &script.Expr{Kind: "number", Number: v.Number}
			if v.IsString {
				e.Kind = "string"
				e.Text = v.Text
			}
			expr.Args = append(expr.Args, e)
		}
		return script.Stmt{Kind: "call", Value: expr}
	}
	p := s.Props.Get("door")
	inst := &script.Instance{Name: "test", Script: &script.Script{Handlers: map[string]*script.Handler{
		"select": {Name: "select", Body: []script.Stmt{call("propdeg", script.Str("door"), script.Num(1)), call("nested"), call("propview", script.Str("door"), script.Str("plain"))}},
		"open":   {Name: "open", Body: []script.Stmt{call("propview", script.Str("door"), script.Str("plain"))}},
		"cancel": {Name: "cancel", Body: []script.Stmt{call("abandon"), call("propvisible", script.Str("door"), script.Num(1))}},
	}}}
	s.Interp.Register("nested", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		before := s.Interp.CurrentEvent
		_, err := s.Interp.Run(inst, "open", nil, script.CallContext{}, f)
		if before == 0 || s.Interp.CurrentEvent != before {
			t.Error("nested handler lost its caller event")
		}
		return script.Num(0), err
	})
	s.Interp.Register("abandon", func(*script.Interpreter, []script.Value, *script.Expr, *script.Frame) (script.Value, error) {
		s.Interp.Abandon()
		return script.Num(0), nil
	})
	run := func(name string) {
		t.Helper()
		task := s.Track(name, false, func(*Task) error {
			_, err := s.Interp.Run(inst, name, nil, script.CallContext{Me: "door"}, nil)
			return err
		})
		s.Pump(0, false, 1000)
		if !task.Done() || task.Err() != nil {
			t.Fatal("handler failed", task.Err())
		}
		if s.Interp.CurrentEvent != 0 {
			t.Fatal("event was not restored")
		}
	}
	run("select")
	if !p.FrameLocked || p.Animating {
		t.Fatal("same-event degree selection did not hold its frame")
	}
	run("open")
	if p.FrameLocked || !p.Animating {
		t.Fatal("later event failed to start animation")
	}
	run("cancel")
	if p.Visible {
		t.Fatal("abandoned script mutated the new game state")
	}
}
func TestSetScriptsLifecycleAndPaintingRouting(t *testing.T) {
	s := builtinTestSession(t)
	b := NewSetScripts(&df.Set{SetName: "ROOM", File: &df.File{}, Scenes: []df.Scene{{Index: 0, SceneName: "hall", Views: []df.SceneView{{ViewName: "front", Objects: []df.ObjectEntry{{Identifier: "painting"}}}}}}}, s)
	var got []string
	s.Interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		got = append(got, f.Instance.Name)
		if !b.InLifecycle() && f.Handler == "openscene" {
			t.Error("lifecycle context absent")
		}
		return script.Num(0), nil
	})
	b.Main = dispatchInstance("set", "openscene", "")
	b.Scenes = []*script.Instance{dispatchInstance("scene", "openscene", "passcode")}
	s.StageScript = dispatchInstance("stage", "openscene", "")
	b.OpenScene(0)
	if !reflect.DeepEqual(got, []string{"scene", "set"}) || b.LastSceneIdx != 0 || b.InLifecycle() {
		t.Fatal("lifecycle chain mismatch", got)
	}
	got = nil
	s.RestoringSave = true
	b.OpenScene(0)
	if len(got) != 0 {
		t.Fatal("load replayed initialization")
	}
	s.RestoringSave = false
	b.Objects.Set("0:0:0", dispatchInstance("painting", "mousedown", "passcode"))
	b.Scenes[0] = dispatchInstance("scene", "mousedown", "")
	if !b.PaintingEvent("HALL", "FRONT", "PAINTING", "mousedown", nil, nil) || !reflect.DeepEqual(got, []string{"painting", "scene"}) {
		t.Fatal("painting did not bubble through scene", got)
	}
}
