package engine

import (
	"encoding/json"
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"os"
	"reflect"
	"testing"
)

func builtinTestSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
	t.Cleanup(s.Close)
	s.SeedRandom(123)
	s.Listener = func() *Listener { return &Listener{X: -123, Y: 456, Deg: 511} }
	s.HitTestAt = func(x, y float64) HitTarget { return HitTarget{fmt.Sprintf("hit:%g,%g", x, y), "actor"} }
	s.PointInSet = func(x, y float64) bool { return x >= 0 && y >= 0 }
	s.PointInStage = func(x, y float64) bool { return x < 0 }
	states := []df.PropState{{Identifier: "open", Animated: true, Frames: []int{1, 2, 3, 4}, Degrees: []int{0, 0, 1, 1}}, {Identifier: "dial", Frames: []int{1, 2, 3}, Degrees: []int{0, 85, 170}}, {Identifier: "plain", Animated: true, Frames: []int{1, 2, 3}}}
	shop := s.Props.AddShop("test.shp", &df.Shop{Groups: []df.PropGroup{{Name: "door", States: states}}})
	shop.frames = map[int]*df.Sprite{}
	for i := 1; i <= 4; i++ {
		shop.frames[i] = &df.Sprite{Width: 4, Height: 3, PosXraw: 2, PosYraw: 1, Indexed: make([]byte, 12), Opaque: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	}
	return s
}
func callBuiltin(t *testing.T, s *Session, name string, args ...script.Value) script.Value {
	t.Helper()
	fn := s.Interp.Builtins[name]
	if fn == nil {
		t.Fatal("missing builtin", name)
	}
	var value script.Value
	task := s.Track(name, false, func(*Task) error { var err error; value, err = fn(s.Interp, args, nil, nil); return err })
	s.Pump(s.Executor.Now(), false, 10000)
	if !task.Done() {
		t.Fatal(name, "did not complete")
	}
	if task.Err() != nil {
		t.Fatal(task.Err())
	}
	return value
}
func scriptValue(v any) script.Value {
	if str, ok := v.(string); ok {
		return script.Str(str)
	}
	return script.Num(v.(float64))
}
func builtinProps(s *Session) []any {
	out := []any{}
	for name, p := range s.Props.Props.All() {
		value := func(v script.Value) any {
			if v.IsString {
				return v.Text
			}
			return v.Number
		}
		out = append(out, map[string]any{"name": name, "state": p.StateName, "visible": p.Visible, "hidden": p.Hidden, "x": p.AnchorX, "y": p.AnchorY, "world": p.WorldSpace, "wx": p.WorldX, "wy": p.WorldY, "wz": p.WorldZ, "scale": p.Scale, "deg": value(p.Deg), "degEvent": p.DegEvent, "variants": p.DegVariants, "locked": p.FrameLocked, "animating": p.Animating, "index": p.FrameIdx, "order": p.FrameOrder, "owner": value(p.Owner), "value": value(p.Value)})
	}
	return out
}
func TestBuiltinReference(t *testing.T) {
	path := os.Getenv("TAOOT_BUILTIN_REFERENCE")
	if path == "" {
		path = "../../tests/fixtures/builtins.json"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var calls []struct {
		SetupScene bool
		Name       string
		Args       []any
		Result     any
		Props      any
	}
	if err = json.Unmarshal(b, &calls); err != nil {
		t.Fatal(err)
	}
	s := builtinTestSession(t)
	for idx, call := range calls {
		if call.SetupScene {
			s.SetCurrentSetName("room")
			s.CurrentSceneName = func() string { return "Scene1" }
			s.CurrentViewName = func() string { return "Front" }
			s.Actors.AddCast("test.cst", &df.Cast{Members: []df.CastMember{{Name: "alice"}}})
		}
		t.Run(fmt.Sprintf("%03d_%s", idx, call.Name), func(t *testing.T) {
			args := make([]script.Value, len(call.Args))
			for i, v := range call.Args {
				args[i] = scriptValue(v)
			}
			got := callBuiltin(t, s, call.Name, args...)
			var result any
			if got.IsString {
				units := script.UTF16Units(got.Text)
				result = map[string]any{"text": units}
			} else {
				var num any = got.Number
				if math.IsNaN(got.Number) || math.IsInf(got.Number, 0) {
					num = nil
				}
				result = map[string]any{"num": num}
			}
			if !reflect.DeepEqual(normalizedJSON(result), call.Result) {
				t.Errorf("%s%v result got %v want %v", call.Name, call.Args, normalizedJSON(result), call.Result)
			}
			if !reflect.DeepEqual(normalizedJSON(builtinProps(s)), call.Props) {
				t.Errorf("prop state differs after %s%v\ngot %v\nwant %v", call.Name, call.Args, normalizedJSON(builtinProps(s)), call.Props)
			}
		})
	}
}
