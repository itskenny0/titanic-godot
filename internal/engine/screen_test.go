package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/script"
	"os"
	"reflect"
	"testing"
)

func TestScreenReference(t *testing.T) {
	b, err := os.ReadFile("../../tests/fixtures/screen.json")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Pixels  string
		Repaint []bool
		Fades   []struct {
			Level    float64
			Commands []DrawCommand
		}
		Wraps []struct {
			Text  string
			Lines []string
		}
	}
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	s := NewScreenPresenter(8, 6)
	ctx := NewDrawContext(8, 6)
	s.ClearFrame()
	src := make([]byte, 4*3*4)
	for i := range src {
		src[i] = byte(i * 5)
	}
	s.BlitAt(src, 4, 3, 2, 1)
	s.FrameValid = true
	h := sha256.Sum256(s.Frame)
	if hex.EncodeToString(h[:]) != want.Pixels {
		t.Fatal("screen blit differs")
	}
	sig := new(DrawSignature).Reset().Num(17)
	for i, expected := range want.Repaint {
		paint := s.ShouldPaint(sig)
		if paint != expected {
			t.Fatal("redraw decision differs", i)
		}
		if paint {
			s.Blit(ctx)
		}
	}
	overlay := []TextOverlay{{Text: "Test", X: 1, Y: 3, Color: 0, Size: 12}, {Text: "Café", X: 2, Y: 4, Color: 3, Size: 16}}
	for _, fade := range want.Fades {
		s.Blit(ctx)
		s.DrawTextOverlay(ctx, overlay)
		s.ApplyFadeExcept(ctx, fade.Level, ScreenRect{X: 2, Y: 1, W: 4, H: 3})
		if !reflect.DeepEqual(ctx.Commands, fade.Commands) {
			t.Errorf("overlay/fade differs at %g\ngot %v\nwant %v", fade.Level, ctx.Commands, fade.Commands)
		}
	}
	for _, wrap := range want.Wraps {
		got := WrapText(wrap.Text, 55, func(s string) float64 { return float64(len(script.UTF16Units(s))) * 7 })
		if !reflect.DeepEqual(got, wrap.Lines) {
			t.Error("word wrap differs", wrap.Text, got, wrap.Lines)
		}
	}
	snapshot := s.Capture()
	s.ClearFrame()
	if snapshot == nil || bytes.Equal(snapshot.RGBA, s.Frame) {
		t.Fatal("capture aliases the mutable framebuffer")
	}
}
