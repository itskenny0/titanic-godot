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
