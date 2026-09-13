package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/script"
)

// Replay the same timed inputs after loading the user's checkpoints in each
// engine. The local corpus contains save paths and state, never bundled assets.
// A reference with serialized idle dispatch avoids the old JavaScript lost-update
// race between calctime and openscene; see TestClockAndRoomEntryPreserveBothIncrements.
func TestSavedPlayReference(t *testing.T) {
	root := os.Getenv("TAOOT_PLAY_REFERENCE")
	if root == "" {
		t.Skip("set TAOOT_PLAY_REFERENCE to local TypeScript saved-game replay output")
	}
	files := ownedFiles(t)
	paths, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatal("missing replay corpus", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var run struct {
			Path    string
			Samples []struct {
				Command *PlayerCommand
				State   map[string]any
				Globals [][]any
			}
		}
		if err := json.Unmarshal(data, &run); err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(run.Path), func(t *testing.T) {
			p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
			defer p.Close()
			if err := p.Boot(PlayerConfig{Index: files.Index, Save: run.Path}); err != nil {
				t.Fatal(err)
			}
			p.Host.Session.SeedRandom(123)
			if os.Getenv("TAOOT_PLAY_TRACE") != "" {
				s := p.Host.Session
				for _, name := range []string{"clockcount", "sec", "sinkflag"} {
					s.Interp.WatchGlobals[name] = true
				}
				s.Interp.OnGlobalChange = func(name string, before, after script.Value) {
					t.Logf("%g %s %s %s: %s -> %s", p.now, s.Executor.Current().Name, s.CurrentViewName(), name, before, after)
				}
			}
			ends := map[uint64]float64{}
			collect := func() {
				for _, raw := range p.Events() {
					if e, ok := raw.(AudioEvent); ok {
						if e.Type == "audio_play" {
							p.TakeAudio(e.ID)
							if !e.Loop {
								ends[e.ID] = p.now + float64(e.Samples)*1000/float64(e.Rate)
							}
						}
						if e.Type == "audio_stop" {
							delete(ends, e.ID)
						}
					}
					if e, ok := raw.(map[string]any); ok && e["type"] == "error" {
						t.Fatal(e["text"])
					}
				}
			}
			collect()
			for i, sample := range run.Samples {
				if sample.Command != nil {
					if err := p.Command(*sample.Command); err != nil {
						t.Fatal(err)
					}
					collect()
				}
				for id, end := range ends {
					if p.now >= end {
						p.Command(PlayerCommand{Action: "audio_done", ID: id})
						delete(ends, id)
					}
				}
				if err := p.Tick(50); err != nil {
					t.Fatal(err)
				}
				collect()
				if got := normalizedJSON(p.State()); !reflect.DeepEqual(got, sample.State) {
					t.Fatalf("frame %d state differs\ngot %v\nwant %v", i, got, sample.State)
				}
				globals := [][]any{}
				for _, name := range p.Host.Session.Interp.Globals.Keys() {
					v, _ := p.Host.Session.Interp.Globals.Get(name)
					var value any = v.Number
					if v.IsString {
						value = v.Text
					}
					globals = append(globals, []any{name, value})
				}
				if !reflect.DeepEqual(globals, sample.Globals) {
					for j, g := range globals {
						if j >= len(sample.Globals) || !reflect.DeepEqual(g, sample.Globals[j]) {
							var expected any
							if j < len(sample.Globals) {
								expected = sample.Globals[j]
							}
							t.Fatalf("frame %d global %d differs: got %v, want %v", i, j, g, expected)
						}
					}
					t.Fatalf("frame %d global count differs: got %d, want %d", i, len(globals), len(sample.Globals))
				}
			}
		})
	}
}
