package engine

import (
	"bytes"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func puppetViewFixture(t *testing.T) (*PuppetView, *PuppetState) {
	t.Helper()
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
	t.Cleanup(s.Close)
	raw := make([]byte, 2048)
	for i := 0; i < 256; i++ {
		raw[i*8+3], raw[i*8+5], raw[i*8+7] = byte(i), byte(i), byte(i)
	}
	p := &PuppetState{Name: "test.pup", Visible: true, Pup: &df.Puppet{PaletteRaw: raw, Stances: []df.PuppetStance{{Layers: []df.PuppetLayer{{Frames: []int{1}}, {Frames: []int{2}}}}}}, Pose: &df.PuppetAnimFrame{Layers: []df.PuppetAnimLayer{{Frame: 0}, {Frame: 0, X: 1, Y: 223}}}}
	s.PuppetCtrl.Puppet = p
	v := NewPuppetView(s, nil)
	v.frames[puppetFrameKey{p.Name, 1}] = &df.Sprite{Width: 2, Height: 1, Indexed: []byte{7, 7}, Opaque: []byte{1, 1}}
	v.frames[puppetFrameKey{p.Name, 2}] = &df.Sprite{Width: 2, Height: 2, Indexed: []byte{80, 90, 100, 110}, Opaque: []byte{1, 0, 1, 1}}
	v.band.name = p.Name
	v.band.frame = &df.Sprite{Width: 1, Height: 1, Indexed: []byte{120}, Opaque: []byte{1}}
	return v, p
}
func TestPuppetCompositeMatteSubtitleAndGamma(t *testing.T) {
	v, p := puppetViewFixture(t)
	palette := make([]byte, 1024)
	copy(palette, []byte{0, 0, 0, 255, 20, 30, 40, 255})
	backdrop := &PuppetBackdrop{Pixels: []byte{1, 1}, Palette: palette, Width: 2, Height: 1}
	dest := make([]byte, 512*384*4)
	if err := v.Composite(dest, backdrop); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dest[:8], []byte{20, 30, 40, 255, 20, 30, 40, 255}) {
		t.Fatal("flat character matte covered the room")
	}
	at := (223*512 + 1) * 4
	if dest[at] != v.Gamma.DisplayChannel(80, 0) || dest[at+3] != 255 || dest[at+7] != 0 {
		t.Fatal("character transparency or placement differs")
	}
	if dest[(324*512+256)*4] != v.Gamma.DisplayChannel(120, 0) {
		t.Fatal("answer band anchor differs")
	}
	p.Subtitle = "Hello"
	if err := v.Composite(dest, backdrop); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dest[(224*512+1)*4:(224*512+1)*4+4], []byte{0, 0, 0, 255}) {
		t.Fatal("character painted over subtitle strip")
	}
	v.Session.PuppetParams[7] = 0
	if err := v.Composite(dest, backdrop); err != nil {
		t.Fatal(err)
	}
	if dest[(224*512+1)*4] != v.Gamma.DisplayChannel(100, 0) {
		t.Fatal("hiding subtitles did not restore character pixels")
	}
	v.Gamma.Set(1, AllGammaChannels)
	if err := v.Composite(dest, backdrop); err != nil {
		t.Fatal(err)
	}
	if dest[at] != 80 {
		t.Fatal("gamma change reused stale character image")
	}
}
func TestPuppetBevelBoundsAndPress(t *testing.T) {
	v, p := puppetViewFixture(t)
	p.Bevels = make([]Bevel, 7)
	if len(v.BevelRects()) != 5 {
		t.Fatal("more than five answers entered visible band")
	}
	for _, point := range []struct {
		x, y float64
		want int
	}{{0, 264, 0}, {511.99, 287.99, 0}, {0, 288, 1}, {512, 264, -1}, {0, 263.99, -1}, {0, 384, -1}} {
		if got := v.BevelAt(point.x, point.y); got != point.want {
			t.Fatal("answer boundary differs", point, got)
		}
	}
	press := &PuppetPress{Index: 1, Until: 100}
	if !v.PressHeld(press) {
		t.Fatal("minimum press feedback missing")
	}
	v.Session.Executor.AdvanceClock(100)
	if v.PressHeld(press) {
		t.Fatal("released press stayed highlighted")
	}
	v.Session.PointerDown = true
	v.Session.SetPointer(10, 290)
	if !v.PressHeld(press) {
		t.Fatal("pointer hold did not retain answer feedback")
	}
	v.Session.SetPointer(10, 320)
	if v.PressHeld(press) {
		t.Fatal("dragging off answer retained feedback")
	}
}
