package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"math/big"
	"strings"
)

// DrawCommand is the existing Godot overlay protocol. Raster pixels stay in a
// separate buffer; text is measured and rendered with the host's bundled fonts.
type DrawCommand struct {
	Op    string  `json:"op"`
	Text  string  `json:"text,omitempty"`
	Font  string  `json:"font,omitempty"`
	Color string  `json:"color"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Line  float64 `json:"line,omitempty"`
}
type drawingStyle struct {
	Font, Fill, Stroke string
	LineWidth          float64
}
type DrawContext struct {
	Width, Height int
	Version       uint64
	Commands      []DrawCommand
	drawingStyle
	stack   []drawingStyle
	Measure func(string, string) float64
}

func NewDrawContext(w, h int) *DrawContext {
	return &DrawContext{Width: w, Height: h, drawingStyle: drawingStyle{Font: "12px Arial", Fill: "#fff", Stroke: "#fff", LineWidth: 1}}
}
func (c *DrawContext) Save() { c.stack = append(c.stack, c.drawingStyle) }
func (c *DrawContext) Restore() {
	if len(c.stack) == 0 {
		panic("unbalanced drawing state")
	}
	c.drawingStyle = c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
}
func (c *DrawContext) FillText(text string, x, y float64) {
	c.Commands = append(c.Commands, DrawCommand{Op: "text", Text: text, X: x, Y: y, Font: c.Font, Color: c.Fill})
}
func (c *DrawContext) FillRect(x, y, w, h float64) {
	c.Commands = append(c.Commands, DrawCommand{Op: "rect", X: x, Y: y, W: w, H: h, Color: c.Fill})
}
func (c *DrawContext) StrokeRect(x, y, w, h float64) {
	c.Commands = append(c.Commands, DrawCommand{Op: "stroke", X: x, Y: y, W: w, H: h, Color: c.Stroke, Line: c.LineWidth})
}
func (c *DrawContext) MeasureText(text string) float64 {
	if c.Measure != nil {
		return c.Measure(text, c.Font)
	}
	size := script.Str(strings.SplitN(c.Font, "px", 2)[0]).Num()
	return float64(float64(float64(len(script.UTF16Units(text)))*size) * .6)
}

const cjkFallback = `"MS PGothic", "Hiragino Kaku Gothic ProN", "Yu Gothic", "Noto Sans CJK JP", "Noto Sans JP"`

func SubtitleFont(size float64) string {
	return script.Num(size).String() + "px Arial, Helvetica, " + cjkFallback + ", sans-serif"
}
func OverlayFont(size float64) string {
	return script.Num(size).String() + "px \"Courier New\", " + cjkFallback + ", monospace"
}
func breaksAnywhere(r rune) bool {
	return r >= 0x3000 && r <= 0x30ff || r >= 0x3400 && r <= 0x4dbf || r >= 0x4e00 && r <= 0x9fff || r >= 0xf900 && r <= 0xfaff || r >= 0xff00 && r <= 0xffef
}
func WrapText(text string, maxWidth float64, measure func(string) float64) []string {
	pieces := []string{}
	var word strings.Builder
	for _, ch := range text {
		if ch == ' ' || breaksAnywhere(ch) {
			if word.Len() > 0 {
				pieces = append(pieces, word.String())
				word.Reset()
			}
			pieces = append(pieces, string(ch))
		} else {
			word.WriteRune(ch)
		}
	}
	if word.Len() > 0 {
		pieces = append(pieces, word.String())
	}
	lines := []string{""}
	for _, piece := range pieces {
		cur := lines[len(lines)-1]
		if piece == " " && cur == "" {
			continue
		}
		grown := cur + piece
		never := strings.ContainsAny(piece, "、。，．・？！ー」』）｝〕】〉》”’ぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮヵヶ")
		if cur != "" && measure(grown) > maxWidth && !never {
			if piece == " " {
				piece = ""
			}
			lines = append(lines, piece)
		} else {
			lines[len(lines)-1] = grown
		}
	}
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

// Fade's CSS alpha uses Number.toFixed(3), including exact ties rounded up.
// Convert the exact binary float to a rational so decimal boundary cases match.
func fadeColor(level float64) string {
	value := math.Min(1, level)
	if math.IsNaN(value) {
		return "rgba(0,0,0,NaN)"
	}
	r := new(big.Rat).SetFloat64(value)
	if r == nil {
		return "rgba(0,0,0,0.000)"
	}
	r.Mul(r, big.NewRat(1000, 1))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if new(big.Int).Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	n := q.Int64()
	return fmt.Sprintf("rgba(0,0,0,%d.%03d)", n/1000, n%1000)
}
