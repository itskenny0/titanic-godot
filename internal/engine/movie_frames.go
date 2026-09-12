package engine

import (
	"container/list"
	"slices"

	"github.com/itskenny0/titanic-godot/internal/df"
)

type MovieImage struct {
	Pixels        []byte
	Width, Height int
	ID            uint64
}
type movieImageEntry struct {
	index int
	image *MovieImage
}
type movieCheckpoint struct {
	index  int
	buffer df.FrameBuffer
	shown  []byte
}

func (c *movieCheckpoint) bytes() int {
	return len(c.buffer.Pixels) + len(c.buffer.ZPixels) + len(c.shown)
}

// MovieFrames decodes deltas on demand. Sparse restart points bound seek cost
// without retaining the entire credits or intro movie in handheld memory.
type MovieFrames struct {
	Segment                       *df.MovieSegment
	ImageBudget, CheckpointBudget int
	TransparentV1                 bool
	DecodedFrames                 int
	fb                            df.FrameBuffer
	position                      int
	shown                         []byte
	images, checkpoints           list.List
	imageIndex, checkpointIndex   map[int]*list.Element
	imageBytes, checkpointBytes   int
}

func NewMovieFrames(seg *df.MovieSegment) *MovieFrames {
	return &MovieFrames{Segment: seg, ImageBudget: 2 << 20, CheckpointBudget: 4 << 20, position: -1, imageIndex: map[int]*list.Element{}, checkpointIndex: map[int]*list.Element{}}
}
func (m *MovieFrames) Len() int { return len(m.Segment.Frames) }
func (m *MovieFrames) RetainedBytes() int {
	return m.imageBytes + m.checkpointBytes + len(m.fb.Pixels) + len(m.fb.ZPixels) + len(m.shown)
}
func (m *MovieFrames) Get(index int) (*MovieImage, error) {
	if index < 0 || index >= m.Len() {
		return nil, nil
	}
	if e := m.imageIndex[index]; e != nil {
		m.images.MoveToBack(e)
		return e.Value.(movieImageEntry).image, nil
	}
	if index <= m.position {
		m.restore(index)
	}
	var result *MovieImage
	for m.position < index {
		next := m.position + 1
		meta := m.Segment.Frames[next]
		f, err := df.DecodeFrame(m.Segment.File.Data(meta.LocationFrame), &m.fb, m.Segment.File.Order)
		if err != nil {
			return nil, err
		}
		m.DecodedFrames++
		if next == index || m.TransparentV1 {
			pixels := slices.Clone(m.fb.Pixels[:f.Width*f.Height])
			if m.TransparentV1 {
				if len(m.shown) == len(pixels) {
					for i, p := range pixels {
						if p == 0 || p == 255 {
							pixels[i] = m.shown[i]
						}
					}
				}
				m.shown = pixels
			}
			if next == index {
				result = &MovieImage{Pixels: pixels, Width: f.Width, Height: f.Height, ID: newDrawID()}
			}
		}
		m.position = next
		if next%32 == 0 {
			m.checkpoint()
		}
	}
	if result != nil {
		m.imageIndex[index] = m.images.PushBack(movieImageEntry{index, result})
		m.imageBytes += len(result.Pixels)
		for m.imageBytes > m.ImageBudget && m.images.Len() > 1 {
			e := m.images.Front()
			old := e.Value.(movieImageEntry)
			m.imageBytes -= len(old.image.Pixels)
			delete(m.imageIndex, old.index)
			m.images.Remove(e)
		}
	}
	return result, nil
}
func (m *MovieFrames) checkpoint() {
	if m.checkpointIndex[m.position] != nil {
		return
	}
	size := len(m.fb.Pixels) + len(m.fb.ZPixels) + len(m.shown)
	if size > m.CheckpointBudget {
		return
	}
	c := movieCheckpoint{index: m.position, buffer: df.FrameBuffer{Pixels: slices.Clone(m.fb.Pixels), ZPixels: slices.Clone(m.fb.ZPixels), Width: m.fb.Width, Height: m.fb.Height}, shown: slices.Clone(m.shown)}
	m.checkpointIndex[m.position] = m.checkpoints.PushBack(c)
	m.checkpointBytes += size
	for m.checkpointBytes > m.CheckpointBudget {
		e := m.checkpoints.Front()
		old := e.Value.(movieCheckpoint)
		m.checkpointBytes -= old.bytes()
		delete(m.checkpointIndex, old.index)
		m.checkpoints.Remove(e)
	}
}
func (m *MovieFrames) restore(index int) {
	nearest := -1
	var selected *list.Element
	for at, e := range m.checkpointIndex {
		if at < index && at > nearest {
			nearest, selected = at, e
		}
	}
	m.fb = df.FrameBuffer{}
	m.shown = nil
	m.position = nearest
	if selected != nil {
		c := selected.Value.(movieCheckpoint)
		m.fb = df.FrameBuffer{Pixels: slices.Clone(c.buffer.Pixels), ZPixels: slices.Clone(c.buffer.ZPixels), Width: c.buffer.Width, Height: c.buffer.Height}
		m.shown = slices.Clone(c.shown)
		m.checkpoints.MoveToBack(selected)
	}
}
