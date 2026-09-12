package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestStageReference(t *testing.T) {
	path := os.Getenv("TAOOT_STAGE_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_STAGE_REFERENCE to owned-data stage image hashes")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path, RefName string
		Frames        []struct {
			Name                 string
			Index, Width, Height int
			Pixels, Palette      *string
			Regions              []df.StageRegion
		}
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	count := 0
	hash := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	for _, e := range corpus {
		b, err := os.ReadFile(e.Path)
		if err != nil {
			t.Fatal(err)
		}
		stg, err := df.ReadStage(b)
		if err != nil {
			t.Fatal(err)
		}
		d := NewScriptDispatch(script.NewInterpreter(), new(PropRuntime), new(ActorRuntime))
		s := NewStageController(d, new(FadeState), nil)
		s.File = stg
		s.Name = "test.stg"
		for _, f := range stg.Flats {
			s.FlatNames = append(s.FlatNames, f.Name)
		}
		if len(stg.Flats) > 0 {
			s.GotoFlat(stg.Flats[0].Name)
		}
		if s.StageRefName() != e.RefName {
			t.Fatal("stage reference name differs", e.Path)
		}
		for _, f := range e.Frames {
			s.GotoFlat(f.Name)
			image := s.FlatImage(s.CurrentFlat)
			if f.Pixels == nil {
				if image != nil {
					t.Fatal("expected missing flat image", e.Path, f.Name)
				}
			} else {
				if image == nil || image.Width != f.Width || image.Height != f.Height || hash(image.Pixels) != *f.Pixels || hash(image.Palette) != *f.Palette {
					t.Fatal("flat pixels or palette differ", e.Path, f.Name)
				}
				if s.FlatImage(s.CurrentFlat) != image {
					t.Fatal("flat image was decoded twice")
				}
			}
			if s.FlatToIndex(f.Name) != f.Index || !reflect.DeepEqual(s.CurrentFlatRegions(), f.Regions) {
				t.Fatal("flat index or regions differ", e.Path, f.Name)
			}
			count++
		}
	}
	t.Logf("Compared %d flat visits across %d stages", count, len(corpus))
}
func TestStageLifecycleAndFlatSelection(t *testing.T) {
	interp := script.NewInterpreter()
	d := NewScriptDispatch(interp, new(PropRuntime), new(ActorRuntime))
	calls := []string{}
	interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		calls = append(calls, f.Instance.Name+"."+f.Handler)
		return script.Num(0), nil
	})
	s := NewStageController(d, new(FadeState), nil)
	s.Name = "test.stg"
	s.CurrentFlat = "one"
	s.FlatNames = []string{"one", "Two"}
	s.File = &df.Stage{File: &df.File{}}
	s.ClearText = func() { calls = append(calls, "clear") }
	s.ResetPlugins = func() { calls = append(calls, "reset") }
	for _, name := range s.FlatNames {
		inst := dispatchInstance(name, "closeflat", "")
		inst.Script.Handlers["openflat"] = dispatchInstance(name, "openflat", "").Script.Handlers["openflat"]
		d.FlatScripts.Set(strings.ToLower(name), inst)
	}
	d.StageScript = dispatchInstance("test.stg", "closestage", "")
	d.BootScripts = []*script.Instance{dispatchInstance("boot", "boot", "")}
	d.RefreshFallbacks()
	s.GotoFlat("2")
	s.CloseStageFile()
	want := []string{"one.closeflat", "clear", "Two.openflat", "test.stg.closestage", "Two.closeflat", "reset"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatal("flat/stage lifecycle ordering differs", calls)
	}
	if s.Name != "none" || s.CurrentFlat != "none" || s.File != nil || d.StageScript != nil || d.FlatScripts.Len() != 0 || len(d.Interp.Fallbacks) != 1 || d.Interp.Fallbacks[0] != d.Boot() {
		t.Fatal("closed stage left stale script bindings")
	}
	s.ResetOverlayStack()
	for _, name := range []string{"savestage1", "saveflat1", "savestage3", "saveflat3"} {
		v, ok := interp.Globals.Get(name)
		if !ok || v.String() != "" {
			t.Fatal("overlay stack was not cleared")
		}
	}
}
func TestStageHotspotBoundariesAndButtonContext(t *testing.T) {
	interp := script.NewInterpreter()
	d := NewScriptDispatch(interp, new(PropRuntime), new(ActorRuntime))
	calls := [][3]string{}
	interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		calls = append(calls, [3]string{f.Instance.Name, f.Context.Me, f.Context.Target})
		return script.Num(0), nil
	})
	s := NewStageController(d, new(FadeState), nil)
	s.Name = "test.stg"
	s.CurrentFlat = "main"
	s.File = &df.Stage{File: &df.File{}}
	s.regions["test.stg:main"] = []df.StageRegion{{Name: "button", Left: 10, Top: 5, Right: 20, Bottom: 15}}
	flat := dispatchInstance("main", "mousedown", "")
	d.FlatScripts.Set("main", flat)
	d.StageScript = dispatchInstance("stage", "mousedown", "")
	point := [2]float64{}
	s.SetPointer = func(x, y float64) { point = [2]float64{x, y} }
	handled, err := s.ClickAt(20, 15)
	if err != nil || handled || len(calls) != 2 || point != [2]float64{20, 15} {
		t.Fatal("inclusive hotspot or fallback dispatch differs", handled, calls, err)
	}
	calls = nil
	if _, err = s.ClickAt(21, 15); err != nil || len(calls) != 0 {
		t.Fatal("outside hotspot dispatched input")
	}
	if _, err = s.SendToButton("main", "BUTTON", "mousedown", nil, "caller", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, [][3]string{{"main", "button", "button"}}) {
		t.Fatal("button fallback lost its target context", calls)
	}
}
