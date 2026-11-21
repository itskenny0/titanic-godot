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
	"time"

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

func TestVectorSpriteAlphaIsDisplayOnly(t *testing.T) {
	pal := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	f := &df.Sprite{Width: 2, Height: 1, Indexed: []byte{0, 1}, Opaque: []byte{1, 0}}
	source := spriteRGBA(f, pal)
	hd := hdFixture(t, source, 2, 1)
	key := hdpack.Key(2, 1, source)
	entry := hd.Pack.Manifest.Images[key]
	entry.SpriteAlpha = true
	hd.Pack.Manifest.Version = hdpack.VectorVersion
	hd.Pack.Manifest.Images[key] = entry
	img := hd.Pack.Get(key, 2, 1)
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:i+4], []byte{200, 100, 50, 128})
	}
	img.Pix[3] = 0 // A genuine vector cutout inside the old opaque mask.
	hd.Clear()
	for i := 0; i < len(hd.Pixels); i += 4 {
		copy(hd.Pixels[i:i+4], []byte{10, 20, 30, 255})
	}
	logical := make([]byte, 16)
	r := screenSprite(f, 0, 0)
	compositeSprite(r, logical, 2, 2, pal, 2, 2, nil, 0, hd)
	if !bytes.Equal(hd.Pixels[:4], []byte{10, 20, 30, 255}) || !bytes.Equal(hd.Pixels[4:8], []byte{105, 60, 40, 255}) || hd.Pixels[8] != 105 {
		t.Fatal("smooth alpha did not blend over background or extend beyond pixel mask", hd.Pixels[:16])
	}
	if logical[0] != 1 || logical[4] != 0 || r.sample(1, 0) != -1 {
		t.Fatal("vector alpha changed logical pixels or click mask")
	}
	hd.Clear()
	occ := &Occlusion{W: 2, H: 2, Z: []byte{0, 0, 0, 0}}
	hd.Sprite(r, pal, 2, 2, occ, 1)
	if hd.Pixels[4] != 0 {
		t.Fatal("vector alpha bypassed occlusion")
	}
	hd.Clear()
	r.screen = false
	hd.Sprite(r, pal, 2, 2, nil, 0)
	if hd.Pixels[8] != 0 {
		t.Fatal("screen sprite alpha leaked into world sprite masks")
	}
}

// Repaints must not hash a sprite's pixels again, and a different palette
// must never reuse another palette's artwork.
func TestHDSpriteKeyIsCachedPerPalette(t *testing.T) {
	palA := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	palB := []byte{9, 9, 9, 255, 4, 5, 6, 255}
	f := &df.Sprite{Width: 2, Height: 1, Indexed: []byte{0, 1}, Opaque: []byte{1, 1}}
	hd := hdFixture(t, spriteRGBA(f, palA), 2, 1)
	if got := hd.spriteKey(f, palA); got != hdpack.Key(2, 1, spriteRGBA(f, palA)) {
		t.Fatal("cached key differs from the artwork key")
	}
	if len(hd.spriteKeys) != 1 || hd.spriteKey(f, palA) != hd.spriteKey(f, palA) || len(hd.spriteKeys) != 1 {
		t.Fatal("repeated draw was hashed again")
	}
	if hd.spriteKey(f, palB) == hd.spriteKey(f, palA) || hd.spriteKey(f, palB) != hdpack.Key(2, 1, spriteRGBA(f, palB)) {
		t.Fatal("palette change reused stale artwork key")
	}
	hd.Clear()
	hd.Sprite(screenSprite(f, 0, 0), palA, 2, 2, nil, 0)
	if hd.Hits != 1 || hd.Pixels[0] != 30 {
		t.Fatal("matching sprite artwork was not drawn")
	}
	hd.Clear()
	hd.Sprite(screenSprite(f, 0, 0), palB, 2, 2, nil, 0)
	if hd.Hits != 1 || hd.Pixels[0] != 9 {
		t.Fatal("unmatched palette did not fall back to original pixels")
	}
}

func serviceUntil(t *testing.T, hd *HDSurface) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !hd.Service(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("artwork never finished decoding")
		}
	}
}
func TestHDAsyncShowsOriginalUntilArtworkArrives(t *testing.T) {
	src := []byte{10, 20, 30, 255, 40, 50, 60, 255, 70, 80, 90, 255, 100, 110, 120, 255}
	screen := NewScreenPresenter(2, 2)
	screen.HD = hdFixture(t, src, 2, 2)
	screen.HD.Pack.Async = true
	sig := new(DrawSignature)
	screen.ShouldPaint(sig)
	screen.ClearFrame()
	screen.BlitTop(src, 2, 2)
	screen.presented, screen.FrameValid = true, true
	hd := screen.HD
	if hd.Hits != 0 || !bytes.Equal(hd.Pixels[:8], []byte{10, 20, 30, 255, 10, 20, 30, 255}) {
		t.Fatalf("original was not shown while decoding: %v", hd.Pixels[:8])
	}
	if screen.ShouldPaint(sig) {
		t.Fatal("unchanged screen repainted")
	}
	serviceUntil(t, hd)
	screen.Invalidate()
	if !screen.ShouldPaint(sig) {
		t.Fatal("arrived artwork did not force a repaint")
	}
	screen.ClearFrame()
	screen.BlitTop(src, 2, 2)
	if hd.Hits != 1 || !bytes.Equal(hd.Pixels[:4], []byte{30, 80, 120, 255}) {
		t.Fatalf("decoded artwork not shown: hits=%d %v", hd.Hits, hd.Pixels[:4])
	}
	if hd.hashed != 1 {
		t.Fatalf("unchanged picture hashed %d times", hd.hashed)
	}
}
func TestHDMovementUsesOnlyDecodedArtworkAndWarmsAhead(t *testing.T) {
	palette := make([]byte, 1024)
	copy(palette, []byte{10, 20, 30, 255, 40, 50, 60, 255})
	frame := &CachedFrame{Pixels: []byte{0, 1, 1, 0}, Width: 2, Height: 2, ID: newDrawID()}
	src := make([]byte, 16)
	if err := df.IndexedRGBA(frame.Pixels, palette, src); err != nil {
		t.Fatal(err)
	}
	hd := hdFixture(t, src, 2, 2)
	hd.Pack.Async = true
	hd.Moving()
	hd.Clear()
	hd.Blit(src, 2, 2, 0, 0)
	if hd.Hits != 0 || hd.Pack.Loading() != 0 || hd.late != 1 {
		t.Fatal("a frame already on screen started a decode", hd.Pack.Loading(), hd.late)
	}
	if !hd.WarmLanding(frame, palette) || hd.Pack.Loading() != 1 || hd.keep == "" {
		t.Fatal("landing picture was not requested")
	}
	if !hd.WarmMotion(frame, palette) || hd.hashed != 1 {
		t.Fatal("known frame was hashed again", hd.hashed)
	}
	serviceUntil(t, hd)
	hd.Moving()
	hd.Clear()
	hd.Blit(src, 2, 2, 0, 0)
	if hd.Hits != 1 || hd.shown != 1 {
		t.Fatal("warmed artwork not used during movement")
	}
	// Two movements that were mostly late turn movement artwork off.
	for i := 0; i < 2; i++ {
		hd.late, hd.shown = 3, 1
		hd.moving = 1
		hd.Service()
	}
	if !hd.motionOff || hd.WarmMotion(&CachedFrame{Pixels: []byte{1, 1, 1, 1}, Width: 2, Height: 2, ID: newDrawID()}, palette) {
		t.Fatal("slow movement artwork stayed enabled")
	}
	if !hd.WarmLanding(frame, palette) {
		t.Fatal("landing artwork must stay enabled")
	}
}

// HD room pictures keep their artwork after F2/F3 changed the brightness,
// drawn at the new brightness (issue #3).
func TestHDRoomArtworkSurvivesBrightness(t *testing.T) {
	gamma := NewScreenGamma()
	pixels := []byte{1, 2, 3, 4}
	clut := make([]byte, 256*4)
	for i := 1; i <= 4; i++ {
		clut[i*4], clut[i*4+1], clut[i*4+2], clut[i*4+3] = byte(40*i), byte(30*i), byte(20*i), 255
	}
	render := func(pal []byte) []byte {
		out := make([]byte, 16)
		if err := df.IndexedRGBA(pixels, pal, out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	screen := NewScreenPresenter(2, 2)
	screen.HD = hdFixture(t, render(gamma.DisplayPalette(clut)), 2, 2)
	screen.HD.Gamma = gamma
	draw := func() {
		screen.ClearFrame()
		screen.HD.Clear()
		pal := gamma.DisplayPalette(clut)
		screen.BlitIndexed(render(pal), pixels, pal, 2, 2, 0, 0)
	}
	draw()
	if screen.HD.Pixels[0] != 30 || screen.HD.Pixels[1] != 80 {
		t.Fatalf("the HD room should show at the default brightness: %v", screen.HD.Pixels[:4])
	}
	for _, up := range []bool{true, true, false, false, false} {
		gamma.Step(up, AllGammaChannels)
		draw()
		ramp := gamma.ArtworkRamp()
		if ramp == nil {
			continue
		}
		if got, want := screen.HD.Pixels[:3], []byte{ramp[0][30], ramp[1][80], ramp[2][120]}; !bytes.Equal(got, want) {
			t.Fatalf("at gamma %.3f the HD room artwork was lost or not brightened: got %v, want %v", gamma.Average(), got, want)
		}
		if !bytes.Equal(screen.Frame, render(gamma.DisplayPalette(clut))) {
			t.Fatal("the logical picture must stay the original at the current brightness")
		}
	}
}
