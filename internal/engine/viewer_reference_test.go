package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"os"
	"reflect"
	"testing"
)

type viewerFrameReference struct {
	Width, Height int
	Pixels, Z     string
	Camera        *CameraPose
}
type viewerStateReference struct {
	Scene, View int
	Animating   bool
	Frame       *viewerFrameReference
	Camera      *WorldCamera
}

func viewerFrameSnapshot(f *CachedFrame) *viewerFrameReference {
	if f == nil {
		return nil
	}
	hash := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	z := ""
	if f.Z != nil {
		z = hash(f.Z)
	}
	return &viewerFrameReference{f.Width, f.Height, hash(f.Pixels), z, f.Camera}
}
func TestViewerReference(t *testing.T) {
	path := os.Getenv("TAOOT_VIEWER_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_VIEWER_REFERENCE to owned-room navigation output")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type road struct {
		Name             string
		Register, Arrive int
	}
	var corpus []struct {
		Path  string
		Modes []struct {
			Mode   string
			Stands []struct {
				viewerStateReference
				Roads    []road
				Listener *Listener
				Hits     []struct {
					X, Y float64
					Hit  string
				}
			}
			Actions []struct {
				Scene, View int
				Command     string
				Pace        float64
				Steps       []struct {
					Now float64
					viewerStateReference
				}
			}
		}
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			set, err := df.ReadSet(data)
			if err != nil {
				t.Fatal(err)
			}
			for _, mode := range entry.Modes {
				t.Run(mode.Mode, func(t *testing.T) {
					s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
					defer s.Close()
					s.PictureMode = mode.Mode
					v, err := NewSetViewer(set, s, "", "", nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					// This corpus measures authored frame selection, navigation and geometry.
					// Lifecycle dispatch is exercised independently with cooperative handlers.
					v.Scripts.Main = nil
					clear(v.Scripts.Scenes)
					cache := map[*CachedFrame]*viewerFrameReference{}
					state := func() viewerStateReference {
						f := v.RoomFrame()
						desc := cache[f]
						if f != nil && desc == nil {
							desc = viewerFrameSnapshot(f)
							cache[f] = desc
						}
						return viewerStateReference{v.SceneIdx, v.ViewIdx, v.Animating(), desc, v.RoomCamera()}
					}
					for _, stand := range mode.Stands {
						clear(cache)
						scene := set.Scenes[stand.Scene]
						if _, err = v.JumpTo(scene.SceneName, scene.Views[stand.View].ViewName); err != nil {
							t.Fatal(err)
						}
						if got := state(); !reflect.DeepEqual(got, stand.viewerStateReference) {
							t.Fatalf("standpoint %d/%d differs\ngot %+v\nwant %+v", stand.Scene, stand.View, got, stand.viewerStateReference)
						}
						roads := []road{}
						for _, r := range v.AvailableRoads() {
							roads = append(roads, road{r.Transition.TransitionName, r.Register, r.ArriveViewID})
						}
						if !reflect.DeepEqual(roads, stand.Roads) || !reflect.DeepEqual(s.Listener(), stand.Listener) {
							t.Fatal("roads or listener differ", stand.Scene, stand.View)
						}
						for _, pt := range stand.Hits {
							name := ""
							if hit := v.HitTest(pt.X, pt.Y); hit != nil {
								name = hit.Object.Identifier
							}
							if name != pt.Hit {
								t.Fatal("hotspot selection differs", stand.Scene, stand.View, pt.X, pt.Y, name, pt.Hit)
							}
						}
					}
					for _, action := range mode.Actions {
						clear(cache)
						scene := set.Scenes[action.Scene]
						if _, err = v.JumpTo(scene.SceneName, scene.Views[action.View].ViewName); err != nil {
							t.Fatal(err)
						}
						s.Pump(s.Executor.Now(), false, 10000)
						switch action.Command {
						case "walk":
							err = v.Walk(action.Pace)
						case "left":
							err = v.Turn(LeftTurns, action.Pace)
						case "right":
							err = v.Turn(RightTurns, action.Pace)
						}
						if err != nil {
							t.Fatal(err)
						}
						for i, step := range action.Steps {
							if i > 0 {
								if _, err = v.AdvanceRoom(step.Now); err != nil {
									t.Fatal(err)
								}
							}
							got := state()
							if !reflect.DeepEqual(got, step.viewerStateReference) {
								a, _ := json.Marshal(got)
								b, _ := json.Marshal(step.viewerStateReference)
								t.Fatal(fmt.Sprintf("%d/%d %s pace %g step %d differs\ngot %s\nwant %s", action.Scene, action.View, action.Command, action.Pace, i, a, b))
							}
						}
						if v.Animating() {
							t.Fatal("movement did not settle")
						}
						s.Pump(s.Executor.Now(), false, 10000)
					}
				})
			}
		})
	}
}
