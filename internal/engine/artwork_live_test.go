package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// svgTestBridge paints every redrawn control a solid colour, standing in for
// the frontend's SVG rasterizer.
type svgTestBridge struct{ testPlayerBridge }

func (b *svgTestBridge) RasterizeSVG(path string, svg []byte, w, h int) ([]byte, error) {
	out := make([]byte, w*h*4)
	for i := 0; i < len(out); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = 255, 0, 255, 255
	}
	return out, nil
}

// The redrawn interface can be switched on and off in the middle of a game:
// the next frame shows the new artwork and the side panels follow it.
func TestArtworkSwitchesDuringPlay(t *testing.T) {
	save := filepath.Join("..", "..", "originalgame", "gstair3 2026-09-12-23-24-22.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no gstair3 save in originalgame/")
	}
	read := func(path string) ([]byte, error) {
		if strings.HasPrefix(path, "res://") {
			path = filepath.Join("../../godot", strings.TrimPrefix(path, "res://"))
		}
		return os.ReadFile(path)
	}
	p := NewPlayer(&svgTestBridge{testPlayerBridge{read: read}})
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{Index: ownedFiles(t).Index, Save: save, HDSync: true}); err != nil {
		t.Fatal(err)
	}
	r := &achRun{t: t, p: p, ends: map[uint64]float64{}, seen: map[string]bool{}}
	r.play(300, nil)
	frame := func() []byte {
		p.Context.Version++
		out := p.Frame()
		return append([]byte(nil), out...)
	}
	// A toolbar pixel inside the life ring, in the frame's own scale.
	magenta := func(f []byte) int {
		scale := 1
		if len(f) == 1024*768*4 {
			scale = 2
		}
		n := 0
		for y := 300 * scale; y < 350*scale; y++ {
			for x := 200 * scale; x < 314*scale; x++ {
				i := (y*512*scale + x) * 4
				if f[i] == 255 && f[i+1] == 0 && f[i+2] == 255 {
					n++
				}
			}
		}
		return n
	}
	original := frame()
	if len(original) != 512*384*4 || magenta(original) != 0 {
		t.Fatalf("the game should start with the original artwork: %d bytes", len(original))
	}
	if l := p.AdaptiveLayout(); !l.Eligible || l.AtlasScale != 1 {
		t.Fatalf("original layout: %+v", l)
	}
	if err := p.Command(PlayerCommand{Action: "artwork", Value: map[string]any{"vector_ui": "res://artwork/ui"}}); err != nil {
		t.Fatal(err)
	}
	r.play(5, nil)
	redrawn := frame()
	if len(redrawn) != 1024*768*4 || magenta(redrawn) == 0 {
		t.Fatalf("the redrawn toolbar should show at once: %d bytes, %d redrawn pixels", len(redrawn), magenta(redrawn))
	}
	l := p.AdaptiveLayout()
	if !l.Eligible || l.AtlasScale != 2 || len(p.AdaptiveAtlas()) != l.AtlasSlots*128*128*4*4 {
		t.Fatalf("the side panels should follow the redrawn artwork: %+v", l)
	}
	// Switching off, also from the paused menu, restores the original.
	p.Command(PlayerCommand{Action: "pause", On: true})
	if err := p.Command(PlayerCommand{Action: "artwork", Value: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if p.Host.Screen().HD != nil || p.Host.Viewer != nil && p.Host.Viewer.Art != nil {
		t.Fatal("the redrawn artwork is still attached")
	}
	p.Command(PlayerCommand{Action: "pause"})
	r.play(5, nil)
	back := frame()
	if len(back) != 512*384*4 || magenta(back) != 0 {
		t.Fatalf("switching off should restore the original artwork: %d bytes", len(back))
	}
	if l := p.AdaptiveLayout(); !l.Eligible || l.AtlasScale != 1 || len(p.AdaptiveAtlas()) != l.AtlasSlots*128*128*4 {
		t.Fatalf("original layout after switching back: %+v", l)
	}
}

// Changing the brightness with F2/F3 keeps the redrawn artwork, in the
// classic frame and the side panels (issue #3: it reverted to the original).
func TestRedrawnArtworkSurvivesBrightness(t *testing.T) {
	save := filepath.Join("..", "..", "originalgame", "gstair3 2026-09-12-23-24-22.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no gstair3 save in originalgame/")
	}
	read := func(path string) ([]byte, error) {
		if strings.HasPrefix(path, "res://") {
			path = filepath.Join("../../godot", strings.TrimPrefix(path, "res://"))
		}
		return os.ReadFile(path)
	}
	p := NewPlayer(&svgTestBridge{testPlayerBridge{read: read}})
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{Index: ownedFiles(t).Index, Save: save, HDSync: true, VectorUI: "res://artwork/ui"}); err != nil {
		t.Fatal(err)
	}
	r := &achRun{t: t, p: p, ends: map[uint64]float64{}, seen: map[string]bool{}}
	r.play(300, nil)
	redrawn := func() (frame, atlas int) {
		p.Context.Version++
		f := p.Frame()
		for y := 600; y < 700; y++ {
			for x := 400; x < 628; x++ {
				i := (y*1024 + x) * 4
				if len(f) == 1024*768*4 && f[i] == 255 && f[i+1] == 0 && f[i+2] == 255 {
					frame++
				}
			}
		}
		a := p.AdaptiveAtlas()
		for i := 0; i+3 < len(a); i += 4 {
			if a[i] == 255 && a[i+1] == 0 && a[i+2] == 255 {
				atlas++
			}
		}
		return
	}
	before, beforeAtlas := redrawn()
	if before == 0 || beforeAtlas == 0 {
		t.Fatalf("the redrawn toolbar should show: frame %d, atlas %d", before, beforeAtlas)
	}
	for _, key := range []int{2, 2, 2, 1, 1, 1, 1, 1, 9} {
		if err := p.Command(PlayerCommand{Action: "gamma", Key: float64(key)}); err != nil {
			t.Fatal(err)
		}
		r.play(3, nil)
		frame, atlas := redrawn()
		if frame < before/2 || atlas < beforeAtlas/2 {
			t.Fatalf("after brightness key %d (gamma %.3f) the redrawn artwork was lost: frame %d of %d, atlas %d of %d", key, p.Host.Director.Gamma.Average(), frame, before, atlas, beforeAtlas)
		}
	}
}

// The brightness chosen with F1-F9 is reported to the frontend, which keeps
// it, and a later boot (loading a save, starting again) restores it.
func TestBrightnessLastsAcrossLoads(t *testing.T) {
	save := filepath.Join("..", "..", "originalgame", "gstair3 2026-09-12-23-24-22.ti")
	if _, err := os.Stat(save); err != nil {
		t.Skip("no gstair3 save in originalgame/")
	}
	boot := func(gamma []float64) *Player {
		p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
		t.Cleanup(p.Close)
		if err := p.Boot(PlayerConfig{Index: ownedFiles(t).Index, Save: save, Gamma: gamma}); err != nil {
			t.Fatal(err)
		}
		r := &achRun{t: t, p: p, ends: map[uint64]float64{}, seen: map[string]bool{}}
		r.play(100, nil)
		return p
	}
	p := boot(nil)
	for _, key := range []float64{2, 2, 4} {
		if err := p.Command(PlayerCommand{Action: "gamma", Key: key}); err != nil {
			t.Fatal(err)
		}
	}
	var got []float64
	for _, e := range p.Events() {
		if m, ok := e.(map[string]any); ok && m["type"] == "gamma" {
			got, _ = m["channels"].([]float64)
		}
	}
	want := p.Host.Director.Gamma.Channels
	if want == [3]float64{DefaultScreenGamma, DefaultScreenGamma, DefaultScreenGamma} {
		t.Fatal("the brightness keys changed nothing")
	}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("the frontend was told %v, the screen uses %v", got, want)
	}
	again := boot(got)
	if again.Host.Director.Gamma.Channels != want {
		t.Fatalf("booting with %v gave %v", got, again.Host.Director.Gamma.Channels)
	}
}
