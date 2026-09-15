package engine

import (
	"encoding/binary"
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
)

const PuppetArtHeight = ScreenHeight - 5*24
const puppetSubtitleTop = PuppetArtHeight - 40

type PuppetBackdrop struct {
	Pixels, Palette []byte
	Width, Height   int
}
type puppetFrameKey struct {
	name string
	loc  int
}
type PuppetView struct {
	Session   *Session
	Gamma     *ScreenGamma
	frames    map[puppetFrameKey]*df.Sprite
	character struct {
		key string
		art *image.NRGBA
	}
	image struct {
		key                   string
		rgba, pixels, palette []byte
	}
	band struct {
		name  string
		frame *df.Sprite
	}
}

func NewPuppetView(s *Session, gamma *ScreenGamma) *PuppetView {
	if gamma == nil {
		gamma = NewScreenGamma()
	}
	return &PuppetView{Session: s, Gamma: gamma, frames: map[puppetFrameKey]*df.Sprite{}}
}
func (v *PuppetView) param(slot int, fallback float64) float64 {
	if value, ok := v.Session.PuppetParams[slot]; ok {
		return value
	}
	return fallback
}
func (v *PuppetView) LayerFrame(loc int) (*df.Sprite, error) {
	p := v.Session.PuppetCtrl.Puppet
	if p == nil {
		return nil, nil
	}
	key := puppetFrameKey{p.Name, loc}
	if f := v.frames[key]; f != nil {
		return f, nil
	}
	f, err := df.DecodeSprite(p.Pup.File.Data(loc))
	if err != nil {
		return nil, err
	}
	v.frames[key] = &f
	return &f, nil
}
func (v *PuppetView) BevelRects() []ScreenRect {
	rects := []ScreenRect{}
	if p := v.Session.PuppetCtrl.Puppet; p != nil {
		for i := 0; i < min(5, len(p.Bevels)); i++ {
			rects = append(rects, ScreenRect{0, float64(PuppetArtHeight + i*24), ScreenWidth, 24})
		}
	}
	return rects
}
func (v *PuppetView) BevelAt(x, y float64) int {
	for i, r := range v.BevelRects() {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return i
		}
	}
	return -1
}
func puppetSubtitleLines(text string, measure func(string) float64) []string {
	if measure(text) < 496 {
		return []string{text}
	}
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == ' ' && measure(text[:i]) < 496 {
			return []string{text[:i], text[i+1:]}
		}
	}
	lines := WrapText(text, 496, measure)
	return lines[:min(2, len(lines))]
}
func sameBytes(a, b []byte) bool { return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0]) }
func (v *PuppetView) Composite(dest []byte, backdrop *PuppetBackdrop) error {
	p := v.Session.PuppetCtrl.Puppet
	if p == nil {
		return nil
	}
	if len(dest) < ScreenWidth*ScreenHeight*4 {
		return fmt.Errorf("puppet: destination too small")
	}
	clipY := PuppetArtHeight
	if p.Subtitle != "" && v.Session.SubtitlesOn() {
		clipY = puppetSubtitleTop
	}
	state := v.Session.PuppetCtrl.Frame()
	var key strings.Builder
	fmt.Fprintf(&key, "%s:%d:%d:%d:", p.Name, p.StanceIdx, clipY, v.Gamma.Generation)
	if state == nil {
		key.WriteByte('-')
	} else {
		for i, l := range state.Layers {
			if i > 0 {
				key.WriteByte(',')
			}
			key.WriteString(strconv.Itoa(l.Frame))
			fmt.Fprintf(&key, "/%d/%d", l.X, l.Y)
		}
	}
	var pixels, palette []byte
	if backdrop != nil {
		pixels, palette = backdrop.Pixels, backdrop.Palette
	}
	if v.image.rgba == nil || v.image.key != key.String() || !sameBytes(v.image.pixels, pixels) || !sameBytes(v.image.palette, palette) {
		rgba := make([]byte, ScreenWidth*PuppetArtHeight*4)
		if backdrop != nil {
			if backdrop.Width < 0 || backdrop.Height < 0 || backdrop.Width > ScreenWidth || backdrop.Height > ScreenHeight {
				return fmt.Errorf("puppet: invalid backdrop dimensions")
			}
			view := make([]byte, backdrop.Width*backdrop.Height*4)
			if err := df.IndexedRGBA(pixels, palette, view); err != nil {
				return err
			}
			for y := 0; y < min(backdrop.Height, PuppetArtHeight); y++ {
				copy(rgba[y*ScreenWidth*4:], view[y*backdrop.Width*4:(y+1)*backdrop.Width*4])
			}
		}
		for i := clipY * ScreenWidth * 4; i < len(rgba); i += 4 {
			rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = 0, 0, 0, 255
		}
		pal := v.Gamma.DisplayPalette(df.PaletteRGBA(p.Pup.PaletteRaw, 256, binary.LittleEndian))
		if len(p.Pup.Stances) > 0 && state != nil {
			stanceIdx := p.StanceIdx
			if stanceIdx < 0 || stanceIdx >= len(p.Pup.Stances) {
				stanceIdx = 0
			}
			stance := p.Pup.Stances[stanceIdx]
			for l := 0; l < min(11, len(state.Layers), len(stance.Layers)); l++ {
				st, layer := state.Layers[l], stance.Layers[l]
				if st.Frame < 0 || len(layer.Frames) == 0 {
					continue
				}
				f, err := v.LayerFrame(layer.Frames[min(st.Frame, len(layer.Frames)-1)])
				if err != nil || f == nil {
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
				compositePuppetSprite(rgba, f, pal, st.X-f.PosXraw, st.Y-f.PosYraw, clipY)
			}
		}
		v.image.key, v.image.rgba, v.image.pixels, v.image.palette = key.String(), rgba, pixels, palette
	}
	copy(dest, v.image.rgba)
	v.compositeBand(dest)
	return nil
}
func compositePuppetSprite(dest []byte, f *df.Sprite, pal []byte, dx, dy, clipY int) {
	for yy := max(0, -dy); yy < min(f.Height, clipY-dy); yy++ {
		for xx := max(0, -dx); xx < min(f.Width, ScreenWidth-dx); xx++ {
			s := yy*f.Width + xx
			if f.Opaque[s] == 0 {
				continue
			}
			c, d := int(f.Indexed[s])*4, ((dy+yy)*ScreenWidth+dx+xx)*4
			copy(dest[d:d+3], pal[c:c+3])
			dest[d+3] = 255
		}
	}
}
func (v *PuppetView) compositeBand(dest []byte) {
	p := v.Session.PuppetCtrl.Puppet
	if v.band.frame == nil || v.band.name != p.Name {
		v.band.name, v.band.frame = "", nil
		loc := p.Pup.BandLocation
		data := p.Pup.File.Data(loc)
		if data == nil {
			v.Session.Log(fmt.Sprintf("puppet %s: no answer band at container %d", p.Name, loc))
			return
		}
		f, err := df.DecodeSprite(data)
		if err != nil {
			v.Session.Log(fmt.Sprintf("puppet %s: answer band: %v", p.Name, err))
			return
		}
		v.band.name, v.band.frame = p.Name, &f
	}
	f := v.band.frame
	pal := v.Gamma.DisplayPalette(df.PaletteRGBA(p.Pup.PaletteRaw, 256, binary.LittleEndian))
	compositePuppetSprite(dest, f, pal, ScreenWidth/2-f.PosXraw, ScreenHeight-60-f.PosYraw, ScreenHeight)
}
func (v *PuppetView) PressHeld(press *PuppetPress) bool {
	if v.Session.Executor.Now() < press.Until {
		return true
	}
	return v.Session.PointerDown && v.BevelAt(v.Session.PointerX, v.Session.PointerY) == press.Index
}
func (v *PuppetView) DrawSignature(sig *DrawSignature) {
	p := v.Session.PuppetCtrl.Puppet
	if p == nil {
		sig.Bool(false)
		return
	}
	sig.Bool(true).Bool(p.Visible).Str(p.Name).Num(float64(p.StanceIdx)).Bool(v.Session.SubtitlesOn()).Str(p.Subtitle)
	state := v.Session.PuppetCtrl.Frame()
	if state == nil {
		sig.Num(-1)
	} else {
		sig.Num(float64(len(state.Layers)))
		for _, l := range state.Layers {
			sig.Num(float64(l.Frame)).Num(float64(l.X)).Num(float64(l.Y))
		}
	}
	sig.Num(float64(len(p.Bevels)))
	for _, b := range p.Bevels {
		sig.Str(b.Text)
	}
	chosen, press := -1, -1
	held := false
	if p.Chosen != nil {
		chosen = *p.Chosen
	}
	if p.Press != nil {
		press = p.Press.Index
		held = v.PressHeld(p.Press)
	}
	sig.Num(float64(chosen)).Num(float64(press)).Bool(held)
}
func (v *PuppetView) clutColor(raw []byte, index float64) string {
	channel := func(offset, channel int) byte {
		at := float64(index*8) + float64(offset)
		var value float64
		if at >= 0 && at < float64(len(raw)) && at == float64(int(at)) {
			value = float64(raw[int(at)])
		}
		return v.Gamma.DisplayChannel(value, channel)
	}
	return fmt.Sprintf("rgb(%d, %d, %d)", channel(3, 0), channel(5, 1), channel(7, 2))
}
func (v *PuppetView) DrawOverlay(ctx *DrawContext) {
	p := v.Session.PuppetCtrl.Puppet
	if p == nil {
		return
	}
	pal := p.Pup.PaletteRaw
	ctx.Save()
	defer ctx.Restore()
	ctx.Font = SubtitleFont(v.param(6, 12))
	if p.Subtitle != "" && v.Session.SubtitlesOn() {
		ctx.Fill = "#000"
		ctx.FillRect(0, puppetSubtitleTop, ScreenWidth, 40)
		ctx.Fill = v.clutColor(pal, 0)
		for i, line := range puppetSubtitleLines(p.Subtitle, ctx.MeasureText) {
			ctx.FillText(line, 8, float64(puppetSubtitleTop+16+i*16))
		}
	}
	rects := v.BevelRects()
	ctx.Fill = v.clutColor(pal, v.param(3, 250))
	for i, r := range rects {
		ctx.FillText(p.Bevels[i].Text, r.X+v.param(10, 8), r.Y+16)
	}
	outline := func(i int) { r := rects[i]; ctx.LineWidth = 3; ctx.StrokeRect(r.X+1.5, r.Y+1.5, r.W-3, r.H-3) }
	if press := p.Press; press != nil && press.Index >= 0 && press.Index < len(rects) && v.PressHeld(press) {
		ctx.Stroke = "#fff"
		outline(press.Index)
	} else if p.Chosen != nil && *p.Chosen >= 0 && *p.Chosen < len(rects) {
		ctx.Stroke = v.clutColor(pal, v.param(4, 251))
		outline(*p.Chosen)
	}
}
