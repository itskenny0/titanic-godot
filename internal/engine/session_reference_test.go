package engine

import (
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/script"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSessionBootReference(t *testing.T) {
	path, root := os.Getenv("TAOOT_SESSION_REFERENCE"), os.Getenv("TAOOT_GAME_DATA")
	if path == "" || root == "" {
		t.Skip("set TAOOT_SESSION_REFERENCE and TAOOT_GAME_DATA for owned-data startup comparison")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	index := map[string]string{}
	for _, disc := range []string{"cd1", "cd2"} {
		err = filepath.WalkDir(filepath.Join(root, disc), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				key := strings.ToLower(d.Name())
				if index[key] == "" {
					index[key] = path
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	s := NewSession(func(name string) ([]byte, error) {
		path := index[strings.ToLower(filepath.Base(name))]
		if path == "" {
			return nil, nil
		}
		return os.ReadFile(path)
	}, new(HostAudio))
	defer s.Close()
	s.SeedRandom(123)
	unknown := []string{}
	s.Interp.OnUnknown = func(name string, _ []script.Value) { unknown = append(unknown, name) }
	task := s.Track("boot", false, func(*Task) error { s.EnsureBooted(); return nil })
	s.Pump(0, false, 100000)
	if !task.Done() || task.Err() != nil {
		t.Fatal("boot did not complete", task.Err())
	}
	value := func(v script.Value) any {
		if v.IsString {
			return v.Text
		}
		return v.Number
	}
	globals := []any{}
	for _, key := range s.Interp.Globals.Keys() {
		v, _ := s.Interp.Globals.Get(key)
		globals = append(globals, []any{key, value(v)})
	}
	props := []any{}
	for name, p := range s.Props.Props.All() {
		props = append(props, map[string]any{"name": name, "owner": value(p.Owner), "value": value(p.Value), "view": p.StateName, "visible": p.Visible, "deg": value(p.Deg), "scale": p.Scale, "x": p.AnchorX, "y": p.AnchorY})
	}
	actors := []any{}
	for name, a := range s.Actors.Actors.All() {
		actors = append(actors, map[string]any{"name": name, "owner": value(a.Owner), "value": value(a.Value), "pose": a.PoseName, "visible": a.Visible, "deg": a.Deg, "set": a.SetName, "star": a.StarName})
	}
	got := normalizedJSON(map[string]any{"globals": globals, "shops": s.Props.Shops.keys, "casts": s.Actors.Casts.keys, "stage": s.StageCtrl.Name, "flat": s.StageCtrl.CurrentFlat, "tracks": s.AudioLib.BankNames(), "props": props, "actors": actors, "unknown": unknown}).(map[string]any)
	for key, expected := range want {
		if !reflect.DeepEqual(got[key], expected) {
			t.Errorf("startup %s differs\ngot %v\nwant %v", key, got[key], expected)
		}
	}
}
