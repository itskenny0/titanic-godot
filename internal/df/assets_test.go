package df

import (
	"encoding/binary"
	"testing"
)

func assetFixture(t *testing.T, order Order, containers ...[]byte) []byte {
	t.Helper()
	f := File{Order: order, Containers: make([]Container, len(containers))}
	for i, c := range containers {
		f.Containers[i] = Container{ID: int32(i), Data: c}
	}
	b, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func fixtureName(b []byte, at int, s string) { b[at] = byte(len(s)); copy(b[at+1:], s) }
func TestMovieSegmentReferencesAndTiming(t *testing.T) {
	for _, order := range []Order{binary.LittleEndian, binary.BigEndian} {
		t.Run(order.String(), func(t *testing.T) {
			containers := [][]byte{}
			for segment := 0; segment < 2; segment++ {
				h := make([]byte, 0x87c+42)
				if order == binary.BigEndian {
					order.PutUint16(h, 4)
				} else {
					order.PutUint32(h[2:], 4)
				}
				put := func(at, v int) { order.PutUint32(h[at:], uint32(v)) }
				put(0x18, 9)
				put(0x1c, 2)
				if segment == 0 {
					put(0x2c, 6)
				}
				put(0x60, 1)
				put(0x64, 2)
				put(0x68, 3)
				put(0x878, 1)
				order.PutUint16(h[0x870:], 264)
				order.PutUint16(h[0x872:], 512)
				fixtureName(h, 0x40, "start")
				fixtureName(h, 0x50, "end")
				at := 0x87c
				order.PutUint32(h[at+12:], 4)
				order.PutUint32(h[at+16:], 5)
				fixtureName(h, at+26, "start")
				sound := make([]byte, 50)
				order.PutUint32(sound[4:], 1)
				order.PutUint32(sound[12:], 4)
				fixtureName(sound, 18, "voice")
				fixtureName(sound, 34, "end")
				loop := make([]byte, 296)
				order.PutUint16(loop[4:], 1)
				order.PutUint16(loop[6:], 1)
				order.PutUint32(loop[266:], 1)
				order.PutUint32(loop[274:], 4)
				fixtureName(loop, 280, "ambient")
				cues := make([]byte, 32)
				order.PutUint32(cues, 1)
				order.PutUint32(cues[4:], 30)
				fixtureName(cues, 16, "end")
				logic := make([]byte, 1158)
				order.PutUint16(logic, 6)
				order.PutUint32(logic[2:], 3)
				logic[6] = 13
				fixtureName(logic, 0x12, "voice")
				order.PutUint32(logic[1090:], 1)
				order.PutUint16(logic[1094:], 2)
				fixtureName(logic, 1094+48, "end")
				containers = append(containers, h, sound, loop, cues, []byte{0}, logic)
			}
			m, err := ReadMovie(assetFixture(t, order, containers...))
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Segments) != 2 {
				t.Fatalf("segments: %d", len(m.Segments))
			}
			for i, s := range m.Segments {
				want := i*6 + 4
				if s.Frames[0].LocationFrame != want || s.AudioChunks[0] != want || s.Sounds["voice"] != want {
					t.Fatalf("segment %d failed relative references: %+v", i, s)
				}
				f := s.Frames[0]
				if f.HoldTicks != 3 || !f.WaitsForVoice || !f.HoldsDeadline || !f.PlaysThroughRegions || s.MinHoldTicks != 2 || s.Flags != 9 || !s.KeySkips {
					t.Fatal("authored timing/skip flags were lost")
				}
				if s.SoundFollows["voice"] != "end" || s.Cues[0].Tick != 30 || s.Cues[0].Target != "end" || f.Regions[0].Target != "end" {
					t.Fatal("movie control flow was lost")
				}
			}
		})
	}
}
func TestSpriteRunsAndTruncation(t *testing.T) {
	// First row: transparent, two literals, three repeats. Second row copies it.
	data := []byte{2, 0, 6, 0, 255, 255, 4, 0, 6, 0, 5, 11, 7, 8, 14, 9, 1, 0, 24}
	s, err := DecodeSprite(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Width != 6 || s.Height != 2 || s.PosYraw != -1 || s.PosXraw != 4 {
		t.Fatalf("geometry: %+v", s)
	}
	if string(s.Indexed) != string([]byte{0, 7, 8, 9, 9, 9, 0, 7, 8, 9, 9, 9}) || string(s.Opaque) != string([]byte{0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1}) {
		t.Fatalf("pixels: %+v", s)
	}
	for n := 0; n < len(data); n++ {
		if _, err := DecodeSprite(data[:n]); err == nil {
			t.Fatalf("accepted truncated sprite at %d", n)
		}
	}
}
func TestMalformedAssetTablesAreBounded(t *testing.T) {
	c0 := make([]byte, 4096)
	binary.LittleEndian.PutUint32(c0[2:], 4)
	for _, at := range []int{0x64, 0x878, 2120, 2158, 2360} {
		binary.LittleEndian.PutUint32(c0[at:], 0x7fffffff)
	}
	bytes := assetFixture(t, binary.LittleEndian, c0)
	readers := []func([]byte) error{
		func(b []byte) error { _, e := ReadSet(b); return e }, func(b []byte) error { _, e := ReadStage(b); return e }, func(b []byte) error { _, e := ReadShop(b); return e }, func(b []byte) error { _, e := ReadCast(b); return e }, func(b []byte) error { _, e := ReadPuppet(b); return e }, func(b []byte) error { _, e := ReadMovie(b); return e },
	}
	for i, read := range readers {
		if err := read(bytes); err == nil {
			t.Fatalf("reader %d accepted impossible table", i)
		}
	}
}
