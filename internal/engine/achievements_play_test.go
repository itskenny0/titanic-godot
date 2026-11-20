package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
)

// achRun boots a real save and plays it, acknowledging sounds after their
// real length, while it records announced achievements and spoken lines.
type achRun struct {
	t     *testing.T
	p     *Player
	ends  map[uint64]float64
	got   []string
	seen  map[string]bool
	heard []string
	quit  bool
	// onEvent sees every other event the player emits.
	onEvent func(map[string]any)
}

func startAchRun(t *testing.T, save string) *achRun {
	t.Helper()
	p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{Index: ownedFiles(t).Index, Save: save}); err != nil {
		t.Fatal(err)
	}
	r := &achRun{t: t, p: p, ends: map[uint64]float64{}, seen: map[string]bool{}}
	s := p.Host.Session
	speak, voice := s.OnSpeak, s.OnVoice
	s.OnSpeak = func(id string) { r.heard = append(r.heard, id); speak(id) }
	s.OnVoice = func(id string) { r.heard = append(r.heard, "voice:"+id); voice(id) }
	r.play(300, nil)
	return r
}

// play ticks until done reports true or the tick budget runs out.
func (r *achRun) play(ticks int, done func() bool) {
	r.t.Helper()
	p := r.p
	for i := 0; i < ticks && !r.quit; i++ {
		if done != nil && done() {
			return
		}
		for id, end := range r.ends {
			if p.now >= end {
				p.Command(PlayerCommand{Action: "audio_done", ID: id})
				delete(r.ends, id)
			}
		}
		if err := p.Tick(50); err != nil {
			r.t.Fatal(err)
		}
		for _, e := range p.Events() {
			switch event := e.(type) {
			case AudioEvent:
				if event.Type == "audio_play" {
					p.TakeAudio(event.ID)
					if !event.Loop {
						r.ends[event.ID] = p.now + float64(event.Samples)*1000/float64(event.Rate)
					}
				} else if event.Type == "audio_stop" {
					delete(r.ends, event.ID)
				}
			case map[string]any:
				switch event["type"] {
				case "achievement":
					id := event["id"].(string)
					if r.seen[id] {
						r.t.Errorf("%s announced twice", id)
					}
					r.seen[id] = true
					r.got = append(r.got, id)
				case "quit":
					r.quit = true
				case "error":
					r.t.Fatal(event["text"])
				}
				if r.onEvent != nil {
					r.onEvent(event)
				}
			}
		}
	}
}

func (r *achRun) global(name string) script.Value {
	v, _ := r.p.Host.Session.Interp.Globals.Get(name)
	return v
}

// run starts one of the game's own global handlers, as its timers would.
func (r *achRun) run(handler string) {
	s := r.p.Host.Session
	s.Track("test:"+handler, false, func(*Task) error {
		_, err := s.RunGlobal(handler, nil)
		return err
	})
}

func (r *achRun) owners(props, actors map[string]string) {
	s := r.p.Host.Session
	for name, owner := range props {
		p := s.Props.Get(name)
		if p == nil {
			r.t.Fatalf("no prop %s", name)
		}
		p.Owner = script.Str(owner)
	}
	for name, owner := range actors {
		a := s.Actors.Get(name)
		if a == nil {
			r.t.Fatalf("no actor %s", name)
		}
		a.Owner = script.Str(owner)
	}
}

func (r *achRun) expect(t *testing.T, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if !r.seen[id] {
			got := append([]string{}, r.got...)
			sort.Strings(got)
			t.Errorf("%s was not unlocked; got %s", id, strings.Join(got, " "))
		}
	}
}

func sinkingSave(t *testing.T) string {
	t.Helper()
	saves, _ := filepath.Glob(filepath.Join("..", "..", "originalgame", "deckbd2 *.ti"))
	if len(saves) == 0 {
		t.Skip("no sinking (deckbd2) save in originalgame/")
	}
	sort.Strings(saves)
	return saves[0]
}

// The endings play from the game's own endgame branch, with the items in
// the hands that choose each future, and the narrator walks its real lists.
func TestAchievementEndingsPlay(t *testing.T) {
	save := sinkingSave(t)
	cases := []struct {
		name          string
		props, actors map[string]string
		want          []string
		// playMore clicks this point on the "play more?" movie after a bad
		// ending.
		playMore []float64
	}{
		{"good", map[string]string{"rubaiyat": "frank", "realneck": "frank", "painting": "frank", "notebook": "frank"}, map[string]string{"ga": "rescued", "shay": "rescued"},
			[]string{"leave_ship", "leave_with_rescues", "leave_with_all_three", "debris_field", "history_judged", "ww1_averted", "hitler_painter", "revolution_aborted", "good_ending"}, nil},
		{"unchanged", map[string]string{"rubaiyat": "vlad", "realneck": "vlad", "painting": "zeit", "notebook": "vlad"}, nil,
			[]string{"ww1_vlad_both", "ww1_sarajevo", "painting_to_germans", "revolution_continues", "ending_unchanged", "second_chance"}, []float64{266, 254}},
		{"berlin", map[string]string{"rubaiyat": "frank", "realneck": "vlad", "painting": "frank", "notebook": "frank"}, nil,
			[]string{"ww1_sarajevo", "ending_berlin_politburo", "ending_soviet_europe"}, nil},
		{"nazi", map[string]string{"rubaiyat": "frank", "realneck": "frank", "painting": "hack", "notebook": "frank"}, nil,
			[]string{"painting_sank", "ending_nazi_europe"}, nil},
		{"nuclear", map[string]string{"rubaiyat": "frank", "realneck": "frank", "painting": "zeit", "notebook": "vlad"}, nil,
			[]string{"ending_nuclear"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := startAchRun(t, save)
			r.owners(c.props, c.actors)
			r.p.Host.Session.Interp.Globals.Set("clock", script.Str("endgame"))
			r.run("advanceday")
			movies := r.p.Host.Director.Movies
			r.play(40000, func() bool {
				if m := movies.active; c.playMore != nil && m != nil && m.name == "playmore.mov" && m.hasRegions {
					if err := movies.Click(c.playMore[0], c.playMore[1]); err != nil {
						t.Fatal(err)
					}
				}
				for _, id := range c.want {
					if !r.seen[id] {
						return false
					}
				}
				return true
			})
			r.expect(t, c.want...)
			t.Logf("%s: %s", c.name, strings.Join(r.got, " "))
		})
	}
}

// The sinking runs through the game's own advancephase timer from the
// first stage to drowning, with Georgia and the baby saved on the way.
func TestAchievementSinkingPlays(t *testing.T) {
	r := startAchRun(t, sinkingSave(t))
	r.owners(map[string]string{"antidote": "ga", "baby": "shay"}, map[string]string{"ga": "none", "shay": "none"})
	g := r.p.Host.Session.Interp.Globals
	g.Set("phase", script.Num(0))
	for i, clock := range []float64{1315, 1330, 1345, 1355, 1400, 1405} {
		g.Set("clock", script.Num(clock))
		r.run("advancephase")
		r.play(6000, func() bool { return r.global("phase").Num() > float64(i) && !r.p.Host.Session.ScriptBusy() })
		if got := r.global("phase").Num(); got != float64(i+1) {
			t.Fatalf("clock %v: phase is %v, want %d", clock, got, i+1)
		}
	}
	r.play(2000, func() bool { return r.seen["death_sinking"] })
	r.expect(t, "sinking_boat_deck", "sink_stage1", "sink_stage2", "sink_stage3", "sink_stage4", "sink_stage5", "georgia_rescued", "shailagh_rescued", "death_sinking")
	t.Logf("sinking: %s", strings.Join(r.got, " "))
}

// Blackjack against Buick Riviera is played at the real table: click it,
// answer "Yes." and let the shuffled deck deal. Seeds are tried until the
// deal is a natural blackjack, whose congratulation must find the player
// already recorded as the winner.
func TestAchievementBlackjackPlays(t *testing.T) {
	save := filepath.Join("..", "..", "originalgame", "deckbd 2026-09-12-21-27-43.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no mission 1 deckbd save in originalgame/")
	}
	for seed := uint32(1); seed <= 400; seed++ {
		r := startAchRun(t, save)
		s := r.p.Host.Session
		s.SeedRandom(seed)
		s.Track("click table", false, func(*Task) error {
			_, err := s.SendEvent("sendtoprop", "blkjacktable", "mousedown", nil, "", nil)
			return err
		})
		answered := false
		dealt := 0
		r.play(4000, func() bool {
			if p := s.PuppetCtrl.Puppet; !answered && p != nil && p.EventWaiter != nil {
				for i, b := range p.Bevels {
					if b.Text == "Yes." {
						s.PuppetCtrl.Choose(nil, i, false)
						answered = true
					}
				}
			}
			if s.StageCtrl.Name == "blkjack.stg" && r.global("playerphase").Num() == 1 && r.global("winner").Text == "" {
				dealt++
			}
			// No natural: the hand now waits for the player to hit or stand.
			return r.seen["natural_blackjack"] || dealt > 100 || r.global("winner").Text == "dealer" || r.global("winner").Text == "draw"
		})
		if !answered {
			t.Fatalf("seed %d: Buick never offered a game; heard %v", seed, r.heard)
		}
		if r.seen["natural_blackjack"] {
			r.expect(t, "beat_riviera", "natural_blackjack")
			t.Logf("seed %d: %s", seed, strings.Join(r.got, " "))
			return
		}
	}
	t.Fatal("no seed dealt a natural blackjack")
}

// achSources maps dialogue lines to the puppet file that holds them and
// voice clips to their sound bank, so a replay can open the right one first.
func achSources(t *testing.T, root string) (lines, clips map[string]string) {
	t.Helper()
	lines, clips = map[string]string{}, map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.ToLower(d.Name())
		switch filepath.Ext(name) {
		case ".pup":
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if pup, err := df.ReadPuppet(data); err == nil {
				for id := range pup.Dialogue {
					lines[strings.ToLower(id)] = name
				}
			}
		case ".sfx", ".trk", ".11k":
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if bank, err := df.ReadAudioBank(data); err == nil {
				for id := range bank.Singles {
					if _, have := clips[strings.ToLower(id)]; !have {
						clips[strings.ToLower(id)] = name
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return lines, clips
}

func achStrs(v ...string) []script.Value {
	out := make([]script.Value, len(v))
	for i, s := range v {
		out[i] = script.Str(s)
	}
	return out
}

// Every achievement is unlocked once by replaying, in a booted game, the
// call the original scripts use to produce its trigger: the line is spoken
// in its own puppet, the movie and clip are played, rooms and stages are
// entered through the game's own changeset and transtoflat, items move
// through the inventory's addinven and giveinven. This catches what reading
// the scripts cannot: a movie with no frames, a line its puppet cannot say,
// a clip no bank holds, a room that never shows.
func TestAchievementTriggersReplay(t *testing.T) {
	root := os.Getenv("TAOOT_GAME_DATA")
	save := filepath.Join("..", "..", "originalgame", "deckbd 2026-09-12-21-27-43.ti")
	if root == "" {
		t.Skip("set TAOOT_GAME_DATA to replay achievement triggers")
	}
	if _, err := os.Stat(save); err != nil {
		t.Skip("no mission 1 deckbd save in originalgame/")
	}
	e := scanAchievementScripts(t, root)
	lines, clips := achSources(t, root)
	list, rules, err := loadAchievements()
	if err != nil {
		t.Fatal(err)
	}
	var plain []string
	for _, a := range list {
		if a.Category != "meta" {
			plain = append(plain, a.ID)
		}
	}
	for _, r := range rules {
		id := r.achievement.ID
		// Entering narend.stg runs the whole narration, which picks its own
		// pictures from the items; TestAchievementEndingsPlay plays every
		// branch of it.
		if len(r.counts) > 0 || strings.HasPrefix(r.achievement.Trigger, "flat:narend.stg/") {
			continue
		}
		t.Run(id, func(t *testing.T) {
			run := startAchRun(t, save)
			s := run.p.Host.Session
			// Start from nothing unlocked so this achievement can announce.
			run.p.achievements = newAchievementTracker(rules, nil, run.p.achievements.Unlock)
			run.got, run.seen = nil, map[string]bool{}
			builtin := func(name string, args ...string) {
				b := s.Interp.Builtins[name]
				if b == nil {
					t.Fatalf("no builtin %s", name)
				}
				if _, err := b(s.Interp, achStrs(args...), nil, nil); err != nil {
					t.Fatalf("%s%v: %v", name, args, err)
				}
			}
			global := func(name string, args ...string) {
				if _, err := s.RunGlobal(name, achStrs(args...)); err != nil {
					t.Fatalf("%s%v: %v", name, args, err)
				}
			}
			shop := func(handler string, args ...string) {
				if _, err := s.SendEvent("sendtoshop", "inven.shp", handler, achStrs(args...), "", nil); err != nil {
					t.Fatalf("%s%v: %v", handler, args, err)
				}
			}
			conds := append([]achCond{}, r.state...)
			if r.event != nil {
				conds = append(conds, *r.event)
			}
			s.Track("replay "+id, false, func(*Task) error {
				for _, c := range conds {
					switch c.kind {
					case "global":
						v := c.value
						switch {
						case c.op == ">=" || c.op == ">":
							n, _ := strconv.ParseFloat(v, 64)
							if c.op == ">" {
								n++
							}
							s.Interp.Globals.Set(c.name, script.Num(n))
						case v == "true":
							s.Interp.Globals.Set(c.name, script.Bool(true))
						default:
							if n, err := strconv.ParseFloat(v, 64); err == nil {
								s.Interp.Globals.Set(c.name, script.Num(n))
							} else {
								s.Interp.Globals.Set(c.name, script.Str(v))
							}
						}
					case "prop":
						// The darkroom's photographs exist only while its
						// stage, and so photo.shp, is open.
						if c.name == "pic1" {
							global("transtoflat", "photo.stg")
						}
						switch {
						case c.value == "frank":
							shop("addinven", c.name)
						case e.props[c.name][c.value] && s.Props.Get(c.name) != nil:
							builtin("propowner", c.name, c.value)
						default:
							t.Fatalf("no way to give %s to %s", c.name, c.value)
						}
					case "actor":
						builtin("actorowner", c.name, c.value)
					case "set":
						call, ok := e.roomCalls[c.name]
						if !ok {
							t.Fatalf("no script enters %s with a literal scene and view", c.name)
						}
						global("changeset", call[:]...)
					case "stage":
						global("transtoflat", c.name)
					case "flat":
						stage, flat, _ := strings.Cut(c.name, "/")
						global("transtoflat", stage)
						builtin("gotoflat", flat)
					case "movie":
						builtin("playmovie", c.name)
					case "voice":
						bank, ok := clips[c.name]
						if !ok {
							t.Fatalf("no sound bank holds %s", c.name)
						}
						builtin("opentrackfile", bank)
						builtin("voicesound", c.name)
					case "speak":
						pup, ok := lines[c.name]
						if !ok {
							t.Fatalf("no puppet holds line %s", c.name)
						}
						builtin("openpuppetfile", pup)
						builtin("puppetspeak", c.name)
					}
				}
				return nil
			})
			run.play(1200, func() bool { return run.seen[id] })
			if !run.seen[id] {
				t.Errorf("replaying %q did not unlock %s; got %v", r.achievement.Trigger, id, run.got)
			}
		})
	}
	// Milestones follow from the others: with every other achievement
	// earned, all of them settle at the next check.
	t.Run("milestones", func(t *testing.T) {
		run := startAchRun(t, save)
		run.p.achievements = newAchievementTracker(rules, plain, run.p.achievements.Unlock)
		run.got, run.seen = nil, map[string]bool{}
		run.play(60, nil)
		for _, a := range list {
			if a.Category == "meta" && !run.seen[a.ID] {
				t.Errorf("%s did not unlock with every other achievement earned; got %v", a.ID, run.got)
			}
		}
	})
}
