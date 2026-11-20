package engine

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

// Clues are short notes for the player's scratchpad. Each is learned from
// any of the dialogue lines that mention it, such as a cabin number, and
// never changes the game.
type Clue struct {
	ID    string   `json:"id"`
	Text  string   `json:"text"`
	Lines []string `json:"lines"`
}

//go:embed clues.json
var clueData []byte

var clueCatalog struct {
	once   sync.Once
	list   []Clue
	byLine map[string]*Clue
	err    error
}

func loadClues() ([]Clue, map[string]*Clue, error) {
	c := &clueCatalog
	c.once.Do(func() {
		if c.err = json.Unmarshal(clueData, &c.list); c.err != nil {
			return
		}
		c.byLine = map[string]*Clue{}
		for i := range c.list {
			for _, line := range c.list[i].Lines {
				c.byLine[strings.ToLower(line)] = &c.list[i]
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
	if c := byLine[strings.ToLower(line)]; c != nil {
		p.emit("clue", map[string]any{"id": c.ID, "text": c.Text})
	}
}
