package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/hdpack"
)

func hdFixture(t *testing.T, source []byte, w, h int) *HDSurface {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w*2, h*2))
	for y := 0; y < h*2; y++ {
		for x := 0; x < w*2; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(30 + x), uint8(80 + y), 120, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	key := hdpack.Key(w, h, source)
	m := hdpack.Manifest{Version: 1, Scale: 2, Model: "synthetic", Images: map[string]hdpack.Entry{key: {Width: w * 2, Height: h * 2, SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))}}}
	manifest, _ := json.Marshal(m)
	pack, err := hdpack.Open("test", func(path string) ([]byte, error) {
		if path == "test/manifest.json" {
			return manifest, nil
		}
		return b.Bytes(), nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewHDSurface(pack, 2, 2)
}
func TestHDBackgroundDoesNotChangeLogicalScreen(t *testing.T) {
	src := []byte{10, 20, 30, 255, 40, 50, 60, 255, 70, 80, 90, 255, 100, 110, 120, 255}
	screen := NewScreenPresenter(2, 2)
	screen.HD = hdFixture(t, src, 2, 2)
	screen.ClearFrame()
	screen.BlitTop(src, 2, 2)
	if !bytes.Equal(screen.Frame, src) {
		t.Fatal("HD changed game-state pixels")
	}
	if screen.HD.Pixels[0] != 30 || screen.HD.Pixels[4] != 31 {
		t.Fatal("subpixel HD detail was lost")
	}
	screen.FrameValid = true
	if !bytes.Equal(screen.Capture().RGBA, src) {
		t.Fatal("save/photo capture changed size or pixels")
	}
}
func TestHDSpritePreservesMaskAndOcclusion(t *testing.T) {
	pal := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	f := &df.Sprite{Width: 2, Height: 1, Indexed: []byte{0, 1}, Opaque: []byte{1, 0}}
	source := []byte{1, 2, 3, 255, 0, 0, 0, 0}
	hd := hdFixture(t, source, 2, 1)
	hd.Clear()
	logical := make([]byte, 2*2*4)
	r := screenSprite(f, 0, 0)
	compositeSprite(r, logical, 2, 2, pal, 2, 2, nil, 0, hd)
	if hd.Pixels[0] != 30 || hd.Pixels[4] != 31 || hd.Pixels[8] != 0 {
		t.Fatal("sprite mask or high-resolution detail lost")
	}
	if logical[0] != 1 || logical[4] != 0 {
		t.Fatal("logical sprite changed")
	}
	hd.Clear()
	occ := &Occlusion{W: 2, H: 2, Z: []byte{0, 0, 0, 0}}
	compositeSprite(r, logical, 2, 2, pal, 2, 2, occ, 1, hd)
	if hd.Pixels[0] != 0 {
		t.Fatal("HD sprite ignored original occlusion")
	}
	hd.Clear()
	r.X = 0.5
	compositeSprite(r, logical, 2, 2, pal, 2, 2, nil, 0, hd)
	if hd.Valid {
		t.Fatal("fractional sprite must preserve original rendering")
	}
}

func TestHDTurnWipeUsesLogicalSnapshot(t *testing.T) {
	src := []byte{1, 2, 3, 255, 4, 5, 6, 255, 7, 8, 9, 255, 10, 11, 12, 255}
	screen := NewScreenPresenter(2, 2)
	screen.HD = hdFixture(t, src, 2, 2)
	screen.ClearFrame()
	screen.BlitTop(src, 2, 2)
	snapshot := &RGBAFrame{RGBA: src, Width: 2, Height: 2}
	d := &ScreenDirector{Screen: screen, Session: &Session{Wipe: WipeState{Span: 1, Steps: 2, Step: 1, From: snapshot, To: snapshot}}}
	d.pushTurn("turnright")
	if screen.HD.Valid {
		t.Fatal("wipe would present a stale HD image")
	}
}
