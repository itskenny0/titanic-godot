package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestGammaReference(t *testing.T) {
	b, err := os.ReadFile("../../tests/fixtures/gamma.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Op         string
		Value      float64
		Channels   [3]bool
		Dim        ClutDim
		Gammas     [3]float64
		Generation uint64
		Hash       string
	}
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	palette := make([]byte, 1024)
	for i := range palette {
		palette[i] = byte(i*17 + 19)
	}
	g := NewScreenGamma()
	for i, want := range cases {
		switch want.Op {
		case "reset":
			g.Reset()
		case "set":
			g.Set(want.Value, want.Channels)
		default:
			g.Step(want.Op == "up", want.Channels)
		}
		got := g.DisplayPalette(DimPalette(palette, want.Dim))
		h := sha256.Sum256(got)
		if g.Channels != want.Gammas || g.Generation != want.Generation || hex.EncodeToString(h[:]) != want.Hash {
			t.Fatal("gamma channels, palette or generation differ at", i, g.Channels, want.Gammas)
		}
	}
	g.Set(math.NaN(), AllGammaChannels)
	if g.Channels != [3]float64{.65, .65, .65} {
		t.Fatal("invalid gamma did not restore default")
	}
	generation := g.Generation
	g.Reset()
	if g.Generation != generation {
		t.Fatal("unchanged gamma invalidated cached frames")
	}
}
