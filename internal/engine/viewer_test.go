package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"reflect"
	"testing"
)

func viewerFixture(t *testing.T) (*SetViewer, *Session) {
	t.Helper()
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
	t.Cleanup(s.Close)
	file := &df.File{Containers: make([]df.Container, 9)}
	for i := 1; i < len(file.Containers); i++ {
		file.Containers[i].Data = []byte{1, 0, 2, 0, 4, byte(i), byte(i)}
	}
	set := &df.Set{File: file, SetName: "room", ColorCount: 256, PaletteRaw: make([]byte, 2048), ViewPortWidth: 512, ViewPortHeight: 264, DefaultSceneName: "scene1", DefaultViewName: "front", Scenes: []df.Scene{{Index: 0, SceneName: "scene1", LocationViews: 10, Views: []df.SceneView{{ViewName: "front", ViewID: 10}, {ViewName: "back", ViewID: 11, Rotation: math.Pi, Rotation8: 128}}}, {Index: 1, SceneName: "scene2", LocationViews: 20, Views: []df.SceneView{{ViewName: "front", ViewID: 20}}}}}
	set.Scenes[0].Turns[0].Frames = []df.FrameInfo{{FrameContainerLoc: 1, FramePairID: 1, MotionInfo: 1, ViewID: 0, PosY16: 30}, {FrameContainerLoc: 2, FramePairID: 2, MotionInfo: 1, ViewID: 1, PosY16: 30, AxisX8: 128}}
	set.Scenes[0].Turns[1].Frames = []df.FrameInfo{{FrameContainerLoc: 3, FramePairID: 1, MotionInfo: 2, ViewID: 0, PosY16: 30}, {FrameContainerLoc: 4, FramePairID: 2, MotionInfo: 2, ViewID: 1, PosY16: 30, AxisX8: 128}}
	set.Scenes[1].Turns[0].Frames = []df.FrameInfo{{FrameContainerLoc: 5, FramePairID: 3, MotionInfo: 1, ViewID: 0, PosY16: 70}}
	set.Scenes[1].Turns[1].Frames = []df.FrameInfo{{FrameContainerLoc: 6, FramePairID: 3, MotionInfo: 2, ViewID: 0, PosY16: 70}}
	set.Transitions = []df.Transition{{ViewIDstart: 10, ViewIDend: 20, TransitionName: "corridor", FrameRegisters: [2]df.FrameRegister{{Destination: 20, Frames: []df.FrameInfo{{FrameContainerLoc: 7, PosY16: 999}, {FrameContainerLoc: 8, PosY16: -999}}}}}}
	v, err := NewSetViewer(set, s, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v, s
}
func TestViewerMovementTimingAndCameraHeight(t *testing.T) {
	v, s := viewerFixture(t)
	if v.current.Pixels[0] != 3 || v.WorldCamera().Z != 30 {
		t.Fatal("sharp standpoint or eye height differs")
	}
	if err := v.Turn(RightTurns, 50); err != nil {
		t.Fatal(err)
	}
	s.Pump(0, false, 100)
	if !v.Animating() || s.CurrentViewName() != "moving" {
		t.Fatal("movement was not advertised")
	}
	if _, err := v.AdvanceRoom(100); err != nil {
		t.Fatal(err)
	}
	if !v.Animating() || v.current.Pixels[0] != 2 {
		t.Fatal("original turn did not present its soft arrival first")
	}
	v.AdvanceRoom(10000)
	s.Pump(10000, false, 100)
	if v.Animating() || v.ViewIdx != 1 || v.current.Pixels[0] != 4 {
		t.Fatal("turn failed to settle at sharp arrival")
	}
	v.JumpTo("scene1", "front")
	if err := v.Walk(0); err != nil {
		t.Fatal(err)
	}
	if v.animation[0].Camera.Z != 30 || v.animation[len(v.animation)-1].Camera.Z != 70 {
		t.Fatal("walk entry/exit changed eye height")
	}
	v.AdvanceRoom(10001)
	s.Pump(10001, false, 100)
	if v.SceneIdx != 1 || v.Animating() || v.current.Pixels[0] != 6 {
		t.Fatal("instant walk did not reach destination")
	}
}
func TestViewerDepartureCompletesBeforeArrivalHandler(t *testing.T) {
	v, s := viewerFixture(t)
	events := []string{}
	s.Interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		events = append(events, f.Handler)
		if f.Handler == "openscene" && !s.NavGestureActive {
			t.Error("arrival script had no navigation hooks")
		}
		return script.Num(0), nil
	})
	close := dispatchInstance("scene1", "closescene", "")
	close.Script.Handlers["closescene"].Body = append(close.Script.Handlers["closescene"].Body, script.Stmt{Kind: "call", Value: &script.Expr{Kind: "call", Text: "delay", Args: []*script.Expr{{Kind: "number", Number: 3}}}})
	close.Script.Handlers["openscene"] = dispatchInstance("scene1", "openscene", "").Script.Handlers["openscene"]
	v.Scripts.Scenes[0] = close
	v.Turn(RightTurns, 0)
	s.Pump(100, false, 100)
	v.AdvanceRoom(100)
	s.Pump(100, false, 100)
	if !reflect.DeepEqual(events, []string{"closescene"}) {
		t.Fatal("arrival ran before departing script completed", events)
	}
	s.Pump(150, false, 100)
	if !reflect.DeepEqual(events, []string{"closescene", "openscene"}) || s.NavGestureActive {
		t.Fatal("arrival dispatch or restored hooks differ", events)
	}
}
func TestViewerHotspotsAndFractionalPointer(t *testing.T) {
	v, s := viewerFixture(t)
	v.View().Objects = []df.ObjectEntry{{Identifier: "large", StartRegionX: 10, StartRegionY: 10, EndRegionX: 0, EndRegionY: 0}, {Identifier: "small", StartRegionX: 2, StartRegionY: 2, EndRegionX: 4, EndRegionY: 4}, {Identifier: "equal", StartRegionX: 2, StartRegionY: 2, EndRegionX: 4, EndRegionY: 4}}
	if hit := v.HitTest(2.5, 3.5); hit == nil || hit.Object.Identifier != "small" {
		t.Fatal("smallest hit rectangle or tie order differs")
	}
	if v.HitTest(-.1, 0) != nil {
		t.Fatal("outside point hit a hotspot")
	}
	shop := s.Props.AddShop("test", testShop())
	seedSprites(shop)
	p := s.Props.Get("door")
	p.Visible = true
	p.AnchorX = .5
	p.AnchorY = .5
	if got, _ := s.Props.PropAtPoint(.5, .5, nil, false, nil); got != p {
		t.Fatal("fractional pointer lost an opaque sprite pixel")
	}
	if got, _ := s.Props.PropAtPoint(.75, .5, nil, false, nil); got != nil {
		t.Fatal("fractional mask index was incorrectly rounded")
	}
	occ := &Occlusion{Z: []byte{0, 0, 0, 0}, W: 2, H: 2}
	if SceneryOccludesPoint(occ, .25, .25, 1) || !SceneryOccludesPoint(occ, .5, .25, 1) {
		t.Fatal("depth mask indexing no longer matches typed-array addressing")
	}
}

func TestScriptedTurnReportsMovementBeforeReturning(t *testing.T) {
	v, s := viewerFixture(t)
	s.HasRealFrames = true
	s.NavFromScript = true
	reported := ""
	s.Track("scripted pan", false, func(*Task) error {
		v.Navigate("right")
		reported = s.CurrentViewName()
		return nil
	})
	s.Pump(0, true, 100)
	if reported != "moving" || !v.Animating() || v.animationPace != EngineStepMS {
		t.Fatal("script cannot observe its turn before polling currentview", reported, v.Animating(), v.animationPace)
	}
	// A subsequent scripted movement waits for this one to settle.
	v.Navigate("left")
	for i := 1; i <= 20; i++ {
		v.AdvanceRoom(float64(i * 50))
		s.Pump(float64(i*50), true, 100)
	}
	if v.Animating() || v.ViewIdx != 0 || len(s.Pending()) != 0 {
		t.Fatal("queued script turn did not settle", v.ViewIdx, s.Pending())
	}
}
