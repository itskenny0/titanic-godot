package save

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func sampleMetadata() Metadata {
	return Metadata{Version: 1, Globals: []Global{{"mission", float64(2)}, {"a_long_custom_global", "retained"}, {"clock", float64(123)}, {"precise", 1.25}}, State: State{Disk: "TitanicCD2", Set: "hallc.set", Scene: "scene2", View: "view7", Frame: 12345, Inventory: []SavedProp{{Name: "brush", View: "idle", Owner: "inventory", Visible: true, X: 256, Y: 192}}, Actors: []SavedActor{{Name: "steward", Owner: "none", Placement: Placement{Set: "hallc", Star: "anchor", Pose: "stand", Visible: true, Scale: 1000}}}, Loops: []SavedLoop{{Kind: "scene", Name: "scene2", Handler: "calctime", Period: 10}}, Crickets: []SavedCricket{}, Walks: []SavedWalk{}, CastFiles: []string{"gang.cst"}, TrackFiles: []string{"theme.trk"}}}
}
func TestRestoredSaveRoundTrip(t *testing.T) {
	base := NeutralTemplate()
	original := bytes.Clone(base)
	m := sampleMetadata()
	data, err := AppendMetadata(base, m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(base, original) {
		t.Fatal("writer modified the input save")
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.State, m.State) || got.NumGlobals["mission"] != 2 || got.NumGlobals["precise"] != 1.25 || got.StrGlobals["a_long_custom_global"] != "retained" || got.Clock != "123" {
		t.Fatalf("save lost state: %+v", got)
	}
	again, err := AppendMetadata(data, m)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("replacing trailer duplicated or changed the save", err)
	}
	legacy, err := Parse(base)
	if err != nil || legacy.Title != "Titanic 1.0" || legacy.Stage != "main.stg" {
		t.Fatal("neutral envelope is not a valid legacy save", err)
	}
	if path := os.Getenv("TAOOT_SAVE_OUTPUT"); path != "" {
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestDamagedRestoredSaves(t *testing.T) {
	base := NeutralTemplate()
	good, err := AppendMetadata(base, sampleMetadata())
	if err != nil {
		t.Fatal(err)
	}
	for end := len(base) + 1; end < len(good); end++ {
		if _, err = Parse(good[:end]); err == nil {
			t.Fatalf("accepted truncated trailer at %d", end)
		}
	}
	for _, at := range []int{2000, len(base) + len(metadataMark) + 20, len(good) - len(metadataMark) - 5} {
		bad := bytes.Clone(good)
		bad[at] ^= 1
		if _, err = Parse(bad); err == nil {
			t.Fatalf("accepted checksum failure at %d", at)
		}
	}
	for _, offset := range []int{20, 1024} {
		bad := bytes.Clone(base)
		binary.LittleEndian.PutUint32(bad[offset:], math.MaxUint32)
		if _, err = Parse(bad); err == nil {
			t.Fatal("accepted unbounded container index")
		}
	}
	raw, _ := ReadRaw(base)
	raw.Containers[6].Data = make([]byte, 1)
	bad, _ := WriteRaw(raw)
	if _, err = Parse(bad); err == nil {
		t.Fatal("accepted malformed track array")
	}
}
func TestMetadataValidation(t *testing.T) {
	valid, _ := json.Marshal(sampleMetadata())
	cases := []func(map[string]any){
		func(m map[string]any) { delete(m, "theme") },
		func(m map[string]any) { m["frame"] = "123" },
		func(m map[string]any) { m["version"] = 2 },
		func(m map[string]any) { m["frame"] = -1 },
		func(m map[string]any) { m["globals"] = []any{[]any{"Name", 1}, []any{"name", 2}} },
		func(m map[string]any) { m["globals"] = []any{[]any{"__internal", 1}} },
		func(m map[string]any) { m["inventory"] = nil },
		func(m map[string]any) {
			m["walks"] = []any{map[string]any{"actor": "steward", "star": "anchor", "type": 3, "turnTo": 0, "deg": 0, "startX": 0, "startY": 0, "startZ": 0, "destX": 0, "destY": 0, "destZ": 0, "progress": 0, "dist": 1, "hasPayload": true, "paused": false, "path": []any{}}}
		},
		func(m map[string]any) {
			m["loops"] = []any{map[string]any{"kind": "unknown", "name": "x", "handler": "x", "period": 1}}
		},
	}
	for i, change := range cases {
		var m map[string]any
		json.Unmarshal(valid, &m)
		change(m)
		b, _ := json.Marshal(m)
		if _, err := decodeMetadata(b); err == nil {
			t.Fatalf("accepted invalid metadata case %d", i)
		}
	}
	if _, err := decodeMetadata(append([]byte{0xef, 0xbb, 0xbf}, valid...)); err != nil {
		t.Fatal("UTF-8 BOM compatibility lost", err)
	}
	bad := bytes.Clone(valid)
	bad[20] = 0xff
	if _, err := decodeMetadata(bad); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}
func TestSaveReference(t *testing.T) {
	path := os.Getenv("TAOOT_SAVE_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_SAVE_REFERENCE to local save reference output")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Neutral string
		Saves   []struct {
			Path, Rewrite string
			State         Game
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	hash := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	if hash(NeutralTemplate()) != corpus.Neutral {
		t.Fatal("neutral template differs byte for byte")
	}
	for _, e := range corpus.Saves {
		t.Run(e.Path, func(t *testing.T) {
			b, err := os.ReadFile(e.Path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := Parse(b)
			if err != nil {
				t.Fatal(err)
			}
			raw := g.Raw
			g.Raw = nil
			g.Index = Index{}
			if !reflect.DeepEqual(*g, e.State) {
				t.Fatal("decoded save state differs from the reference")
			}
			out, err := WriteRaw(raw)
			if err != nil || hash(out) != e.Rewrite {
				t.Fatal("legacy rewrite differs byte for byte", err)
			}
		})
	}
	t.Logf("Compared %d saves against the existing reader", len(corpus.Saves))
}
func FuzzSaveReader(f *testing.F) {
	f.Add(NeutralTemplate())
	restored, _ := AppendMetadata(NeutralTemplate(), sampleMetadata())
	f.Add(restored)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = Parse(b)
	})
}

func TestLegacyVariablesAndPlacement(t *testing.T) {
	raw, err := ReadRaw(NeutralTemplate())
	if err != nil {
		t.Fatal(err)
	}
	vars := make([]byte, 32*13)
	pool := make([]byte, 64)
	writeField(pool, 0, "22:35", 15)
	for i := 0; i < 12; i++ {
		binary.LittleEndian.PutUint32(vars[i*32+20:], 0x12345678)
		binary.LittleEndian.PutUint16(vars[i*32+24:], 4)
		binary.LittleEndian.PutUint32(vars[i*32+26:], uint32(i*100))
		if i > 0 {
			writeField(vars, i*32+8, "var"+string(rune('a'+i)), 15)
		}
	}
	binary.LittleEndian.PutUint16(vars[24:], 3)
	writeField(vars, 40, "clock", 15)
	raw.Containers[7].Data = vars
	raw.Containers[8].Data = pool
	props := make([]byte, 158*2)
	actors := make([]byte, 160*2)
	for i := 0; i < 2; i++ {
		p := props[i*158:]
		binary.LittleEndian.PutUint16(p, 1)
		binary.LittleEndian.PutUint16(p[18:], 1)
		binary.LittleEndian.PutUint16(p[20:], 123)
		binary.LittleEndian.PutUint16(p[22:], 234)
		writeField(p, 78, "prop"+string(rune('a'+i)), 15)
		writeField(p, 126, "idle", 15)
		writeField(p, 142, "inventory", 15)
		a := actors[i*160:]
		binary.LittleEndian.PutUint16(a, 1)
		binary.LittleEndian.PutUint16(a[26:], 321)
		binary.LittleEndian.PutUint16(a[28:], 432)
		writeField(a, 80, "actor"+string(rune('a'+i)), 15)
		writeField(a, 96, "hallc", 15)
		writeField(a, 112, "anchor", 15)
		writeField(a, 128, "stand", 15)
		writeField(a, 144, "none", 15)
	}
	raw.Containers[4].Data = props
	raw.Containers[2].Data = actors
	data, err := WriteRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if g.Clock != "22:35" || g.NumGlobals["varc"] != 100 || len(g.Vars) != 11 {
		t.Fatal("shifted variable name/value records decoded incorrectly", len(g.Vars))
	}
	if len(g.Inventory) != 2 || !g.Inventory[0].Is3d || !g.Inventory[0].Visible || g.Inventory[0].X != 234 || g.Inventory[0].Y != 123 {
		t.Fatal("legacy prop placement differs")
	}
	if len(g.Actors) != 2 || g.Actors[0].Placement.Set != "hallc" || g.Actors[0].Placement.X != 321 || g.Actors[0].Placement.Y != 432 {
		t.Fatal("legacy actor placement differs")
	}
	// A long variable name overlaps its vtable, but the remaining bytes still
	// identify the record and its value remains in the preceding node.
	writeField(vars, 40, "longglobalname", 15)
	decoded := decodeVars(vars, pool)
	if len(decoded) != 11 || decoded[0].Name != "longglobalname" || decoded[0].Str == nil || *decoded[0].Str != "22:35" {
		t.Fatal("overlapping variable name lost its value")
	}
}
