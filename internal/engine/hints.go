package engine

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Hint is one entry of the "What should I do next?" table. The first entry
// whose conditions all hold is shown, so the table runs from the most
// specific (a step, in a particular room) to the general.
type Hint struct {
	ID   string `json:"id"`
	When string `json:"when"`
	Text string `json:"hint"`
}

//go:embed hints.json
var hintData []byte

var hintCatalog struct {
	once  sync.Once
	list  []Hint
	conds [][]achCond
	err   error
}

func loadHints() ([]Hint, [][]achCond, error) {
	c := &hintCatalog
	c.once.Do(func() {
		if c.err = json.Unmarshal(hintData, &c.list); c.err != nil {
			return
		}
		for _, h := range c.list {
			var conds []achCond
			for _, part := range strings.Split(h.When, " & ") {
				if strings.TrimSpace(part) == "" {
					continue
				}
				cond, err := parseAchCond(part)
				if err != nil {
					c.err = fmt.Errorf("hint %s: %w", h.ID, err)
					return
				}
				if achievementEvents[cond.kind] || cond.kind == "count" {
					c.err = fmt.Errorf("hint %s: %s is not a state", h.ID, part)
					return
				}
				conds = append(conds, cond)
			}
			c.conds = append(c.conds, conds)
		}
	})
	return c.list, c.conds, c.err
}

// hintHolds is an achievement state check, except that being in a room
// counts while a conversation or close-up hides it: the player asks for a
// hint from wherever they are.
func hintHolds(s *Session, c achCond) bool {
	if c.kind == "set" {
		return strings.EqualFold(strings.TrimSuffix(s.CurrentSetFile, ".set"), strings.TrimSuffix(c.name, ".set"))
	}
	return achStateHolds(s, c)
}

// NextHint returns the hint for the current game state.
func (p *Player) NextHint() (string, string) {
	list, conds, err := loadHints()
	if err != nil || p.Host == nil {
		return "", ""
	}
	s := p.Host.Session
next:
	for i, h := range list {
		for _, c := range conds[i] {
			if !hintHolds(s, c) {
				continue next
			}
		}
		return h.ID, h.Text
	}
	return "", ""
}

// interceptHelp turns a click on the original Help button into the
// frontend's Help menu while hints are on.
func (p *Player) interceptHelp(name string) bool {
	if !p.hints || !strings.EqualFold(name, "invenhelp") {
		return false
	}
	p.emit("help_menu", nil)
	return true
}

// originalHelp plays the original help, as the Help button did.
func (p *Player) originalHelp() error {
	s := p.Host.Session
	prop := s.Props.Get("invenhelp")
	if prop == nil {
		return nil
	}
	d := p.Host.Director
	s.Track("original help", false, func(*Task) error {
		_, err := d.RunPropMousedown(prop)
		return err
	})
	return nil
}
