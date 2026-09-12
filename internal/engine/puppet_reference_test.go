package engine

import (
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"os"
	"reflect"
	"testing"
)

func TestPuppetReference(t *testing.T) {
	path := os.Getenv("TAOOT_PUPPET_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_PUPPET_REFERENCE to local dialogue reference output")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path          string
		DefaultStance int
		DefaultPose   *df.PuppetAnimFrame
		Lines         []struct {
			Ident, Subtitle         string
			Stance                  int
			Base, First, At50, Last *df.PuppetAnimFrame
			Seconds                 *float64
		}
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			e := NewExecutor()
			defer e.Close()
			d := NewScriptDispatch(script.NewInterpreter(), new(PropRuntime), new(ActorRuntime))
			audio := &recordingAudio{}
			c := NewPuppetController(d, e, audio, func(string) ([]byte, error) { return data, nil })
			if !c.OpenPuppetFile("test.pup") {
				t.Fatal("could not open puppet")
			}
			p := c.Puppet
			if p.DefaultStance != entry.DefaultStance || !reflect.DeepEqual(p.DefaultPose, entry.DefaultPose) {
				t.Fatal("initial stance or pose differs")
			}
			now := 100.
			for _, line := range entry.Lines {
				p.Interrupted = false
				p.VoiceQueue = nil
				c.Base(line.Ident)
				if !reflect.DeepEqual(c.Frame(), line.Base) {
					t.Fatal("base pose differs", line.Ident)
				}
				before := len(audio.Calls)
				done := false
				e.Start("reference speech", func(task *Task) error { c.Speak(task, line.Ident); done = true; return nil })
				pumpPuppet(t, c, now)
				if !reflect.DeepEqual(c.Frame(), line.First) || p.StanceIdx != line.Stance || p.Subtitle != line.Subtitle {
					t.Fatal("speech initial pose, stance, or subtitle differs", line.Ident)
				}
				pumpPuppet(t, c, now+50)
				if !reflect.DeepEqual(c.Frame(), line.At50) {
					t.Fatal("speech frame at 50 ms differs", line.Ident)
				}
				c.SkipLine()
				pumpPuppet(t, c, now+51)
				if !done || !reflect.DeepEqual(c.Frame(), line.Last) || p.Anim != nil || p.Subtitle != "" {
					t.Fatal("skipped speech did not settle to reference pose", line.Ident)
				}
				if line.Seconds == nil {
					if len(audio.Calls) != before {
						t.Fatal("unexpected voice playback", line.Ident)
					}
				} else {
					if len(audio.Calls) != before+1 || audio.Calls[len(audio.Calls)-1].Seconds != *line.Seconds {
						t.Fatal("voice duration differs", line.Ident)
					}
				}
				count++
				now += 100
			}
		})
	}
	t.Logf("Compared %d dialogue lines across %d characters", count, len(corpus))
}
