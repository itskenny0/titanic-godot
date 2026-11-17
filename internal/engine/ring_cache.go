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
	failed   bool
}

// ringJob is one ring decoding off the game thread. The worker owns it until
// done closes; only then does the game thread read the result.
type ringJob struct {
	ring   *cachedRing
	done   chan struct{}
	frames map[int]*CachedFrame
	bytes  int
	err    error
}

// ringWorkers bounds concurrent background decodes. Two leave the game and
// audio threads their own cores on a four-core handheld.
const ringWorkers = 2

// RingCache decodes authored deltas in order and retains a bounded LRU of rings.
// Callers may retain returned frames for an active movement after cache eviction.
//
// With Background set, Prefetch decodes rings the player is likely to need on
// worker goroutines, so turning and walking do not stall on the decoder. All
// cache state still changes only on the game thread: workers return finished
// frames, and Prefetch or Ensure adopts them.
type RingCache struct {
	Set          *df.Set
	Budget       int
	Background   bool
	rings        []*cachedRing
	index        map[*df.FrameRegister]*cachedRing
	clock        uint64
	decodedBytes int
	jobs         []*ringJob
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

// ringSource captures a ring's compressed frames on the game thread, so a
// worker never reads the container table.
type ringSource struct {
	info df.FrameInfo
	data []byte
}

func (c *RingCache) source(reg *df.FrameRegister) []ringSource {
	out := make([]ringSource, 0, len(reg.Frames))
	for _, fi := range reg.Frames {
		if fi.FrameContainerLoc != 0 {
			out = append(out, ringSource{fi, c.Set.File.Data(fi.FrameContainerLoc)})
		}
	}
	return out
}
func decodeRing(source []ringSource, order df.Order) (map[int]*CachedFrame, int, error) {
	fb := new(df.FrameBuffer)
	decoded := map[int]*CachedFrame{}
	bytes := 0
	for _, src := range source {
		fi, loc := src.info, src.info.FrameContainerLoc
		frame, err := df.DecodeFrame(src.data, fb, order)
		if err != nil {
			return nil, 0, err
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
	return decoded, bytes, nil
}
func (c *RingCache) store(ring *cachedRing, frames map[int]*CachedFrame, bytes int) {
	c.clock++
	ring.Used = c.clock
	ring.Frames = frames
	ring.Bytes = bytes
	c.decodedBytes += bytes
	c.evict()
}

// collect adopts finished background rings. A ring that failed is left for
// Ensure, which reports the same error to the caller that actually needs it.
func (c *RingCache) collect() {
	pending := c.jobs[:0]
	for _, job := range c.jobs {
		select {
		case <-job.done:
			c.adopt(job)
		default:
			pending = append(pending, job)
		}
	}
	clear(c.jobs[len(pending):])
	c.jobs = pending
}
func (c *RingCache) adopt(job *ringJob) {
	if job.err != nil {
		job.ring.failed = true
	} else if job.ring.Frames == nil {
		c.store(job.ring, job.frames, job.bytes)
	}
}

// Prefetch asks for a ring to be ready soon without waiting for it. It reports
// whether the ring is still outstanding; false means it is decoded, unknown,
// or cannot be decoded.
func (c *RingCache) Prefetch(reg *df.FrameRegister) bool {
	c.collect()
	ring := c.index[reg]
	if ring == nil || ring.Frames != nil || ring.failed {
		return false
	}
	for _, job := range c.jobs {
		if job.ring == ring {
			return true
		}
	}
	if !c.Background || len(c.jobs) >= ringWorkers {
		return true
	}
	job := &ringJob{ring: ring, done: make(chan struct{})}
	source, order := c.source(reg), c.Set.File.Order
	c.jobs = append(c.jobs, job)
	go func() {
		defer close(job.done)
		job.frames, job.bytes, job.err = decodeRing(source, order)
	}()
	return true
}

// Touch marks a decoded ring as recently wanted, so prefetching further
// ahead evicts stale rings before the ones around the player.
func (c *RingCache) Touch(reg *df.FrameRegister) {
	if ring := c.index[reg]; ring != nil && ring.Frames != nil {
		c.clock++
		ring.Used = c.clock
	}
}
func (c *RingCache) Ensure(reg *df.FrameRegister) (map[int]*CachedFrame, error) {
	ring := c.index[reg]
	if ring == nil {
		return map[int]*CachedFrame{}, nil
	}
	// Already decoding in the background: finishing that is quicker than
	// starting over.
	for i, job := range c.jobs {
		if job.ring == ring {
			<-job.done
			c.jobs = slices.Delete(c.jobs, i, i+1)
			c.adopt(job)
			break
		}
	}
	c.clock++
	ring.Used = c.clock
	if ring.Frames != nil {
		return ring.Frames, nil
	}
	decoded, bytes, err := decodeRing(c.source(reg), c.Set.File.Order)
	if err != nil {
		return nil, err
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
