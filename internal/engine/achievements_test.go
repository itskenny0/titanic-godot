package engine

import (
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/script"
)

func testRules(t *testing.T, list ...Achievement) []achRule {
	t.Helper()
	var rules []achRule
	for _, a := range list {
		r, err := parseAchRule(a)
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, r)
	}
	return rules
}

func TestAchievementCatalogParses(t *testing.T) {
	list, rules, err := loadAchievements()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(rules) {
		t.Fatalf("%d achievements, %d rules", len(list), len(rules))
	}
	for _, a := range list {
		if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Description) == "" || a.Category == "" {
			t.Errorf("%s lacks a title, description or category", a.ID)
		}
	}
}

func TestAchievementTriggerSyntax(t *testing.T) {
	for _, bad := range []string{"", "room:gym", "global:mission", "prop:painting>=1", "count:*=3", "movie:a.mov & speak:x", "set:"} {
		if _, err := parseAchRule(Achievement{ID: "bad", Trigger: bad}); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	r, err := parseAchRule(Achievement{ID: "ok", Trigger: "movie:Movies/BERG.MOV & global:mission>=4 & prop:painting!=frank"})
	if err != nil || r.event == nil || r.event.name != "berg.mov" || len(r.state) != 2 {
		t.Fatal("event with state conditions", r, err)
	}
}

func TestAchievementCompare(t *testing.T) {
	cases := []struct {
		v    script.Value
		op   string
		want string
		ok   bool
	}{
		{script.Num(4), "=", "4", true}, {script.Num(4), ">=", "3", true}, {script.Num(2), ">=", "3", false},
		{script.Str("good"), "=", "GOOD", true}, {script.Str("good"), "!=", "good", false},
		{script.Str("startdisk2"), ">=", "4", false}, {script.Str("by sasha"), "=", "by sasha", true},
		{script.Num(-1), "<", "0", true}, {script.Bool(true), "=", "true", true}, {script.Bool(false), "=", "TRUE", false}, {script.Str(""), ">=", "0", false},
	}
	for _, c := range cases {
		if achCompare(c.v, c.op, c.want) != c.ok {
			t.Errorf("%v %s %s should be %v", c.v, c.op, c.want, c.ok)
		}
	}
}

func TestAchievementTracker(t *testing.T) {
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
	rules := testRules(t,
		Achievement{ID: "chapter2", Category: "story", Trigger: "global:mission>=2"},
		Achievement{ID: "iceberg", Category: "story", Trigger: "movie:berg.mov & global:mission=4"},
		Achievement{ID: "line", Category: "character", Trigger: "speak:penny1.009"},
		Achievement{ID: "two_story", Category: "meta", Trigger: "count:story>=2"},
		Achievement{ID: "three_any", Category: "meta", Trigger: "count:*>=3"},
		Achievement{ID: "tour", Category: "secret", Trigger: "global:tour=1 & global:mission=-1"},
	)
	var got []string
	tracker := newAchievementTracker(rules, []string{"line", "unknown"}, func(a Achievement) { got = append(got, a.ID) })
	s.Interp.Globals.Set("mission", script.Num(1))
	tracker.Poll(s)
	tracker.Event(s, "movie", "berg.mov")
	if len(got) != 0 {
		t.Fatal("nothing should unlock yet", got)
	}
	tracker.Event(s, "speak", "PENNY1.009")
	if len(got) != 0 {
		t.Fatal("an achievement unlocked earlier must not be announced again", got)
	}
	s.Interp.Globals.Set("mission", script.Num(4))
	tracker.Poll(s)
	if strings.Join(got, ",") != "chapter2" {
		t.Fatal("state achievement", got)
	}
	tracker.Event(s, "movie", `C:\Movies\Berg.mov`)
	if strings.Join(got, ",") != "chapter2,iceberg,two_story,three_any" {
		t.Fatal("event and chained milestones", got)
	}
	tracker.Event(s, "movie", "berg.mov")
	tracker.Poll(s)
	if len(got) != 4 {
		t.Fatal("each achievement unlocks once", got)
	}
	s.Interp.Globals.Set("tour", script.Num(1))
	s.Interp.Globals.Set("mission", script.Num(-1))
	tracker2 := newAchievementTracker(rules, nil, func(a Achievement) { got = append(got, a.ID) })
	got = nil
	tracker2.Poll(s)
	tracker2.Event(s, "speak", "penny1.009")
	if strings.Join(got, ",") != "tour" {
		t.Fatal("the guided tour only unlocks tour achievements", got)
	}
	got = nil
	off := newAchievementTracker(rules, nil, func(a Achievement) { got = append(got, a.ID) })
	off.Off = true
	off.Poll(s)
	off.Event(s, "speak", "penny1.009")
	if len(got) != 0 {
		t.Fatal("a tracker turned off announced", got)
	}
	off.Off = false
	off.Poll(s)
	if strings.Join(got, ",") != "tour" {
		t.Fatal("turning achievements back on catches up", got)
	}
	var nilTracker *AchievementTracker
	nilTracker.Poll(s)
	nilTracker.Event(s, "movie", "x.mov")
}
