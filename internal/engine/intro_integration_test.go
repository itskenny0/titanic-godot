package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type introMilestone struct {
	Label, Set, Scene, View string
	Movie                   any
	Now                     float64
	Frame                   float64
	InputLocked             bool
}

// Run the original raid handlers from every London room. Audio completion is
// simulated from sample durations; the camera, movies and scripts use owned data.
func TestNativeAirRaidIntegration(t *testing.T) {
	files := ownedFiles(t)
	for _, standpoint := range []struct{ scene, view string }{{"scene1", "view31"}, {"scene2", "view14"}, {"scene3", "view22"}} {
		t.Run(standpoint.scene, func(t *testing.T) {
			var milestones []introMilestone
			p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
			defer p.Close()
			if err := p.Boot(PlayerConfig{Index: files.Index}); err != nil {
				t.Fatal(err)
			}
			ends := map[uint64]float64{}
			step := func() {
				for id, end := range ends {
					if p.now >= end {
						p.Command(PlayerCommand{Action: "audio_done", ID: id})
						delete(ends, id)
					}
				}
				if err := p.Tick(50); err != nil {
					t.Fatal(err)
				}
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
			until := func(label string, done func() bool) {
				t.Helper()
				for i := 0; i < 1200; i++ {
					if done() {
						state := p.State()
						milestones = append(milestones, introMilestone{label, state["set"].(string), state["scene"].(string), state["view"].(string), state["movie"], p.now, p.Host.Session.Clock.FrameCounter, p.Host.Director.InputLocked()})
						return
					}
					step()
				}
				t.Fatal(label, p.State(), p.Host.Session.Pending())
			}
			until("logo", func() bool { return p.Host.Director.Movies.Playing() })
			p.Command(PlayerCommand{Action: "key", Key: ".", Special: true})
			until("menu", func() bool { return p.State()["movie"] == "playmode.mov" })
			p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: 266, Y: 254})
			p.Command(PlayerCommand{Action: "pointer", Kind: "release", X: 266, Y: 254})
			until("date", func() bool { return p.State()["movie"] == "datebed.mov" })
			p.Command(PlayerCommand{Action: "key", Key: ".", Special: true})
			until("room", func() bool { return p.State()["set"] == "bedsit1" && !p.Host.Director.InputLocked() })
			if ok, err := p.Host.Viewer.JumpTo(standpoint.scene, standpoint.view); err != nil || !ok {
				t.Fatal("standpoint", ok, err)
			}
			// Interaction normally arms this same loop after the player explores.
			p.Host.Session.Scheduler.MakeLoop("scene", "scene1", "bomb", 1)
			started := p.now
			until("bomb impact", func() bool { return p.State()["movie"] == "bedex.mov" })
			if p.Host.Session.CurrentViewName() != "view31" || p.now-started > 30000 {
				t.Fatal("raid missed its window or timing", p.now-started, p.State())
			}
			until("opening credits", func() bool { return p.State()["movie"] == "ocredits.mov" })
			p.Command(PlayerCommand{Action: "key", Key: ".", Special: true})
			until("leave London", func() bool { return p.State()["set"] != "bedsit1" })
			if root := os.Getenv("TAOOT_INTRO_REFERENCE"); root != "" {
				data, err := os.ReadFile(filepath.Join(root, "sequence-"+standpoint.scene+"-reference.json"))
				if err != nil {
					t.Fatal(err)
				}
				var expected []introMilestone
				if err := json.Unmarshal(data, &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(milestones, expected) {
					got, _ := json.Marshal(milestones)
					t.Fatalf("intro sequence differs from TypeScript\ngot %s\nwant %s", got, data)
				}
			}
		})
	}
}
