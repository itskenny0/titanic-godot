package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// playAchievements boots a real game, plays it for a while and returns the
// achievements it announced.
func playAchievements(t *testing.T, files *Files, save string, ticks int, unlocked []string) ([]string, *Player) {
	got, _, p := playAchievementsHeard(t, files, save, ticks, unlocked, false)
	return got, p
}

func playAchievementsHeard(t *testing.T, files *Files, save string, ticks int, unlocked []string, off bool) ([]string, []string, *Player) {
	t.Helper()
	p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{Index: files.Index, Save: save, Achievements: unlocked, DisableAchievements: off}); err != nil {
		t.Fatal(err)
	}
	var got, heard []string
	s := p.Host.Session
	speak, voice := s.OnSpeak, s.OnVoice
	s.OnSpeak = func(id string) { heard = append(heard, id); speak(id) }
	s.OnVoice = func(id string) { heard = append(heard, "voice:"+id); voice(id) }
	ends := map[uint64]float64{}
	for i := 0; i < ticks; i++ {
		for id, end := range ends {
			if p.now >= end {
				p.Command(PlayerCommand{Action: "audio_done", ID: id})
				delete(ends, id)
			}
		}
		if err := p.Tick(50); err != nil {
			t.Fatal(err)
		}
		for _, e := range p.Events() {
			switch event := e.(type) {
			case AudioEvent:
				if event.Type == "audio_play" {
					p.TakeAudio(event.ID)
					if !event.Loop {
						ends[event.ID] = p.now + float64(event.Samples)*1000/float64(event.Rate)
					}
				} else if event.Type == "audio_stop" {
					delete(ends, event.ID)
				}
			case map[string]any:
				switch event["type"] {
				case "achievement":
					got = append(got, event["id"].(string))
				case "error":
					t.Fatal(event["text"])
				}
			}
		}
	}
	return got, heard, p
}

func TestAchievementsInOwnedGame(t *testing.T) {
	files := ownedFiles(t)
	saves, _ := filepath.Glob(filepath.Join("..", "..", "originalgame", "*.ti"))
	sort.Strings(saves)
	if len(saves) == 0 {
		t.Skip("no original saves in originalgame/")
	}
	all := map[string]bool{}
	for _, save := range saves {
		got, heard, p := playAchievementsHeard(t, files, save, 600, nil, false)
		m, _ := p.Host.Session.Interp.Globals.Get("mission")
		t.Logf("%s (mission %v): %s | heard %v", filepath.Base(save), m, strings.Join(got, " "), heard)
		seen := map[string]bool{}
		for _, id := range got {
			if seen[id] {
				t.Errorf("%s: %s announced twice", save, id)
			}
			seen[id] = true
			all[id] = true
		}
		if strings.HasSuffix(save, "stair1c1 2026-09-12-20-51-35.ti") && strings.Join(heard, " ") == "" {
			t.Errorf("%s: the dialogue in this save never reached the achievement tracker", save)
		}
		if m.Num() >= 3 && !seen["mission_three"] {
			t.Errorf("%s: a mission %v save should unlock mission_three", save, m)
		}
		if off, _, _ := playAchievementsHeard(t, files, save, 100, nil, true); len(off) != 0 {
			t.Errorf("%s: achievements were announced while turned off: %v", save, off)
		}
		if again, _ := playAchievements(t, files, save, 100, got); len(again) != 0 {
			t.Errorf("%s: unlocked achievements were announced again: %v", save, again)
		}
	}
	for _, id := range []string{"blitz_bomb", "opening_credits", "first_steps", "got_painting"} {
		if !all[id] {
			t.Errorf("no save unlocked %s", id)
		}
	}
}
