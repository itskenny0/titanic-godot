package df

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func canonicalAsset(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			key := strings.ToLower(k[:1]) + k[1:]
			if k == "IDOffset" {
				key = "idOffset"
			}
			if k == "IDLimit" {
				key = "idLimit"
			}
			out[key] = canonicalAsset(v)
		}
		return out
	case []any:
		for i := range x {
			x[i] = canonicalAsset(x[i])
		}
		return x
	default:
		return v
	}
}
func firstAssetDifference(a, b any, path string) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			for k, v := range am {
				if w, found := bm[k]; !found {
					return path + "." + k + " missing"
				} else if diff := firstAssetDifference(v, w, path+"."+k); diff != "" {
					return diff
				}
			}
			for k := range bm {
				if _, found := am[k]; !found {
					return path + "." + k + " unexpected"
				}
			}
		}
	}
	if aa, ok := a.([]any); ok {
		if bb, ok := b.([]any); ok && len(aa) == len(bb) {
			for i := range aa {
				if diff := firstAssetDifference(aa[i], bb[i], path+"[]"); diff != "" {
					return diff
				}
			}
		}
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if len(aj) > 200 {
		aj = aj[:200]
	}
	if len(bj) > 200 {
		bj = bj[:200]
	}
	return path + ": expected " + string(aj) + ", got " + string(bj)
}
func TestAssetReference(t *testing.T) {
	path := os.Getenv("TAOOT_ASSET_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_ASSET_REFERENCE to the owned-data oracle")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Path, Kind string
		Expected   any
		Regions    any
	}
	if err = json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, e := range entries {
		bytes, err := os.ReadFile(e.Path)
		if err != nil {
			t.Fatal(err)
		}
		var value any
		switch e.Kind {
		case ".mov":
			var m *Movie
			m, err = ReadMovie(bytes)
			if m != nil {
				value = m.Segments
			}
		case ".set":
			value, err = ReadSet(bytes)
		case ".stg":
			value, err = ReadStage(bytes)
		case ".shp":
			value, err = ReadShop(bytes)
		case ".cst":
			value, err = ReadCast(bytes)
		case ".pup":
			value, err = ReadPuppet(bytes)
		case ".sfx", ".trk", ".11k":
			value, err = ReadAudioBank(bytes)
		default:
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", e.Path, err)
		}
		actual, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var normalized any
		if err = json.Unmarshal(actual, &normalized); err != nil {
			t.Fatal(err)
		}
		normalized = canonicalAsset(normalized)
		if diff := firstAssetDifference(e.Expected, normalized, "root"); diff != "" {
			t.Fatalf("%s: %s", e.Path, diff)
		}
		counts[e.Kind]++
		if stage, ok := value.(*Stage); ok {
			regions := [][]StageRegion{}
			for _, f := range stage.Flats {
				regions = append(regions, ReadStageRegions(stage.File.Data(f.LocationClickLogic), stage.Version))
			}
			b, _ := json.Marshal(regions)
			var result any
			json.Unmarshal(b, &result)
			if diff := firstAssetDifference(e.Regions, canonicalAsset(result), "regions"); diff != "" {
				t.Fatalf("%s: %s", e.Path, diff)
			}
		}
	}
	t.Logf("Compared asset metadata: %v", counts)
}

func TestAssetPixelsAndTracks(t *testing.T) {
	path := os.Getenv("TAOOT_ASSET_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_ASSET_REFERENCE to the owned-data oracle")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path, Kind string
		Sprites    []struct {
			Loc, Width, Height, PosXraw, PosYraw int
			Indexed, Opaque                      string
		}
		Animations []struct {
			Loc    int
			Frames []PuppetAnimFrame
		}
		Paths []struct {
			Loc    int
			Points []StarPathPoint
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	sprites, animations, paths := 0, 0, 0
	for _, e := range corpus {
		if len(e.Sprites)+len(e.Animations)+len(e.Paths) == 0 {
			continue
		}
		data, err := os.ReadFile(e.Path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := ReadFile(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range e.Sprites {
			s, err := DecodeSprite(f.Data(want.Loc))
			if err != nil {
				t.Fatalf("%s sprite %d: %v", e.Path, want.Loc, err)
			}
			if s.Width != want.Width || s.Height != want.Height || s.PosXraw != want.PosXraw || s.PosYraw != want.PosYraw || digest(s.Indexed) != want.Indexed || digest(s.Opaque) != want.Opaque {
				t.Fatalf("%s sprite %d differs", e.Path, want.Loc)
			}
			sprites++
		}
		p := Puppet{File: f}
		for _, want := range e.Animations {
			frames := p.AnimLogic(want.Loc)
			if !reflect.DeepEqual(want.Frames, frames) {
				t.Fatalf("%s animation %d differs", e.Path, want.Loc)
			}
			animations += len(frames)
		}
		for _, want := range e.Paths {
			points, err := ReadStarPath(f, want.Loc, 4)
			if err != nil || !reflect.DeepEqual(points, want.Points) {
				t.Fatalf("%s star path %d differs: %v", e.Path, want.Loc, err)
			}
			paths++
		}
	}
	t.Logf("Compared %d sprites, %d puppet animation frames, %d star paths", sprites, animations, paths)
}
