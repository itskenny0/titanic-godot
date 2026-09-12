package engine

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func TestMovieFramesBoundedSeeking(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		seg := &df.MovieSegment{File: &df.File{Order: binary.LittleEndian}}
		expected := [][]byte{}
		pixels := make([]byte, 64*32)
		var shown []byte
		for i := 0; i < 200; i++ {
			data := []byte{32, 0, 64, 0}
			for y := 0; y < 32; y++ {
				if i == 0 || y == i%32 {
					data = append(data, 20, 6, 32, byte(i))
					for x := 0; x < 64; x++ {
						pixels[y*64+x] = byte(i)
					}
				} else {
					data = append(data, 40)
				}
			}
			seg.Frames = append(seg.Frames, df.MovieFrame{LocationFrame: len(seg.File.Containers)})
			seg.File.Containers = append(seg.File.Containers, df.Container{Data: data})
			next := slices.Clone(pixels)
			if transparent && shown != nil {
				for j, p := range next {
					if p == 0 || p == 255 {
						next[j] = shown[j]
					}
				}
			}
			expected = append(expected, next)
			shown = next
		}
		m := NewMovieFrames(seg)
		m.ImageBudget = 8192
		m.CheckpointBudget = 16384
		m.TransparentV1 = transparent
		if m.DecodedFrames != 0 {
			t.Fatal("movie decoded before demand")
		}
		retained, err := m.Get(0)
		if err != nil {
			t.Fatal(err)
		}
		visits := make([]int, 200)
		for i := range visits {
			visits[i] = i
		}
		visits = append(visits, 199, 100, 101, 32, 0, 1, 199, 198, 197, 42, 43)
		for _, i := range visits {
			f, err := m.Get(i)
			if err != nil {
				t.Fatal(err)
			}
			if f == nil || !bytes.Equal(f.Pixels, expected[i]) {
				t.Fatal("seek produced wrong pixels", transparent, i)
			}
			bound := 8192 + 16384 + 64*32*2
			if transparent {
				bound += 64 * 32
			}
			if m.RetainedBytes() > bound {
				t.Fatal("cache exceeded memory budget", m.RetainedBytes(), bound)
			}
		}
		if !bytes.Equal(retained.Pixels, expected[0]) {
			t.Fatal("decoding mutated retained image")
		}
		for _, i := range []int{-1, 200} {
			if f, err := m.Get(i); err != nil || f != nil {
				t.Fatal("invalid index accepted")
			}
		}
		n := m.DecodedFrames
		m.Get(43)
		if n != m.DecodedFrames {
			t.Fatal("stationary frame decoded again")
		}
	}
}
