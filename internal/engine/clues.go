package engine

import (
	_ "embed"
	"encoding/json"
	"slices"
	"strings"
	"sync"
)

// Clues are short notes for the player's scratchpad. Each is learned from
// any of the dialogue lines that mention it, such as a cabin number, and
// never changes the game.
type Clue struct {
	ID string `json:"id"`
	// Kind lets players choose how much help they get: "places" (cabins and
	// directions), "story" (who is who, what happened), "errands" (what
	// people want of Frank) or "puzzles" (how to get past an obstacle).
	Kind  string   `json:"kind"`
	Text  string   `json:"text"`
	Lines []string `json:"lines"`
}

// ClueKinds lists the kinds from least to most revealing.
var ClueKinds = []string{"places", "story", "errands", "puzzles"}

//go:embed clues.json
var clueData []byte

var clueCatalog struct {
	once   sync.Once
	list   []Clue
	byLine map[string][]*Clue
	err    error
}

func loadClues() ([]Clue, map[string][]*Clue, error) {
	c := &clueCatalog
	c.once.Do(func() {
		if c.err = json.Unmarshal(clueData, &c.list); c.err != nil {
			return
		}
		c.byLine = map[string][]*Clue{}
		for i := range c.list {
			for _, line := range c.list[i].Lines {
				key := strings.ToLower(line)
				// One line may carry several facts, each its own note.
				c.byLine[key] = append(c.byLine[key], &c.list[i])
			}
		}
	})
	return c.list, c.byLine, c.err
}

func (p *Player) noteClue(s *Session, line string) {
	if !p.clues || achTouring(s) {
		return
	}
	_, byLine, err := loadClues()
	if err != nil {
		return
	}
	for _, c := range byLine[strings.ToLower(line)] {
		if !slices.Contains(p.pendingClues, c) {
			p.pendingClues = append(p.pendingClues, c)
		}
	}
}

// serviceClues reports learned clues once the conversation is over, so a
// note never gives away a line before the character has said it.
func (p *Player) serviceClues() {
	if len(p.pendingClues) == 0 {
		return
	}
	if !p.clues {
		p.pendingClues = nil
		return
	}
	if d := p.Host.Director; d.Conversing() || d.Speaking() {
		return
	}
	for _, c := range p.pendingClues {
		p.emit("clue", map[string]any{"id": c.ID, "kind": c.Kind, "text": c.Text})
	}
	p.pendingClues = nil
}
