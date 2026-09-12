package save

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"strconv"
	"strings"
)

func SaveIndex(raw *RawFile) (Index, error) {
	ix := Index{}
	n := len(raw.Containers)
	if n < 12 {
		return ix, fmt.Errorf("save has %d containers; at least 12 are required", n)
	}
	list := bytesView(raw.Containers[6].Data)
	if len(list)%40 != 0 {
		return ix, fmt.Errorf("save track descriptors are truncated")
	}
	tracks := len(list) / 40
	globals := 7 + 3*tracks
	if globals+5 > n {
		return ix, fmt.Errorf("save is missing track containers")
	}
	for k := 0; k < tracks; k++ {
		for j, off := range []int{4, 6, 8} {
			want := int(int16(list.u16(k*40+off))) * 104
			if len(raw.Containers[7+3*k+j].Data) != want {
				return ix, fmt.Errorf("save track %d array %d has an invalid size", k, j)
			}
		}
	}
	for j, size := range []int{32 * 42, 16 * 74, 16 * 110} {
		if len(raw.Containers[globals+2+j].Data) != size {
			return ix, fmt.Errorf("invalid save scheduler table %d", j)
		}
	}
	return Index{2, 3, 4, 5, 6, tracks, globals, globals + 1, globals + 2, globals + 3, globals + 4}, nil
}
func validObjectName(s string) bool {
	if len(s) < 2 || len(s) > 20 {
		return false
	}
	for i, c := range s {
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}
func propAt(d bytesView, o int) *SavedProp {
	name := field(d, o)
	if !validObjectName(name) {
		return nil
	}
	view, owner := field(d, o+48), field(d, o+64)
	if view == "" || owner == "" {
		return nil
	}
	base := o - 78
	return &SavedProp{Name: strings.ToLower(name), View: strings.ToLower(view), Owner: strings.ToLower(owner), Visible: d.i16(base) > 0, Is3d: d.i16(base+18) == 1, X: d.i16(base + 22), Y: d.i16(base + 20), Deg: d.i16(base + 24), Dist: d.i16(base + 38), Scale: d.i16(base + 40), Value: d.i32(base + 70), Zclip: d.i16(base + 74)}
}
func actorAt(d bytesView, o int) *SavedActor {
	name, owner := field(d, o), field(d, o+64)
	if !validObjectName(name) || owner == "" {
		return nil
	}
	base := o - 80
	return &SavedActor{Name: strings.ToLower(name), Owner: strings.ToLower(owner), Value: d.i32(base + 72), Placement: Placement{Visible: d.i16(base) > 0, Set: strings.ToLower(field(d, o+16)), Star: strings.ToLower(field(d, o+32)), Pose: strings.ToLower(field(d, o+48)), Deg: d.i16(base + 24), X: d.i16(base + 26), Y: d.i16(base + 28), Z: d.i16(base + 30), Speed: d.i16(base + 38), Turn: d.i16(base + 32), Scale: d.i16(base + 42), Zclip: d.i16(base + 76)}}
}
func walkGrid[T any](d bytesView, stride int, read func(bytesView, int) *T) []T {
	out := []T{}
	last, seed := len(d)-64, -1
	for o := 0; o < last; o++ {
		if read(d, o) != nil && read(d, o+stride) != nil {
			seed = o
			break
		}
	}
	if seed < 0 {
		return out
	}
	base := seed
	for base-stride >= 0 && read(d, base-stride) != nil {
		base -= stride
	}
	for o := base; o < last; o += stride {
		v := read(d, o)
		if v == nil {
			break
		}
		out = append(out, *v)
	}
	return out
}

var loopKinds = []string{"", "actor", "prop", "scene", "flat"}

func decodeLoops(d bytesView) []SavedLoop {
	out := []SavedLoop{}
	for s := 0; s+42 <= len(d); s += 42 {
		if d.u16(s) == 0 {
			continue
		}
		kind := int(d.u16(s + 4))
		name, handler := field(d, s+10), field(d, s+26)
		if kind <= 0 || kind >= len(loopKinds) || name == "" || handler == "" {
			continue
		}
		out = append(out, SavedLoop{Kind: loopKinds[kind], Name: strings.ToLower(name), Handler: strings.ToLower(handler), Period: float64(d.u32(s + 6))})
	}
	return out
}
func decodeCrickets(d bytesView) []SavedCricket {
	out := []SavedCricket{}
	for s := 0; s+74 <= len(d); s += 74 {
		if d.u16(s) == 0 {
			continue
		}
		name := field(d, s+58)
		if name == "" {
			continue
		}
		out = append(out, SavedCricket{Name: strings.ToLower(name), Set: strings.ToLower(field(d, s+42)), X: d.i16(s + 4), Y: d.i16(s + 6), Radius: float64(d.u32(s + 8)), Base: float64(d.u32(s + 12)), Jitter: d.i32(s + 16), Next: float64(d.u32(s + 20))})
	}
	return out
}
func decodeWalkPath(d bytesView) []Waypoint {
	if len(d) < 20 {
		return nil
	}
	n := int(d.u32(8))
	if n < 2 || n > (len(d)-20)/8 {
		return nil
	}
	out := make([]Waypoint, n)
	cum := 0.
	for i := range out {
		o := 20 + i*8
		cum += float64(d.u16(o + 6))
		out[i] = Waypoint{X: d.i16(o), Y: d.i16(o + 2), Z: d.i16(o + 4), Cum: cum}
	}
	if cum != float64(d.u32(0)) {
		return nil
	}
	return out
}
func decodeWalks(d bytesView, payloads []df.Container) []SavedWalk {
	out := []SavedWalk{}
	for s := 0; s+110 <= len(d); s += 110 {
		if d.u16(s) == 0 {
			continue
		}
		actor := field(d, s+46)
		if actor == "" {
			continue
		}
		w := SavedWalk{Actor: strings.ToLower(actor), Type: d.i16(s + 4), HasPayload: d.u32(s+18) != 0, Paused: d.u16(s+2) != 0, TurnTo: d.i16(s + 8), Deg: d.i16(s + 10), StartX: d.i16(s + 12), StartY: d.i16(s + 14), StartZ: d.i16(s + 16), Star: field(d, s+62)}
		if w.HasPayload && len(payloads) > 0 {
			w.Path = decodeWalkPath(payloads[0].Data)
			payloads = payloads[1:]
		}
		w.DestX, w.DestY, w.DestZ = w.StartX, w.StartY, w.StartZ
		if w.Type == 1 {
			w.DestX -= d.i32(s + 26)
			w.DestY -= d.i32(s + 30)
			w.DestZ -= d.i32(s + 34)
			w.Dist = d.i32(s + 38)
		}
		if w.Type == 1 || w.Path != nil {
			w.Progress = d.i32(s + 22)
		}
		if w.Path != nil {
			w.Dist = w.Path[len(w.Path)-1].Cum
		}
		out = append(out, w)
	}
	return out
}
func decodeTheme(raw *RawFile, ix Index) *SavedTheme {
	list := bytesView(raw.Containers[ix.Tracks].Data)
	type liveTrack struct {
		track  string
		volume float64
		count  int
		names  map[string]bool
	}
	live := []liveTrack{}
	for k := 0; k < ix.TrackCount; k++ {
		t := liveTrack{track: strings.ToLower(field(list, k*40+22)), volume: 255, names: map[string]bool{}}
		for _, j := range []int{1, 2} {
			arr := bytesView(raw.Containers[ix.Tracks+1+3*k+j].Data)
			for s := 0; s+104 <= len(arr); s += 104 {
				if t.count == 0 {
					t.volume = float64(arr.u16(s + 4))
				}
				t.count++
				t.names[strings.ToLower(field(arr, s+8))] = true
			}
		}
		if t.count > 0 {
			live = append(live, t)
		}
	}
	if len(live) == 0 {
		return nil
	}
	best := 0
	for i := range live {
		if live[i].count >= live[best].count {
			best = i
		}
	}
	extras := 0
	for i, t := range live {
		if i != best {
			extras += len(t.names)
		}
	}
	return &SavedTheme{Track: live[best].track, Volume: live[best].volume, Extras: float64(extras)}
}

var hallDeck = map[string]string{"halla": "a", "hallb": "b", "hallc": "c", "halld": "d", "hallf2c": "f", "hallf3c": "f", "decka": "a", "deckbd": "bd", "deckbd2": "bd"}

func ParseLegacy(b []byte) (*Game, error) {
	raw, err := ReadRaw(b)
	if err != nil {
		return nil, err
	}
	ix, err := SaveIndex(raw)
	if err != nil {
		return nil, err
	}
	c0, c1 := raw.Containers[0].Data, bytesView(raw.Containers[1].Data)
	title := df.NewReader(c0, nil)
	name := title.PString()
	if title.Err != nil {
		return nil, title.Err
	}
	g := &Game{Title: name, Stage: field(c1, 520), Raw: raw, Index: ix, NumGlobals: map[string]float64{}, StrGlobals: map[string]string{}}
	g.Disk, g.Set, g.Scene, g.View = field(c0, 256), field(c1, 596), field(c1, 612), field(c1, 628)
	g.Frame = float64(c1.u32(442))
	g.Vars = decodeVars(raw.Containers[ix.Globals].Data, raw.Containers[ix.Pool].Data)
	for _, v := range g.Vars {
		if v.Type == 3 {
			if _, ok := g.StrGlobals[v.Name]; !ok && v.Str != nil {
				g.StrGlobals[v.Name] = *v.Str
			}
		} else {
			if _, ok := g.NumGlobals[v.Name]; !ok {
				g.NumGlobals[v.Name] = v.Num
			}
		}
	}
	g.Inventory = walkGrid(raw.Containers[ix.Inventory].Data, 158, propAt)
	g.Actors = walkGrid(raw.Containers[ix.Actors].Data, 160, actorAt)
	g.CastFiles = []string{}
	casts := raw.Containers[ix.Casts].Data
	for o := 0; o+28 <= len(casts); o += 28 {
		if name := checkedString(casts, o+12, 1, 12); name != nil {
			g.CastFiles = append(g.CastFiles, strings.ToLower(*name))
		}
	}
	g.TrackFiles = []string{}
	tracks := raw.Containers[ix.Tracks].Data
	for k := 0; k < ix.TrackCount; k++ {
		if name := field(tracks, k*40+22); name != "" {
			g.TrackFiles = append(g.TrackFiles, strings.ToLower(name))
		}
	}
	g.Loops = decodeLoops(raw.Containers[ix.Loops].Data)
	g.Crickets = decodeCrickets(raw.Containers[ix.Crickets].Data)
	g.Walks = decodeWalks(raw.Containers[ix.Walks].Data, raw.Containers[ix.Walks+1:])
	g.Theme = decodeTheme(raw, ix)
	g.Hallside = g.StrGlobals["hallside"]
	g.Savedeck = g.StrGlobals["savedeck"]
	if _, ok := g.StrGlobals["savedeck"]; !ok {
		g.Savedeck = hallDeck[strings.ToLower(g.Set)]
	}
	g.Clock = g.StrGlobals["clock"]
	if _, ok := g.StrGlobals["clock"]; !ok {
		if n, ok := g.NumGlobals["clock"]; ok {
			g.Clock = strconv.FormatFloat(n, 'f', -1, 64)
		}
	}
	return g, nil
}
