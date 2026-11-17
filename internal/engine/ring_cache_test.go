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

func ringFixture() (*df.Set, *RingCache) {
	file := &df.File{Containers: []df.Container{{}, {Data: []byte{1, 0, 2, 0, 4, 1, 2}}, {Data: []byte{1, 0, 2, 0, 4, 3, 4}}, {Data: []byte{9}}}}
	set := &df.Set{File: file, Scenes: []df.Scene{
		{Turns: [2]df.FrameRegister{{Frames: []df.FrameInfo{{FrameContainerLoc: 1, PosX16: 17}}}, {Frames: []df.FrameInfo{{FrameContainerLoc: 2}}}}},
		{Turns: [2]df.FrameRegister{{Frames: []df.FrameInfo{{FrameContainerLoc: 3}}}, {Frames: []df.FrameInfo{{FrameContainerLoc: 1}, {FrameContainerLoc: 2}}}}},
	}}
	return set, NewRingCache(set)
}

func TestRingCachePrefetchDecodesOffTheGameThread(t *testing.T) {
	set, c := ringFixture()
	right, left := &set.Scenes[0].Turns[0], &set.Scenes[0].Turns[1]
	if !c.Prefetch(right) || len(c.jobs) != 0 || !c.NeedsDecode(right) {
		t.Fatal("sequential cache started background work")
	}
	c.Background = true
	if !c.Prefetch(right) || !c.Prefetch(right) || len(c.jobs) != 1 {
		t.Fatal("prefetch did not start exactly one job for the ring")
	}
	if !c.NeedsDecode(right) {
		t.Fatal("ring appeared before the game thread adopted it")
	}
	<-c.jobs[0].done
	if c.Prefetch(right) || c.NeedsDecode(right) || len(c.jobs) != 0 || c.DecodedBytes() != 2 {
		t.Fatal("finished ring was not adopted", c.DecodedBytes())
	}
	// Ensure must wait for a ring in flight instead of decoding it again.
	c.Prefetch(left)
	job := c.jobs[0]
	frames, err := c.Ensure(left)
	if err != nil || len(c.jobs) != 0 || frames[2] != job.frames[2] || !bytes.Equal(frames[2].Pixels, []byte{3, 4}) {
		t.Fatal("ensure did not adopt the in-flight ring", err)
	}
	_, sequential := ringFixture()
	want, _ := sequential.Ensure(&sequential.Set.Scenes[0].Turns[0])
	got, _ := c.Ensure(right)
	if !bytes.Equal(got[1].Pixels, want[1].Pixels) || *got[1].Camera != *want[1].Camera || c.DecodedBytes() != 4 {
		t.Fatal("background ring differs from a sequential decode")
	}
}

func TestRingCachePrefetchLimitsWorkersAndSurvivesBadRings(t *testing.T) {
	set, c := ringFixture()
	c.Background = true
	bad, far := &set.Scenes[1].Turns[0], &set.Scenes[1].Turns[1]
	regs := []*df.FrameRegister{&set.Scenes[0].Turns[0], &set.Scenes[0].Turns[1], far}
	for _, reg := range regs {
		if !c.Prefetch(reg) {
			t.Fatal("ring not outstanding")
		}
	}
	if len(c.jobs) > ringWorkers {
		t.Fatal("too many concurrent decodes", len(c.jobs))
	}
	for _, reg := range regs {
		for c.Prefetch(reg) {
			for _, job := range c.jobs {
				<-job.done
			}
		}
	}
	if c.DecodedBytes() != 8 {
		t.Fatal("queued rings were not all decoded", c.DecodedBytes())
	}
	// A damaged ring must not be retried forever, and still reports its error
	// to the caller that needs it.
	c.Prefetch(bad)
	<-c.jobs[0].done
	if c.Prefetch(bad) || len(c.jobs) != 0 || !c.NeedsDecode(bad) {
		t.Fatal("damaged ring keeps being prefetched")
	}
	if _, err := c.Ensure(bad); err == nil {
		t.Fatal("damaged ring error was swallowed")
	}
	// Touching keeps nearby rings ahead of stale ones in eviction order.
	c.Touch(regs[0])
	c.Budget = 6
	c.evict()
	if c.NeedsDecode(regs[0]) || !c.NeedsDecode(regs[1]) {
		t.Fatal("eviction ignored the most recently wanted ring")
	}
}
