package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHintTableParses(t *testing.T) {
	list, conds, err := loadHints()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i, h := range list {
		if h.ID == "" || seen[h.ID] || strings.TrimSpace(h.Text) == "" {
			t.Errorf("hint %d needs a unique id and text: %+v", i, h)
		}
		seen[h.ID] = true
		if n := len(strings.Fields(h.Text)); n > 60 {
			t.Errorf("hint %s is %d words; a hint is a short nudge", h.ID, n)
		}
		if len(conds[i]) == 0 && i != len(list)-1 {
			t.Errorf("hint %s always matches, so every hint after it is unreachable", h.ID)
		}
	}
	if last := list[len(list)-1]; strings.TrimSpace(last.When) != "" {
		t.Errorf("the table must end with a fallback that always matches, not %q", last.When)
	}
}

func startHintRun(t *testing.T, save string, hints bool) (*achRun, *[]map[string]any) {
	t.Helper()
	p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{Index: ownedFiles(t).Index, Save: save, Hints: hints}); err != nil {
		t.Fatal(err)
	}
	r := &achRun{t: t, p: p, ends: map[uint64]float64{}, seen: map[string]bool{}}
	var events []map[string]any
	r.onEvent = func(e map[string]any) {
		switch e["type"] {
		case "help_menu", "hint":
			events = append(events, e)
		}
	}
	r.play(300, nil)
	return r, &events
}

// The Help button of a new voyage opens the Help menu while hints are on,
// and plays the original help movie while they are off.
func TestHelpButtonOffersHints(t *testing.T) {
	save := filepath.Join("..", "..", "originalgame", "bedsit1 2026-09-12-20-27-50.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no bedsit save in originalgame/")
	}
	for _, hints := range []bool{false, true} {
		r, events := startHintRun(t, save, hints)
		r.play(3000, func() bool { return r.p.Host.Session.ViewShowing() && !r.p.Host.Director.InputLocked() })
		var help *AdaptiveControl
		for _, c := range r.p.AdaptiveLayout().Controls {
			if c.Name == "invenhelp" {
				help = &c
			}
		}
		if help == nil {
			t.Fatalf("no Help button: %+v", r.p.AdaptiveLayout())
		}
		movies := r.p.Host.Director.Movies
		r.p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: help.AimX, Y: help.AimY})
		r.play(2, nil)
		r.p.Command(PlayerCommand{Action: "pointer", Kind: "release", X: help.AimX, Y: help.AimY})
		played := false
		r.play(100, func() bool {
			played = played || movies.active != nil && strings.HasPrefix(movies.active.name, "help")
			return played || len(*events) > 0
		})
		if hints && (played || len(*events) != 1 || (*events)[0]["type"] != "help_menu") {
			t.Fatalf("with hints on, Help should open the menu: played %v, events %v", played, *events)
		}
		if !hints && (!played || len(*events) != 0) {
			t.Fatalf("with hints off, Help should play the original help: played %v, events %v", played, *events)
		}
		if !hints {
			continue
		}
		// The menu's "How to play" still plays the original help.
		r.p.Command(PlayerCommand{Action: "original_help"})
		r.play(100, func() bool { return movies.active != nil && strings.HasPrefix(movies.active.name, "help") })
		if movies.active == nil || !strings.HasPrefix(movies.active.name, "help") {
			t.Fatal("How to play did not play the original help")
		}
		r.p.Command(PlayerCommand{Action: "hint"})
		r.play(1, nil)
		last := (*events)[len(*events)-1]
		if last["type"] != "hint" || last["text"] == "" {
			t.Fatalf("no hint: %v", last)
		}
		t.Logf("hint %s: %s", last["id"], last["text"])
	}
}

// Every owned save, from the first voyage to the sinking, gets a hint about
// its own situation rather than the general fallback.
func TestOwnedSavesGetSpecificHints(t *testing.T) {
	saves, _ := filepath.Glob(filepath.Join("..", "..", "originalgame", "*.ti"))
	if len(saves) == 0 {
		t.Skip("no saves in originalgame/")
	}
	ownedFiles(t)
	for _, save := range saves {
		r, events := startHintRun(t, save, true)
		r.play(3000, func() bool { return r.p.Host.Session.ViewShowing() && !r.p.Host.Director.InputLocked() })
		r.p.Command(PlayerCommand{Action: "hint"})
		r.play(1, nil)
		if len(*events) == 0 {
			t.Fatalf("%s: no hint", save)
		}
		h := (*events)[len(*events)-1]
		g := r.p.Host.Session.Interp.Globals
		mission, _ := g.Get("mission")
		phase, _ := g.Get("phase")
		t.Logf("%s [mission %s phase %s, %s]: %s: %s", filepath.Base(save), mission.String(), phase.String(), r.p.Host.Session.CurrentSetFile, h["id"], h["text"])
		if h["id"] == "fallback" {
			t.Errorf("%s gets only the general fallback", filepath.Base(save))
		}
	}
}
