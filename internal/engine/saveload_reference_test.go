package engine

import (
	"encoding/binary"
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/script"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func ownedSessionFiles(t *testing.T, root string) *Files {
	t.Helper()
	index := map[string]string{}
	for _, disc := range []string{"1", "2"} {
		err := filepath.WalkDir(filepath.Join(root, "cd"+disc), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				index[disc+"/"+strings.ToLower(d.Name())] = path
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return NewFiles(index, os.ReadFile)
}
func snapshotMetadata(t *testing.T, s *Session) any {
	t.Helper()
	b, err := s.SnapshotSave()
	if err != nil {
		t.Fatal(err)
	}
	var value any
	start := int(binary.LittleEndian.Uint32(b[4:])) + 17
	if err = json.Unmarshal(b[start:len(b)-25], &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func TestSaveLoadReference(t *testing.T) {
	path, root := os.Getenv("TAOOT_LOAD_REFERENCE"), os.Getenv("TAOOT_GAME_DATA")
	if path == "" || root == "" {
		t.Skip("set TAOOT_LOAD_REFERENCE and TAOOT_GAME_DATA for owned-save restoration comparison")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []map[string]any
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, want := range cases {
		t.Run(want["path"].(string), func(t *testing.T) {
			files := ownedSessionFiles(t, root)
			s := NewSession(files.Provide, new(HostAudio))
			defer s.Close()
			s.SeedRandom(123)
			unknown := []string{}
			jumps := [][]string{}
			s.Interp.OnUnknown = func(name string, _ []script.Value) { unknown = append(unknown, name) }
			s.OnDiscChange = files.SetDisc
			s.OnSetChange = func(file, scene, view string) error { jumps = append(jumps, []string{file, scene, view}); return nil }
			data, err := os.ReadFile(want["path"].(string))
			if err != nil {
				t.Fatal(err)
			}
			loaded := false
			task := s.Track("load", false, func(*Task) error {
				s.EnsureBooted()
				s.Interp.Globals.Set("unsaved_future", script.Num(123))
				s.Interp.Globals.Set("__keep", script.Num(27))
				s.CursorDepth = -5
				var err error
				loaded, err = s.LoadGame(data)
				return err
			})
			s.Pump(0, false, 100000)
			if !task.Done() || task.Err() != nil || !loaded {
				t.Fatal("load did not complete", task.Err())
			}
			props := []any{}
			for name, p := range s.Props.Props.All() {
				props = append(props, map[string]any{"name": name, "index": p.FrameIdx, "order": p.FrameOrder, "locked": p.FrameLocked, "animating": p.Animating})
			}
			internal, _ := s.Interp.Globals.Get("__keep")
			got := normalizedJSON(map[string]any{"path": want["path"], "metadata": snapshotMetadata(t, s), "disc": files.ActiveDisc(), "jumps": jumps, "stage": s.StageCtrl.Name, "flat": s.StageCtrl.CurrentFlat, "set": s.SetName, "cursor": s.CursorDepth, "internal": internal.Num(), "unknown": unknown, "props": props}).(map[string]any)
			for key, expected := range want {
				if !reflect.DeepEqual(got[key], expected) {
					t.Errorf("restored %s differs\ngot %v\nwant %v", key, got[key], expected)
				}
			}
		})
	}
}
