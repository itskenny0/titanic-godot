package engine

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/script"
)

func explorationSave(t *testing.T) string {
	t.Helper()
	save := filepath.Join("..", "..", "originalgame", "deckbd 2026-09-12-21-27-43.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no mission 1 deckbd save in originalgame/")
	}
	return save
}

// startItemRun boots a save with the given item setting and collects item
// and clue events alongside achievements.
func startItemRun(t *testing.T, save string, original bool) (*achRun, *[]map[string]any) {
	t.Helper()
	p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{Index: ownedFiles(t).Index, Save: save, OriginalItems: original, Clues: true}); err != nil {
		t.Fatal(err)
	}
	r := &achRun{t: t, p: p, ends: map[uint64]float64{}, seen: map[string]bool{}}
	var notices []map[string]any
	r.onEvent = func(e map[string]any) {
		if e["type"] == "item" || e["type"] == "clue" {
			notices = append(notices, e)
		}
	}
	r.play(300, nil)
	return r, &notices
}

func (r *achRun) give(item string) {
	s := r.p.Host.Session
	s.Track("give "+item, false, func(*Task) error {
		_, err := s.SendEvent("sendtoshop", "inven.shp", "addinven", achStrs(item), "", nil)
		return err
	})
}

// A received item is announced with its name and picture and goes into the
// bag once the game is idle, so the side panels need not give way to the
// original toolbar.
func TestReceivedItemGoesIntoTheBag(t *testing.T) {
	r, notices := startItemRun(t, explorationSave(t), false)
	if l := r.p.AdaptiveLayout(); !l.Eligible {
		t.Fatalf("the save should start in exploration, got %s", l)
	}
	r.give("cufflink")
	s := r.p.Host.Session
	r.play(200, func() bool {
		hand, _ := s.Interp.Globals.Get("handitem")
		return len(*notices) > 0 && hand.Text == ""
	})
	if len(*notices) != 1 || (*notices)[0]["type"] != "item" || (*notices)[0]["name"] != "Cufflink" {
		t.Fatalf("notices = %v", *notices)
	}
	icon, err := base64.StdEncoding.DecodeString((*notices)[0]["icon"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if img, err := png.Decode(bytes.NewReader(icon)); err != nil || img.Bounds().Dx() < 8 {
		t.Fatalf("icon is not a picture of the item: %v", err)
	}
	hand, _ := s.Interp.Globals.Get("handitem")
	prop := s.Props.Get("cufflink")
	if hand.Text != "" || prop.Visible || prop.Owner.Text != "frank" {
		t.Fatalf("hand %q, visible %v, owner %v: the cufflink should be in the bag", hand.Text, prop.Visible, prop.Owner)
	}
	r.play(30, nil)
	if l := r.p.AdaptiveLayout(); !l.Eligible {
		t.Fatalf("after the item went into the bag the side panels should return, got %s", l)
	}
	// An item taken out of the bag on purpose stays in hand.
	s.Interp.Globals.Set("handitem", script.Str("cufflink"))
	prop.StateName, prop.Visible = "large", true
	r.play(60, nil)
	if hand, _ := s.Interp.Globals.Get("handitem"); hand.Text != "cufflink" || len(*notices) != 1 {
		t.Fatalf("a chosen item was put away or announced again: hand %q, notices %v", hand.Text, *notices)
	}
}

func TestOriginalItemsStayInHand(t *testing.T) {
	r, notices := startItemRun(t, explorationSave(t), true)
	r.give("cufflink")
	r.play(200, nil)
	hand, _ := r.p.Host.Session.Interp.Globals.Get("handitem")
	if hand.Text != "cufflink" || len(*notices) != 0 {
		t.Fatalf("hand %q, notices %v: the original keeps the item in hand", hand.Text, *notices)
	}
	held := false
	for _, c := range r.p.AdaptiveLayout().Controls {
		held = held || c.Hand && c.Name == "cufflink"
	}
	if !held {
		t.Fatalf("the item in hand should be offered in the side panels: %+v", r.p.AdaptiveLayout())
	}
}

// Every clue comes from lines the original scripts actually speak.
func TestCluesAreSpokenByScripts(t *testing.T) {
	root := os.Getenv("TAOOT_GAME_DATA")
	if root == "" {
		t.Skip("set TAOOT_GAME_DATA to check clues against the game scripts")
	}
	list, _, err := loadClues()
	if err != nil {
		t.Fatal(err)
	}
	e := scanAchievementScripts(t, root)
	seen := map[string]bool{}
	for _, c := range list {
		if c.ID == "" || seen[c.ID] || c.Text == "" || len(c.Lines) == 0 {
			t.Errorf("clue %q needs a unique id, text and lines", c.ID)
		}
		seen[c.ID] = true
		for _, line := range c.Lines {
			if !e.speak[line] {
				t.Errorf("%s: no script speaks %s", c.ID, line)
			}
		}
	}
}

// Speaking a clue line in a real game reports the clue once per line.
func TestClueLineIsReported(t *testing.T) {
	r, notices := startItemRun(t, explorationSave(t), false)
	s := r.p.Host.Session
	s.Track("speak", false, func(*Task) error {
		b := s.Interp.Builtins
		if _, err := b["openpuppetfile"](s.Interp, achStrs("purs1.pup"), nil, nil); err != nil {
			return err
		}
		_, err := b["puppetspeak"](s.Interp, achStrs("purs1.73"), nil, nil)
		return err
	})
	r.play(100, func() bool { return len(*notices) > 0 })
	if len(*notices) != 1 || (*notices)[0]["id"] != "barbicon_cabin" {
		t.Fatalf("notices = %v", *notices)
	}
}

// A room change is a short hold; the full-screen sinking movie needs the
// original screen at once.
func TestAdaptiveHoldAroundRoomsAndMovies(t *testing.T) {
	r, _ := startItemRun(t, explorationSave(t), false)
	s := r.p.Host.Session
	if _, err := s.Interp.Builtins["closesetfile"](s.Interp, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if l := r.p.AdaptiveLayout(); l.Eligible || !l.Hold || l.Reason != "room not showing" {
		t.Fatalf("a closed set between rooms should hold the side panels: %+v", l)
	}
	r = startAchRun(t, sinkingSave(t))
	g := r.p.Host.Session.Interp.Globals
	g.Set("phase", script.Num(0))
	g.Set("clock", script.Num(1315))
	r.run("advancephase")
	full := false
	r.play(3000, func() bool {
		if l := r.p.AdaptiveLayout(); l.Reason == "full-screen movie" {
			full = true
			if l.Hold {
				t.Fatalf("a full-screen movie must not hold the side panels")
			}
		}
		return full
	})
	if !full {
		t.Fatal("the sinking movie never played")
	}
}

// Settings are changed from the paused menu and must still apply.
func TestSettingsApplyWhilePaused(t *testing.T) {
	r, notices := startItemRun(t, explorationSave(t), false)
	for _, c := range []PlayerCommand{{Action: "pause", On: true}, {Action: "item_notices"}, {Action: "clues"}, {Action: "achievements"}, {Action: "pause"}} {
		if err := r.p.Command(c); err != nil {
			t.Fatal(err)
		}
	}
	if !r.p.items.Off || r.p.clues || r.p.achievements == nil || !r.p.achievements.Off {
		t.Fatalf("items off %v, clues %v: settings sent while paused were lost", r.p.items.Off, r.p.clues)
	}
	r.give("cufflink")
	r.play(200, nil)
	if hand, _ := r.p.Host.Session.Interp.Globals.Get("handitem"); hand.Text != "cufflink" || len(*notices) != 0 {
		t.Fatalf("hand %q, notices %v: turning item notices off keeps the original", hand.Text, *notices)
	}
}

// Frank carries the baby in his arms, drawn in the picture above the
// toolbar like an open door; the side panels stay while he does.
func TestCarriedBabyKeepsSidePanels(t *testing.T) {
	r, _ := startItemRun(t, explorationSave(t), false)
	s := r.p.Host.Session
	send := func(handler string, args []script.Value) {
		s.Track(handler, false, func(*Task) error {
			_, err := s.SendEvent("sendtoprop", "baby", handler, args, "", nil)
			return err
		})
		r.play(60, nil)
	}
	send("addbaby", nil)
	baby := s.Props.Get("baby")
	rect, err := baby.ScreenRect()
	if err != nil || rect == nil || !baby.Visible || baby.Owner.Text != "frank" {
		t.Fatalf("the baby should be carried: visible %v owner %v rect %v err %v", baby.Visible, baby.Owner, rect, err)
	}
	if rect.Y+float64(rect.H) > 264 {
		t.Fatalf("the carried baby reaches into the toolbar: %+v", rect)
	}
	if l := r.p.AdaptiveLayout(); !l.Eligible {
		t.Fatalf("carrying the baby should keep the side panels, got %s", l)
	}
	send("givebaby", achStrs("shay"))
	if l := r.p.AdaptiveLayout(); !l.Eligible || baby.Visible {
		t.Fatalf("after handing the baby over: %s, visible %v", l, baby.Visible)
	}
}
