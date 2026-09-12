package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/df"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestRingCacheEvictionAndDeltaIsolation(t *testing.T) {
	// Literal 2x1 indexed images. Ring 2 must not modify retained ring 1 pixels.
	file := &df.File{Containers: []df.Container{{}, {Data: []byte{1, 0, 2, 0, 4, 1, 2}}, {Data: []byte{1, 0, 2, 0, 4, 3, 4}}}}
	set := &df.Set{File: file, Scenes: []df.Scene{{Turns: [2]df.FrameRegister{{Frames: []df.FrameInfo{{FrameContainerLoc: 1, PosX16: 17}, {FrameContainerLoc: 1, PosX16: 99}}}, {Frames: []df.FrameInfo{{FrameContainerLoc: 2}}}}}}}
	c := NewRingCache(set)
	c.Budget = 2
	first, err := c.Ensure(&set.Scenes[0].Turns[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[1].Camera.X != 17 || c.DecodedBytes() != 2 {
		t.Fatal("duplicate container replaced the first camera or inflated memory")
	}
	if _, err = c.Ensure(&set.Scenes[0].Turns[1]); err != nil {
		t.Fatal(err)
	}
	if !c.NeedsDecode(&set.Scenes[0].Turns[0]) || c.DecodedBytes() != 2 || !bytes.Equal(first[1].Pixels, []byte{1, 2}) {
		t.Fatal("LRU retained old ring or damaged an active image")
	}
	again, err := c.Ensure(&set.Scenes[0].Turns[0])
	if err != nil || again[1] == first[1] || !bytes.Equal(again[1].Pixels, first[1].Pixels) {
		t.Fatal("evicted ring failed to reconstruct", err)
	}
	unknown := new(df.FrameRegister)
	empty, err := c.Ensure(unknown)
	if err != nil || len(empty) != 0 || c.NeedsDecode(unknown) {
		t.Fatal("unregistered ring admitted")
	}
}
func TestRingCacheReference(t *testing.T) {
	path := os.Getenv("TAOOT_RING_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_RING_REFERENCE to owned-data ring output")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type frameRef struct {
		Loc, Width, Height int
		Pixels, Z          string
		Camera             *CameraPose
	}
	var corpus []struct {
		Path   string
		Visits []struct {
			Index, Bytes int
			Needed       bool
			Frames       []frameRef
		}
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			set, err := df.ReadSet(data)
			if err != nil {
				t.Fatal(err)
			}
			c := NewRingCache(set)
			regs := []*df.FrameRegister{}
			for i := range set.Scenes {
				for j := range set.Scenes[i].Turns {
					regs = append(regs, &set.Scenes[i].Turns[j])
				}
			}
			for i := range set.Transitions {
				for j := range set.Transitions[i].FrameRegisters {
					regs = append(regs, &set.Transitions[i].FrameRegisters[j])
				}
			}
			for _, visit := range entry.Visits {
				reg := regs[visit.Index]
				if c.NeedsDecode(reg) != visit.Needed {
					t.Fatal("cache reuse differs", visit.Index)
				}
				frames, err := c.Ensure(reg)
				if err != nil {
					t.Fatal(err)
				}
				if c.DecodedBytes() != visit.Bytes {
					t.Fatal("LRU byte accounting differs", visit.Index, c.DecodedBytes(), visit.Bytes)
				}
				got := []frameRef{}
				seen := map[int]bool{}
				for _, fi := range reg.Frames {
					loc := fi.FrameContainerLoc
					if loc == 0 || seen[loc] {
						continue
					}
					seen[loc] = true
					f := frames[loc]
					z := ""
					if f.Z != nil {
						z = digest(f.Z)
					}
					got = append(got, frameRef{loc, f.Width, f.Height, digest(f.Pixels), z, f.Camera})
				}
				if !reflect.DeepEqual(got, visit.Frames) {
					t.Fatal("decoded ring pixels, depth or camera differ", visit.Index)
				}
			}
		})
	}
}
func TestPaletteDimPreservesAlphaAndEvenRounding(t *testing.T) {
	base := []byte{1, 3, 5, 17, 255, 127, 63, 29}
	before := slices.Clone(base)
	got := DimPalette(base, ClutDim{Lo: 0, Hi: 0, Amt: 127.5})
	if !bytes.Equal(got, []byte{0, 2, 2, 17, 255, 127, 63, 29}) || !bytes.Equal(base, before) {
		t.Fatal("palette dim lost alpha, changed its source or rounded half values up", got)
	}
}
