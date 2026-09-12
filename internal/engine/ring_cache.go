package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"slices"
)

const RingBudgetBytes = 24 * 1024 * 1024

type CameraPose struct{ X, Y, Z, Deg float64 }
type CachedFrame struct {
	Pixels, Z     []byte
	Width, Height int
	Camera        *CameraPose
	ID            uint64
}
type cachedRing struct {
	Register *df.FrameRegister
	Frames   map[int]*CachedFrame
	Bytes    int
	Used     uint64
}

// RingCache decodes authored deltas in order and retains a bounded LRU of rings.
// Callers may retain returned frames for an active movement after cache eviction.
type RingCache struct {
	Set          *df.Set
	Budget       int
	rings        []*cachedRing
	index        map[*df.FrameRegister]*cachedRing
	clock        uint64
	decodedBytes int
}

func NewRingCache(set *df.Set) *RingCache {
	c := &RingCache{Set: set, Budget: RingBudgetBytes, index: map[*df.FrameRegister]*cachedRing{}}
	add := func(reg *df.FrameRegister) {
		r := &cachedRing{Register: reg}
		c.index[reg] = r
		c.rings = append(c.rings, r)
	}
	for i := range set.Scenes {
		for dir := 0; dir < 2; dir++ {
			add(&set.Scenes[i].Turns[dir])
		}
	}
	for i := range set.Transitions {
		for j := range set.Transitions[i].FrameRegisters {
			add(&set.Transitions[i].FrameRegisters[j])
		}
	}
	return c
}
func (c *RingCache) DecodedBytes() int { return c.decodedBytes }
func (c *RingCache) NeedsDecode(reg *df.FrameRegister) bool {
	r := c.index[reg]
	return r != nil && r.Frames == nil
}
func (c *RingCache) Ensure(reg *df.FrameRegister) (map[int]*CachedFrame, error) {
	ring := c.index[reg]
	if ring == nil {
		return map[int]*CachedFrame{}, nil
	}
	c.clock++
	ring.Used = c.clock
	if ring.Frames != nil {
		return ring.Frames, nil
	}
	fb := new(df.FrameBuffer)
	decoded := map[int]*CachedFrame{}
	bytes := 0
	for _, fi := range reg.Frames {
		loc := fi.FrameContainerLoc
		if loc == 0 {
			continue
		}
		frame, err := df.DecodeFrame(c.Set.File.Data(loc), fb, c.Set.File.Order)
		if err != nil {
			return nil, err
		}
		// Duplicate locations still update the delta decoder, but the first view of
		// that location owns its cached image and camera pose.
		if decoded[loc] != nil {
			continue
		}
		n := frame.Width * frame.Height
		f := &CachedFrame{Pixels: slices.Clone(fb.Pixels[:n]), Width: frame.Width, Height: frame.Height, Camera: &CameraPose{X: float64(fi.PosX16), Y: float64(fi.PosZ16), Z: float64(fi.PosY16), Deg: float64(fi.AxisX8 & 255)}, ID: newDrawID()}
		if frame.HasZ {
			f.Z = slices.Clone(fb.ZPixels[:n])
			bytes += n
		}
		bytes += n
		decoded[loc] = f
	}
	ring.Frames = decoded
	ring.Bytes = bytes
	c.decodedBytes += bytes
	c.evict()
	return decoded, nil
}
func (c *RingCache) evict() {
	if c.decodedBytes <= c.Budget {
		return
	}
	live := []*cachedRing{}
	for _, r := range c.rings {
		if r.Frames != nil {
			live = append(live, r)
		}
	}
	slices.SortFunc(live, func(a, b *cachedRing) int {
		if a.Used < b.Used {
			return -1
		}
		if a.Used > b.Used {
			return 1
		}
		return 0
	})
	for _, r := range live {
		if c.decodedBytes <= c.Budget || len(live) < 2 {
			break
		}
		if r.Used == c.clock {
			continue
		}
		c.decodedBytes -= r.Bytes
		r.Frames = nil
		r.Bytes = 0
	}
}
