package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"strconv"
	"strings"
)

type spriteCache struct {
	file   *df.File
	frames map[int]*df.Sprite
}

func (c *spriteCache) Frame(loc int) (*df.Sprite, error) {
	if f := c.frames[loc]; f != nil {
		return f, nil
	}
	if c.file == nil || loc < 0 || loc >= len(c.file.Containers) {
		return nil, fmt.Errorf("sprite container %d is unavailable", loc)
	}
	f, err := df.DecodeSprite(c.file.Data(loc))
	if err != nil {
		return nil, err
	}
	if c.frames == nil {
		c.frames = map[int]*df.Sprite{}
	}
	c.frames[loc] = &f
	return &f, nil
}

// Host object properties use Number(), whereas script arithmetic uses parseInt.
func propertyNumber(v script.Value) float64 {
	if !v.IsString {
		return v.Number
	}
	s := strings.TrimSpace(v.Text)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'b', 'B':
			base = 2
		case 'o', 'O':
			base = 8
		}
		if base != 0 {
			n, err := strconv.ParseUint(s[2:], base, 64)
			if err == nil {
				return float64(n)
			}
			return math.NaN()
		}
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return n
}
func propertyNumberOrZero(v script.Value) float64 {
	n := propertyNumber(v)
	if math.IsNaN(n) {
		return 0
	}
	return n
}

type SpriteRect struct {
	F       *df.Sprite
	K, X, Y float64
	W, H    int
	screen  bool
}

func screenSprite(f *df.Sprite, x, y float64) *SpriteRect {
	if f == nil {
		return nil
	}
	return &SpriteRect{f, 1, x - float64(f.PosXraw), y - float64(f.PosYraw), f.Width, f.Height, true}
}
func worldSprite(f *df.Sprite, proj Projection, k float64) *SpriteRect {
	if f == nil {
		return nil
	}
	return &SpriteRect{f, k, proj.X - jsRound(float64(f.PosXraw)*k), proj.Y - jsRound(float64(f.PosYraw)*k), max(1, int(jsRound(float64(f.Width)*k))), max(1, int(jsRound(float64(f.Height)*k))), false}
}
func (r *SpriteRect) intersects(w, h int) bool {
	return r != nil && r.X < float64(w) && r.Y < float64(h) && r.X+float64(r.W) > 0 && r.Y+float64(r.H) > 0
}
func (r *SpriteRect) sample(x, y float64) int {
	if r == nil || r.K <= 0 || x < r.X || y < r.Y || x >= r.X+float64(r.W) || y >= r.Y+float64(r.H) {
		return -1
	}
	if r.screen {
		index := (y-r.Y)*float64(r.F.Width) + (x - r.X)
		if index != math.Trunc(index) || index < 0 || index >= float64(len(r.F.Opaque)) || r.F.Opaque[int(index)] == 0 {
			return -1
		}
		return int(index)
	}
	sx := min(r.F.Width-1, int(math.Floor((x-r.X)/r.K)))
	sy := min(r.F.Height-1, int(math.Floor((y-r.Y)/r.K)))
	i := sy*r.F.Width + sx
	if sx < 0 || sy < 0 || i < 0 || i >= len(r.F.Opaque) || r.F.Opaque[i] == 0 {
		return -1
	}
	return i
}

// RGB writes intentionally preserve the destination alpha, as the original does.
func compositeSprite(r *SpriteRect, rgba []byte, w, h int, palette []byte, clipW, clipH int, occ *Occlusion, level float64) {
	if r == nil || r.K <= 0 || w <= 0 || h <= 0 || len(rgba)/4/w < h {
		return
	}
	maxX, maxY := min(w, clipW, int(math.Ceil(r.X+float64(r.W)))), min(h, clipH, int(math.Ceil(r.Y+float64(r.H))))
	// Normal game coordinates use the integer loop. Preserve flat-buffer indexing
	// for scripts that explicitly place sprites at fractional coordinates.
	if r.X != math.Trunc(r.X) || r.Y != math.Trunc(r.Y) {
		compositeFractionalSprite(r, rgba, w, h, palette, clipW, clipH, occ, level)
		return
	}
	for y := max(0, int(r.Y)); y < maxY; y++ {
		for x := max(0, int(r.X)); x < maxX; x++ {
			i := r.sample(float64(x), float64(y))
			if i < 0 || SceneryOccludes(occ, x, y, level) {
				continue
			}
			pal := int(r.F.Indexed[i]) * 4
			dst := (y*w + x) * 4
			if pal+2 < len(palette) {
				copy(rgba[dst:dst+3], palette[pal:pal+3])
			}
		}
	}
}

func compositeFractionalSprite(r *SpriteRect, rgba []byte, w, h int, palette []byte, clipW, clipH int, occ *Occlusion, level float64) {
	paint := func(x, y float64, i int) {
		if i < 0 || i >= len(r.F.Opaque) || r.F.Opaque[i] == 0 {
			return
		}
		if occ != nil && x >= 0 && y >= 0 && x < float64(occ.W) && y < float64(occ.H) {
			n := y*float64(occ.W) + x
			if n == math.Trunc(n) && n >= 0 && n < float64(len(occ.Z)) && float64(occ.Z[int(n)]) < level {
				return
			}
		}
		d := (y*float64(w) + x) * 4
		if d != math.Trunc(d) || d < 0 || d >= float64(len(rgba)) {
			return
		}
		pal := int(r.F.Indexed[i]) * 4
		for c := 0; c < 3; c++ {
			if int(d)+c < len(rgba) && pal+c < len(palette) {
				rgba[int(d)+c] = palette[pal+c]
			}
		}
	}
	if r.screen {
		for sy := 0; sy < r.F.Height; sy++ {
			y := r.Y + float64(sy)
			if y < 0 || y >= float64(h) {
				continue
			}
			for sx := 0; sx < r.F.Width; sx++ {
				x := r.X + float64(sx)
				if x < 0 || x >= float64(w) {
					continue
				}
				paint(x, y, sy*r.F.Width+sx)
			}
		}
		return
	}
	maxY := math.Min(float64(min(h, clipH)), r.Y+float64(r.H))
	maxX := math.Min(float64(min(w, clipW)), r.X+float64(r.W))
	for y := math.Max(0, r.Y); y < maxY; y++ {
		for x := math.Max(0, r.X); x < maxX; x++ {
			paint(x, y, r.sample(x, y))
		}
	}
}
