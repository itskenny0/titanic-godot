package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/script"
)

func ownedFiles(t *testing.T) *Files {
	t.Helper()
	root := os.Getenv("TAOOT_GAME_DATA")
	if root == "" {
		t.Skip("set TAOOT_GAME_DATA for the native gameplay integration test")
	}
	index := map[string]string{}
	for disc := 1; disc <= 2; disc++ {
		err := filepath.WalkDir(filepath.Join(root, fmt.Sprintf("cd%d", disc)), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				index[fmt.Sprintf("%d/%s", disc, strings.ToLower(entry.Name()))] = path
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return NewFiles(index, os.ReadFile)
}
func TestNativeHostIntegration(t *testing.T) { runNativeHostIntegration(t, ownedFiles(t)) }

func TestNativeHostExtendedIntegration(t *testing.T) {
	root := os.Getenv("TAOOT_MOD_DATA")
	if root == "" {
		t.Skip("set TAOOT_MOD_DATA to the extracted ExtendedMod directory")
	}
	files := ownedFiles(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		for disc := 1; disc <= 2; disc++ {
			files.Index[fmt.Sprintf("%d/%s", disc, name)] = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	runNativeHostIntegration(t, files)
}

func runNativeHostIntegration(t *testing.T, files *Files) {
	audio := new(HostAudio)
	h := NewGameHost(files, audio, HostUI{Log: func(line string) { t.Log(line) }})
	defer h.Close()
	s, d := h.Session, h.Director
	s.HasRealFrames = true
	s.PictureMode = "sharp"
	s.SeedRandom(123)
	unknown := []string{}
	s.Interp.OnUnknown = func(name string, _ []script.Value) { unknown = append(unknown, name) }
	ctx := NewDrawContext(512, 384)
	now := 0.0
	ends := map[uint64]float64{}
	// Audio completions are simulated from sample duration. This checks game
	// sequencing, not a physical device's audio output or timing.
	step := func() {
		now += 50
		for id, end := range ends {
			if now >= end {
				audio.Finish(id)
				delete(ends, id)
			}
		}
		if _, err := d.Tick(now); err != nil {
			t.Fatal(err)
		}
		for _, c := range s.Pump(s.Executor.Now(), true, 100000) {
			if c.Err != nil {
				t.Fatal(c.Name, c.Err)
			}
		}
		if err := d.Render(ctx); err != nil {
			t.Fatal(err)
		}
		for _, e := range audio.DrainEvents() {
			if e.Type == "audio_play" {
				audio.TakePCM(e.ID)
				if !e.Loop {
					ends[e.ID] = now + float64(e.Samples)*1000/float64(e.Rate)
				}
			}
			if e.Type == "audio_stop" {
				delete(ends, e.ID)
			}
		}
	}
	until := func(label string, condition func() bool) {
		t.Helper()
		for i := 0; i < 1200; i++ {
			if condition() {
				return
			}
			step()
		}
		t.Fatalf("%s did not settle: movie=%s set=%s pending=%v fade=%+v", label, d.Movies.PlayingFile(), s.CurrentSetFile, s.Pending(), s.Fade)
	}
	h.Preload()
	boot := s.Track("cold boot", false, func(*Task) error { return h.ColdBoot() })
	until("intro", func() bool { return d.Movies.Playing() })
	if ok, err := d.KeyDown(".", true); err != nil || !ok {
		t.Fatal("intro skip failed", err)
	}
	until("main menu", func() bool { return d.Movies.PlayingFile() == "playmode.mov" && len(d.Movies.WaitingRegions()) > 0 })
	if !d.Screen.FrameValid || !d.AwaitingInput() {
		t.Fatal("main menu was not presented")
	}
	var regionFound bool
	for _, r := range d.Movies.WaitingRegions() {
		if r.Type == 6 {
			if err := d.Click(float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2); err != nil {
				t.Fatal(err)
			}
			regionFound = true
			break
		}
	}
	if !regionFound {
		t.Fatal("new game region missing")
	}
	until("opening date caption", func() bool { return d.Movies.PlayingFile() == "datebed.mov" })
	d.KeyDown(".", true)
	until("first room", func() bool { return boot.Done() && !d.InputLocked() && s.ViewShowing() })
	if s.CurrentSetFile != "bedsit1" || h.Viewer == nil || !d.Screen.FrameValid {
		t.Fatal("new game did not enter bedsit", s.CurrentSetFile)
	}
	scene, view := s.CurrentSceneName(), s.CurrentViewName()
	s.Interp.Globals.Set("native_sentinel", script.Str("saved correctly"))
	saved, err := s.SnapshotSave()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.KeyDown("rightarrow", false); err != nil {
		t.Fatal(err)
	}
	until("navigation", func() bool { return !d.InputLocked() && s.CurrentViewName() != view })
	load := s.Track("load game", false, func(*Task) error { return h.LoadSavedGame(saved) })
	until("save restoration", func() bool { return load.Done() && !d.InputLocked() })
	value, _ := s.Interp.Globals.Get("native_sentinel")
	if s.CurrentSceneName() != scene || s.CurrentViewName() != view || value.String() != "saved correctly" {
		t.Fatal("restored scene, view or globals differ", s.CurrentSceneName(), s.CurrentViewName(), value)
	}
	if len(unknown) > 0 {
		t.Fatal("unimplemented script calls", unknown)
	}
	t.Logf("Native Go boot, menu, navigation and save/load passed at %s/%s/%s", s.CurrentSetFile, scene, view)
}
