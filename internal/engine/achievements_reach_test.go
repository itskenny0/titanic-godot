package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

// achScriptEvidence records what the original scripts can actually do, as
// opposed to what merely exists in the game files: which lines they speak,
// which movies and voice clips they play, which owners they give props and
// actors, and which values they assign to globals.
type achScriptEvidence struct {
	speak, voice, movie map[string]bool
	// prefixes are literals joined to a computed part, as in
	// playmovie("sink" @ numtostring(phase) @ ".mov").
	// numberPrefixes are joined to numtostring(...); wordPrefixes to
	// findword(...) over the comma lists of the file they appear in.
	numberPrefixes map[string]bool
	wordPrefixes   map[string]map[string]bool
	// rooms and stages are literal destinations of the scripts' set and
	// stage changes; flats are literal pictures a stage goes to.
	rooms, stages, flats map[string]bool
	// roomCalls keeps one literal (set, scene, view) entry for each room.
	roomCalls map[string][3]string
	// listWords are the items of comma lists such as "4,07,11,11b,12",
	// which the narrator ending walks with findword to pick each picture
	// and voice clip. They are kept per script file.
	listWords               map[string]map[string]bool
	props, actors, assigned map[string]map[string]bool
	arithmetic              map[string]bool
}

func achAdd(m map[string]map[string]bool, k, v string) {
	if m[k] == nil {
		m[k] = map[string]bool{}
	}
	m[k][v] = true
}

type achToken struct {
	text    string
	literal bool
}

func achTokens(tokens []df.Token) []achToken {
	out := make([]achToken, 0, len(tokens))
	for _, tk := range tokens {
		switch tk.Kind {
		case df.TokenString:
			out = append(out, achToken{strings.ToLower(tk.Text), true})
		case df.TokenVariable, df.TokenOpcode:
			out = append(out, achToken{strings.ToLower(tk.Text), false})
		case df.TokenInteger:
			out = append(out, achToken{strconv.FormatUint(uint64(tk.Number), 10), false})
		case df.TokenBreak:
			out = append(out, achToken{"\n", false})
		}
	}
	return out
}

func (e *achScriptEvidence) scan(toks []achToken, me, file string) {
	at := func(i int, text string) bool { return i < len(toks) && !toks[i].literal && toks[i].text == text }
	lit := func(i int) (string, bool) {
		if i < len(toks) && toks[i].literal {
			return toks[i].text, true
		}
		return "", false
	}
	// name ( "X" ) = "V"  or  name ( "X" , "V" ), with me standing for the prop.
	owner := func(i int) (string, string, bool) {
		if !at(i+1, "(") {
			return "", "", false
		}
		name, ok := lit(i + 2)
		if !ok && at(i+2, "me") && me != "" {
			name, ok = me, true
		}
		if !ok {
			return "", "", false
		}
		if at(i+3, ",") {
			v, ok := lit(i + 4)
			return name, v, ok && at(i+5, ")")
		}
		if at(i+3, ")") && at(i+4, "=") {
			v, ok := lit(i + 5)
			return name, v, ok
		}
		return "", "", false
	}
	for i, tk := range toks {
		if tk.literal {
			if strings.HasSuffix(tk.text, ".mov") {
				e.movie[tk.text] = true
			}
			if at(i+1, "@") {
				if at(i+2, "numtostring") {
					e.numberPrefixes[tk.text] = true
				}
				if at(i+2, "findword") {
					achAdd(e.wordPrefixes, tk.text, file)
				}
			}
			if strings.Contains(tk.text, ",") {
				for _, w := range strings.Split(tk.text, ",") {
					achAdd(e.listWords, file, strings.TrimSpace(w))
				}
			}
			continue
		}
		switch tk.text {
		case "puppetspeak", "voicesound":
			if v, ok := lit(i + 2); ok && at(i+1, "(") {
				if tk.text == "puppetspeak" {
					e.speak[v] = true
				} else {
					e.voice[v] = true
				}
			}
		case "propowner", "actorowner", "giveinven":
			if n, v, ok := owner(i); ok {
				if tk.text == "actorowner" {
					achAdd(e.actors, n, v)
				} else {
					achAdd(e.props, n, v)
				}
			}
		case "changeset", "gotospecial", "initall", "jumpbaby":
			if v, ok := lit(i + 2); ok && at(i+1, "(") {
				room := strings.TrimSuffix(v, ".set")
				e.rooms[room] = true
				scene, ok1 := lit(i + 4)
				view, ok2 := lit(i + 6)
				if _, have := e.roomCalls[room]; !have && ok1 && ok2 && at(i+3, ",") && at(i+5, ",") {
					e.roomCalls[room] = [3]string{v, scene, view}
				}
			}
		case "transtoflat", "openstagefile":
			if v, ok := lit(i + 2); ok && at(i+1, "(") {
				e.stages[v] = true
			}
		case "gotoflat":
			if v, ok := lit(i + 2); ok && at(i+1, "(") {
				e.flats[v] = true
			}
		case "addinven":
			if v, ok := lit(i + 2); ok && at(i+1, "(") {
				achAdd(e.props, v, "frank")
			}
		}
		// A global assignment starts a statement: NAME = VALUE.
		if (i == 0 || at(i-1, "\n")) && at(i+1, "=") && i+2 < len(toks) {
			v := toks[i+2]
			if v.literal || at(i+3, "\n") || i+3 == len(toks) {
				achAdd(e.assigned, tk.text, v.text)
			} else {
				e.arithmetic[tk.text] = true
			}
		}
	}
	// A prop that sets itself up small in a room is picked up by the shared
	// stdmouse handler, which calls addinven(what).
	if me != "" {
		for i := range toks {
			if at(i, "propview") && at(i+1, "(") && at(i+2, "me") && at(i+3, ",") {
				if v, ok := lit(i + 4); ok && v == "small" {
					for j := range toks {
						if at(j, "code") && at(j+1, "setupprop") {
							achAdd(e.props, me, "frank")
						}
					}
				}
			}
		}
	}
}

func scanAchievementScripts(t *testing.T, root string) *achScriptEvidence {
	e := &achScriptEvidence{
		speak: map[string]bool{}, voice: map[string]bool{}, movie: map[string]bool{}, numberPrefixes: map[string]bool{}, wordPrefixes: map[string]map[string]bool{},
		rooms: map[string]bool{}, stages: map[string]bool{}, flats: map[string]bool{}, listWords: map[string]map[string]bool{}, roomCalls: map[string][3]string{},
		props: map[string]map[string]bool{}, actors: map[string]map[string]bool{}, assigned: map[string]map[string]bool{}, arithmetic: map[string]bool{},
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.ToLower(d.Name())
		ext := filepath.Ext(name)
		if ext != ".set" && ext != ".stg" && ext != ".shp" && ext != ".cst" && ext != ".pup" && name != "bootfile" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		file, err := df.ReadFile(data)
		if err != nil {
			return nil
		}
		props := map[int]string{}
		if ext == ".shp" {
			if shop, err := df.ReadShop(data); err == nil {
				for _, g := range shop.Groups {
					props[g.ScriptContainerLocation] = strings.ToLower(g.Name)
				}
			}
		}
		for loc := 1; loc < len(file.Containers); loc++ {
			tokens, err := df.DecodeScript(file.Data(loc))
			if err == nil {
				e.scan(achTokens(tokens), props[loc], name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

var achNumberTail = regexp.MustCompile(`^[0-9]+$`)

// computed reports whether a name can be built at run time: a literal prefix
// joined to numtostring(...), or to findword(...) over a comma list in the
// same script file, as the narrator ending does with "n.".
func (e *achScriptEvidence) computed(name string) bool {
	for prefix := range e.numberPrefixes {
		if prefix != "" && strings.HasPrefix(name, prefix) && achNumberTail.MatchString(strings.TrimSuffix(name[len(prefix):], ".mov")) {
			return true
		}
	}
	for prefix, files := range e.wordPrefixes {
		if prefix == "" || !strings.HasPrefix(name, prefix) {
			continue
		}
		for file := range files {
			if e.listWords[file][name[len(prefix):]] {
				return true
			}
		}
	}
	return false
}

// Every trigger must be something the original scripts can produce, not just
// a name that exists in the files: an item no script ever hands to the
// player would leave its achievement, and the completionist one, out of
// reach.
func TestAchievementTriggersAreProducedByScripts(t *testing.T) {
	root := os.Getenv("TAOOT_GAME_DATA")
	if root == "" {
		t.Skip("set TAOOT_GAME_DATA to check achievement triggers against the game scripts")
	}
	e := scanAchievementScripts(t, root)
	if len(e.speak) < 1000 || len(e.props["rubaiyat"]) == 0 || len(e.assigned["mission"]) == 0 {
		t.Fatalf("script scan found too little: %d lines, rubaiyat owners %v, mission values %v", len(e.speak), e.props["rubaiyat"], e.assigned["mission"])
	}
	_, rules, err := loadAchievements()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		conds := append([]achCond{}, r.state...)
		if r.event != nil {
			conds = append(conds, *r.event)
		}
		for _, c := range conds {
			ok := true
			switch c.kind {
			case "speak":
				ok = e.speak[c.name]
			case "voice":
				ok = e.voice[c.name] || e.computed(c.name)
			case "movie":
				ok = e.movie[c.name] || e.computed(c.name)
			case "prop", "actor":
				owners := e.props[c.name]
				if c.kind == "actor" {
					owners = e.actors[c.name]
				}
				ok = owners[strings.ToLower(c.value)]
				if c.op == "!=" {
					ok = len(owners) > 0
				}
			case "set":
				ok = e.rooms[c.name]
			case "stage":
				ok = e.stages[c.name]
			case "flat":
				stage, flat, _ := strings.Cut(c.name, "/")
				ok = e.stages[stage] && (e.flats[flat] || e.listWords[stage][flat])
			case "global":
				values := e.assigned[c.name]
				want := strings.ToLower(c.value)
				switch {
				case c.op == "=" && want == "true":
					ok = values["true"] || values["1"]
				case c.op == "=":
					ok = values[want]
				default:
					ok = len(values) > 0 || e.arithmetic[c.name]
				}
			}
			if !ok {
				t.Errorf("%s: no script produces %s:%s%s%s", r.achievement.ID, c.kind, c.name, c.op, c.value)
			}
		}
	}
}
