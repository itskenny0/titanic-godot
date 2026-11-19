package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

// Every name a trigger mentions must exist in the owned game files, so a
// misspelt movie or dialogue line cannot leave an achievement unreachable.
func TestAchievementTriggersMatchGameData(t *testing.T) {
	root := os.Getenv("TAOOT_GAME_DATA")
	if root == "" {
		t.Skip("set TAOOT_GAME_DATA to check achievement triggers against the game files")
	}
	known := map[string]map[string]bool{}
	add := func(kind, name string) {
		if known[kind] == nil {
			known[kind] = map[string]bool{}
		}
		known[kind][strings.ToLower(name)] = true
	}
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		name := strings.ToLower(e.Name())
		ext := filepath.Ext(name)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		switch ext {
		case ".mov":
			add("movie", name)
		case ".set":
			add("set", strings.TrimSuffix(name, ext))
		case ".stg":
			add("stage", name)
			if stage, err := df.ReadStage(data); err == nil {
				for _, f := range stage.Flats {
					add("flat", name+"/"+f.Name)
				}
			}
		case ".pup":
			if pup, err := df.ReadPuppet(data); err == nil {
				for id := range pup.Dialogue {
					add("speak", id)
				}
			}
		case ".sfx", ".trk", ".11k":
			if bank, err := df.ReadAudioBank(data); err == nil {
				for id := range bank.Singles {
					add("voice", id)
				}
			}
		}
		if ext == ".set" || ext == ".stg" || ext == ".shp" || ext == ".cst" || ext == ".pup" || name == "bootfile" {
			file, err := df.ReadFile(data)
			if err != nil {
				return nil
			}
			for loc := 1; loc < len(file.Containers); loc++ {
				tokens, err := df.DecodeScript(file.Data(loc))
				if err != nil {
					continue
				}
				for _, tk := range tokens {
					switch tk.Kind {
					case df.TokenString:
						add("literal", tk.Text)
					case df.TokenVariable:
						add("variable", tk.Text)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"movie", "set", "stage", "flat", "speak", "voice", "literal", "variable"} {
		if len(known[kind]) == 0 {
			t.Fatalf("found no %s names in %s", kind, root)
		}
	}
	_, rules, err := loadAchievements()
	if err != nil {
		t.Fatal(err)
	}
	categories := map[string]bool{"*": true}
	for _, r := range rules {
		categories[r.achievement.Category] = true
	}
	valueKnown := func(v string) bool {
		_, numeric := strconv.ParseFloat(v, 64)
		return numeric == nil || v == "true" || v == "false" || known["literal"][strings.ToLower(v)]
	}
	for _, r := range rules {
		conds := append([]achCond{}, r.state...)
		conds = append(conds, r.counts...)
		if r.event != nil {
			conds = append(conds, *r.event)
		}
		for _, c := range conds {
			ok := true
			switch c.kind {
			case "movie", "set", "stage", "flat", "speak", "voice":
				ok = known[c.kind][c.name]
			case "prop", "actor":
				ok = known["literal"][c.name] && valueKnown(c.value)
			case "global":
				ok = known["variable"][c.name] && valueKnown(c.value)
			case "count":
				ok = categories[c.name]
			}
			if !ok {
				t.Errorf("%s: %s:%s%s%s is not in the game files", r.achievement.ID, c.kind, c.name, c.op, c.value)
			}
		}
	}
}
