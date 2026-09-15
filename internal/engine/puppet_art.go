package engine

import (
	"encoding/binary"
	"fmt"
	"image"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
)

// PuppetPoseKey includes placement as well as frame numbers: authored speech
// may move a layer without selecting a different sprite.
func PuppetPoseKey(stance int, state *df.PuppetAnimFrame) string {
	var key strings.Builder
	fmt.Fprintf(&key, "%d:", stance)
	if state != nil {
		for _, layer := range state.Layers {
			fmt.Fprintf(&key, "%d,%d,%d;", layer.Frame, layer.X, layer.Y)
		}
	}
	return key.String()
}

// PuppetArtwork joins the authored face, eye and mouth layers before upscaling.
// Keeping a complete pose avoids independently enhanced facial pieces and seams.
// The returned bounds retain the character's original screen placement.
func PuppetArtwork(pup *df.Puppet, stanceIndex int, state *df.PuppetAnimFrame, palette []byte, frame func(int) (*df.Sprite, error)) (*image.NRGBA, error) {
	if state == nil || len(pup.Stances) == 0 {
		return nil, nil
	}
	if stanceIndex < 0 || stanceIndex >= len(pup.Stances) {
		stanceIndex = 0
	}
	stance := pup.Stances[stanceIndex]
	rgba := make([]byte, ScreenWidth*PuppetArtHeight*4)
	for l := 0; l < min(11, len(state.Layers), len(stance.Layers)); l++ {
		st, layer := state.Layers[l], stance.Layers[l]
		if st.Frame < 0 || len(layer.Frames) == 0 {
			continue
		}
		f, err := frame(layer.Frames[min(st.Frame, len(layer.Frames)-1)])
		if err != nil {
			return nil, err
		}
		if f == nil || f.Width <= 0 || f.Height <= 0 {
			continue
		}
		if l == 0 {
			flat := true
			for i := 1; i < f.Width*f.Height; i++ {
				if f.Opaque[i] != 0 && f.Indexed[i] != f.Indexed[0] {
					flat = false
					break
				}
			}
			if flat {
				continue
			}
		}
		compositePuppetSprite(rgba, f, palette, st.X-f.PosXraw, st.Y-f.PosYraw, PuppetArtHeight)
	}
	x0, y0, x1, y1 := ScreenWidth, PuppetArtHeight, 0, 0
	for y := 0; y < PuppetArtHeight; y++ {
		for x := 0; x < ScreenWidth; x++ {
			if rgba[(y*ScreenWidth+x)*4+3] != 0 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x1 <= x0 {
		return nil, nil
	}
	out := image.NewNRGBA(image.Rect(x0, y0, x1, y1))
	for y := y0; y < y1; y++ {
		copy(out.Pix[(y-y0)*out.Stride:], rgba[(y*ScreenWidth+x0)*4:(y*ScreenWidth+x1)*4])
	}
	return out, nil
}

// CompositeHD changes only the displayed picture. The original compositor
// still supplies saves, hit testing, subtitle clipping and dialogue state.
func (v *PuppetView) CompositeHD(hd *HDSurface, logical []byte) {
	if hd == nil {
		return
	}
	if !hd.Pack.Manifest.Characters {
		hd.Valid = false
		return
	}
	hd.Clear()
	hd.Blit(logical, ScreenWidth, ScreenHeight, 0, 0)
	p := v.Session.PuppetCtrl.Puppet
	if p == nil {
		return
	}
	pose := v.Session.PuppetCtrl.Frame()
	key := fmt.Sprintf("%s:%d:%s", p.Name, v.Gamma.Generation, PuppetPoseKey(p.StanceIdx, pose))
	if key != v.character.key {
		pal := v.Gamma.DisplayPalette(df.PaletteRGBA(p.Pup.PaletteRaw, 256, binary.LittleEndian))
		art, err := PuppetArtwork(p.Pup, p.StanceIdx, pose, pal, v.LayerFrame)
		if err != nil {
			v.Session.Log(fmt.Sprintf("HD character %s: %v", p.Name, err))
		}
		v.character.key, v.character.art = key, art
	}
	if art := v.character.art; art != nil {
		clipY := PuppetArtHeight
		if p.Subtitle != "" && v.Session.SubtitlesOn() {
			clipY = puppetSubtitleTop
		}
		hd.MaskedArtwork(art, clipY)
	}
}
