package engine

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func TestPuppetArtworkMatchesOriginalComposite(t *testing.T) {
	v, p := puppetViewFixture(t)
	pal := v.Gamma.DisplayPalette(df.PaletteRGBA(p.Pup.PaletteRaw, 256, binary.LittleEndian))
	art, err := PuppetArtwork(p.Pup, p.StanceIdx, p.Pose, pal, v.LayerFrame)
	if err != nil || art == nil {
		t.Fatal(art, err)
	}
	if art.Rect.Min.X != 1 || art.Rect.Min.Y != 223 || art.Rect.Dx() != 2 || art.Rect.Dy() != 2 {
		t.Fatal("flat matte entered the portrait or placement changed", art.Rect)
	}
	logical := make([]byte, ScreenWidth*ScreenHeight*4)
	if err := v.Composite(logical, nil); err != nil {
		t.Fatal(err)
	}
	for y := art.Rect.Min.Y; y < art.Rect.Max.Y; y++ {
		start := (y*ScreenWidth + art.Rect.Min.X) * 4
		row := art.Pix[(y-art.Rect.Min.Y)*art.Stride : (y-art.Rect.Min.Y+1)*art.Stride]
		if !bytes.Equal(row, logical[start:start+len(row)]) {
			t.Fatal("exported face differs from the original compositor")
		}
	}
}

func TestPuppetHDLeavesStatePixelsAndSubtitlesUnchanged(t *testing.T) {
	v, p := puppetViewFixture(t)
	pal := v.Gamma.DisplayPalette(df.PaletteRGBA(p.Pup.PaletteRaw, 256, binary.LittleEndian))
	art, err := PuppetArtwork(p.Pup, p.StanceIdx, p.Pose, pal, v.LayerFrame)
	if err != nil {
		t.Fatal(err)
	}
	fixture := hdFixture(t, art.Pix, art.Rect.Dx(), art.Rect.Dy())
	fixture.Pack.Manifest.Characters = true
	hd := NewHDSurface(fixture.Pack, ScreenWidth, ScreenHeight)
	logical := make([]byte, ScreenWidth*ScreenHeight*4)
	if err := v.Composite(logical, nil); err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), logical...)
	v.CompositeHD(hd, logical)
	at := (223*2*ScreenWidth*2 + 1*2) * 4
	if !hd.Valid || hd.Hits != 1 || hd.Pixels[at] != 30 || hd.Pixels[at+4] != 31 {
		t.Fatal("complete character pose did not use HD detail")
	}
	if hd.Pixels[at+8] != 0 {
		t.Fatal("HD picture filled an originally transparent pixel")
	}
	if !bytes.Equal(original, logical) {
		t.Fatal("HD rendering changed save or gameplay pixels")
	}
	p.Subtitle = "Hello"
	if err := v.Composite(logical, nil); err != nil {
		t.Fatal(err)
	}
	v.CompositeHD(hd, logical)
	if hd.Pixels[(224*2*ScreenWidth*2+2)*4] != 0 {
		t.Fatal("HD character covered the subtitle band")
	}
	p.Subtitle = ""
	before := PuppetPoseKey(p.StanceIdx, p.Pose)
	p.Pose.Layers[1].X++
	if before == PuppetPoseKey(p.StanceIdx, p.Pose) {
		t.Fatal("pose key ignored authored layer movement")
	}
	if err := v.Composite(logical, nil); err != nil {
		t.Fatal(err)
	}
	v.CompositeHD(hd, logical)
	if hd.Pixels[at] != 0 || hd.Pixels[at+8] != 30 {
		t.Fatal("cached character did not follow its authored movement")
	}
	hd.Pack.Manifest.Characters = false
	v.CompositeHD(hd, logical)
	if hd.Valid {
		t.Fatal("a pack without characters should retain original dialogue rendering")
	}
}
