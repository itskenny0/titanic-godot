package engine

import (
	"math"
	"slices"
)

const DefaultScreenGamma = .65
const ScreenGammaStep = 1.05

var AllGammaChannels = [3]bool{true, true, true}

type ScreenGamma struct {
	Channels   [3]float64
	Generation uint64
	ramps      [3][256]byte
}

func NewScreenGamma() *ScreenGamma {
	g := new(ScreenGamma)
	for i := range g.Channels {
		g.Channels[i] = DefaultScreenGamma
		g.buildRamp(i)
	}
	return g
}
func (g *ScreenGamma) buildRamp(ch int) {
	for c := range g.ramps[ch] {
		g.ramps[ch][c] = byte(jsRound(float64(255 * math.Pow(float64(c)/255, g.Channels[ch]))))
	}
}
func (g *ScreenGamma) Average() float64 { return (g.Channels[0] + g.Channels[1] + g.Channels[2]) / 3 }
func (g *ScreenGamma) Set(value float64, channels [3]bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = DefaultScreenGamma
	}
	next := math.Max(.3, math.Min(1.6, value))
	moved := false
	for i, on := range channels {
		if !on || g.Channels[i] == next {
			continue
		}
		g.Channels[i] = next
		g.buildRamp(i)
		moved = true
	}
	if moved {
		g.Generation++
	}
}
func (g *ScreenGamma) Step(up bool, channels [3]bool) {
	moved := false
	for i, on := range channels {
		if !on {
			continue
		}
		raw := g.Channels[i] / ScreenGammaStep
		if up {
			raw = float64(g.Channels[i] * ScreenGammaStep)
		}
		next := math.Max(.3, math.Min(1.6, raw))
		if next == g.Channels[i] {
			continue
		}
		g.Channels[i] = next
		g.buildRamp(i)
		moved = true
	}
	if moved {
		g.Generation++
	}
}
func (g *ScreenGamma) Reset() { g.Set(DefaultScreenGamma, AllGammaChannels) }
func (g *ScreenGamma) DisplayPalette(clut []byte) []byte {
	out := slices.Clone(clut)
	for i, c := range clut {
		if ch := i % 4; ch < 3 {
			out[i] = g.ramps[ch][c]
		}
	}
	return out
}
func (g *ScreenGamma) DisplayChannel(value float64, channel int) byte {
	return g.ramps[channel][max(0, min(255, int(int32JS(value))))]
}
func clampedByte(value float64) byte {
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	if value >= 255 {
		return 255
	}
	return byte(math.RoundToEven(value))
}

// DimPalette uses Uint8ClampedArray's ties-to-even assignment, before gamma.
func DimPalette(base []byte, dim ClutDim) []byte {
	out := slices.Clone(base)
	factor := math.Max(0, math.Min(255, 255-dim.Amt)) / 255
	lo, hi := math.Max(0, dim.Lo), math.Min(float64(len(base))/4-1, dim.Hi)
	for i := lo; i <= hi; i++ {
		for ch := 0.; ch < 3; ch++ {
			at := float64(i*4) + ch
			if at == math.Trunc(at) && at >= 0 && at < float64(len(base)) {
				out[int(at)] = clampedByte(float64(float64(base[int(at)]) * factor))
			}
		}
	}
	return out
}
