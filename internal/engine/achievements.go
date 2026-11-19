package engine

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/itskenny0/titanic-godot/internal/script"
)

// Achievements observe the original game without changing it. Each one has a
// trigger made of conditions joined by " & ":
//
//	movie:NAME.mov   a movie starts            speak:ID    a dialogue line starts
//	voice:NAME       a voice clip starts       set:NAME    the current room set
//	stage:NAME.stg   the open stage            flat:STAGE.stg/FLAT  a stage picture
//	prop:NAME=OWNER  a prop's owner            actor:NAME=OWNER     an actor's owner
//	global:NAME=V    a script global (also !=, >=, <=, >, <)
//	count:CATEGORY>=N  unlocked achievements in a category ("*" for all)
//
// At most one condition may be an event (movie, speak, voice); the others are
// checked at that moment. Without an event the conditions are polled.

//go:embed achievements.json
var achievementData []byte

type Achievement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Hidden      bool   `json:"hidden,omitempty"`
	Trigger     string `json:"trigger"`
}

type achCond struct {
	kind, name, op, value string
}

type achRule struct {
	achievement Achievement
	event       *achCond
	state       []achCond
	counts      []achCond
	tour        bool
}

var achievementEvents = map[string]bool{"movie": true, "speak": true, "voice": true}
var achievementKinds = map[string]bool{"movie": true, "speak": true, "voice": true, "set": true, "stage": true, "flat": true, "prop": true, "actor": true, "global": true, "count": true}

func parseAchCond(text string) (achCond, error) {
	kind, rest, ok := strings.Cut(strings.TrimSpace(text), ":")
	kind = strings.ToLower(kind)
	if !ok || !achievementKinds[kind] || rest == "" {
		return achCond{}, fmt.Errorf("invalid condition %q", text)
	}
	c := achCond{kind: kind}
	switch kind {
	case "movie", "speak", "voice", "set", "stage", "flat":
		c.name = strings.ToLower(rest)
		if kind == "movie" {
			c.name = path.Base(c.name)
		}
		return c, nil
	}
	for _, op := range []string{"!=", ">=", "<=", "=", ">", "<"} {
		if at := strings.Index(rest, op); at > 0 {
			c.name, c.op, c.value = strings.ToLower(strings.TrimSpace(rest[:at])), op, strings.TrimSpace(rest[at+len(op):])
			break
		}
	}
	if c.op == "" {
		return achCond{}, fmt.Errorf("condition %q needs a comparison", text)
	}
	if (kind == "prop" || kind == "actor") && c.op != "=" && c.op != "!=" {
		return achCond{}, fmt.Errorf("condition %q can only compare with = or !=", text)
	}
	if kind == "count" {
		if _, err := strconv.Atoi(c.value); err != nil || c.op != ">=" {
			return achCond{}, fmt.Errorf("condition %q must be count:CATEGORY>=N", text)
		}
	}
	return c, nil
}

func parseAchRule(a Achievement) (achRule, error) {
	r := achRule{achievement: a}
	for _, part := range strings.Split(a.Trigger, " & ") {
		c, err := parseAchCond(part)
		if err != nil {
			return r, fmt.Errorf("%s: %w", a.ID, err)
		}
		switch {
		case achievementEvents[c.kind]:
			if r.event != nil {
				return r, fmt.Errorf("%s: more than one event condition", a.ID)
			}
			r.event = &c
		case c.kind == "count":
			r.counts = append(r.counts, c)
		default:
			if c.kind == "global" && c.name == "tour" {
				r.tour = true
			}
			r.state = append(r.state, c)
		}
	}
	if r.event == nil && len(r.state) == 0 && len(r.counts) == 0 {
		return r, fmt.Errorf("%s: empty trigger", a.ID)
	}
	return r, nil
}

var achievementCatalog struct {
	once  sync.Once
	list  []Achievement
	rules []achRule
	err   error
}

func loadAchievements() ([]Achievement, []achRule, error) {
	c := &achievementCatalog
	c.once.Do(func() {
		if c.err = json.Unmarshal(achievementData, &c.list); c.err != nil {
			return
		}
		seen := map[string]bool{}
		for _, a := range c.list {
			if a.ID == "" || seen[a.ID] {
				c.err = fmt.Errorf("duplicate or empty achievement id %q", a.ID)
				return
			}
			seen[a.ID] = true
			r, err := parseAchRule(a)
			if err != nil {
				c.err = err
				return
			}
			c.rules = append(c.rules, r)
		}
	})
	return c.list, c.rules, c.err
}

// AchievementCatalog lists every achievement in display order.
func AchievementCatalog() []Achievement {
	list, _, _ := loadAchievements()
	return list
}

type AchievementTracker struct {
	rules    []achRule
	unlocked map[string]bool
	byCat    map[string]int
	total    int
	Unlock   func(Achievement)
	// Off stops tracking for players who chose to play without achievements.
	Off bool
}

func NewAchievementTracker(unlocked []string, unlock func(Achievement)) (*AchievementTracker, error) {
	_, rules, err := loadAchievements()
	if err != nil {
		return nil, err
	}
	return newAchievementTracker(rules, unlocked, unlock), nil
}

func newAchievementTracker(rules []achRule, unlocked []string, unlock func(Achievement)) *AchievementTracker {
	t := &AchievementTracker{rules: rules, unlocked: map[string]bool{}, byCat: map[string]int{}, Unlock: unlock}
	known := map[string]string{}
	for _, r := range rules {
		known[r.achievement.ID] = r.achievement.Category
	}
	for _, id := range unlocked {
		if cat, ok := known[id]; ok && !t.unlocked[id] {
			t.unlocked[id] = true
			t.byCat[cat]++
			t.total++
		}
	}
	return t
}

func (t *AchievementTracker) Unlocked(id string) bool { return t.unlocked[id] }

// Event reports a movie, dialogue line or voice clip as it starts.
func (t *AchievementTracker) Event(s *Session, kind, name string) {
	if t == nil || s == nil || t.Off {
		return
	}
	if kind == "movie" {
		name = path.Base(strings.ToLower(strings.ReplaceAll(name, "\\", "/")))
	}
	name = strings.ToLower(name)
	tour := achTouring(s)
	for i := range t.rules {
		r := &t.rules[i]
		if r.event == nil || t.unlocked[r.achievement.ID] || r.event.kind != kind || r.event.name != name || tour && !r.tour {
			continue
		}
		if t.stateHolds(s, r) {
			t.grant(r.achievement)
		}
	}
	t.settleCounts()
}

// Poll checks the conditions that describe a state rather than a moment.
func (t *AchievementTracker) Poll(s *Session) {
	if t == nil || s == nil || t.Off {
		return
	}
	tour := achTouring(s)
	for i := range t.rules {
		r := &t.rules[i]
		if r.event != nil || len(r.state) == 0 || t.unlocked[r.achievement.ID] || tour && !r.tour {
			continue
		}
		if t.stateHolds(s, r) {
			t.grant(r.achievement)
		}
	}
	t.settleCounts()
}

func (t *AchievementTracker) grant(a Achievement) {
	t.unlocked[a.ID] = true
	t.byCat[a.Category]++
	t.total++
	if t.Unlock != nil {
		t.Unlock(a)
	}
}

// Milestones unlock from other achievements, possibly in a chain.
func (t *AchievementTracker) settleCounts() {
	for changed := true; changed; {
		changed = false
		for i := range t.rules {
			r := &t.rules[i]
			if len(r.counts) == 0 || r.event != nil || len(r.state) > 0 || t.unlocked[r.achievement.ID] {
				continue
			}
			if t.countsHold(r) {
				t.grant(r.achievement)
				changed = true
			}
		}
	}
}

func (t *AchievementTracker) countsHold(r *achRule) bool {
	for _, c := range r.counts {
		n, _ := strconv.Atoi(c.value)
		have := t.byCat[c.name]
		if c.name == "*" {
			have = t.total
		}
		if have < n {
			return false
		}
	}
	return true
}

func (t *AchievementTracker) stateHolds(s *Session, r *achRule) bool {
	for _, c := range r.state {
		if !achStateHolds(s, c) {
			return false
		}
	}
	return t.countsHold(r)
}

func achTouring(s *Session) bool {
	v, ok := s.Interp.Globals.Get("tour")
	return ok && v.Truthy()
}

func achStageName(name string) string {
	return path.Base(strings.ToLower(strings.ReplaceAll(name, "\\", "/")))
}

func achStateHolds(s *Session, c achCond) bool {
	switch c.kind {
	case "set":
		return strings.EqualFold(strings.TrimSuffix(s.CurrentSetFile, ".set"), strings.TrimSuffix(c.name, ".set")) && s.ViewShowing()
	case "stage":
		return s.StageOpen() && achStageName(s.StageCtrl.Name) == c.name
	case "flat":
		stage, flat, _ := strings.Cut(c.name, "/")
		return s.StageOpen() && achStageName(s.StageCtrl.Name) == stage && strings.EqualFold(s.StageCtrl.CurrentFlat, flat)
	case "prop":
		if p := s.Props.Get(c.name); p != nil {
			return achCompare(p.Owner, c.op, c.value)
		}
		return c.op == "!="
	case "actor":
		if a := s.Actors.Get(c.name); a != nil {
			return achCompare(a.Owner, c.op, c.value)
		}
		return c.op == "!="
	case "global":
		v, ok := s.Interp.Globals.Get(c.name)
		if !ok {
			return c.op == "!="
		}
		return achCompare(v, c.op, c.value)
	}
	return false
}

func achCompare(v script.Value, op, want string) bool {
	switch strings.ToLower(want) {
	case "true":
		want = "1"
	case "false":
		want = "0"
	}
	n, numeric := strconv.ParseFloat(want, 64)
	expected := script.Str(want)
	if numeric == nil {
		expected = script.Num(n)
	}
	switch op {
	case "=":
		return script.Equal(v, expected)
	case "!=":
		return !script.Equal(v, expected)
	}
	if numeric != nil || v.IsString && strings.TrimSpace(v.Text) == "" {
		return false
	}
	have := v.Num()
	switch op {
	case ">=":
		return have >= n
	case "<=":
		return have <= n
	case ">":
		return have > n
	case "<":
		return have < n
	}
	return false
}
