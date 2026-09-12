package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/script"
)

func TestPuppetViewReference(t *testing.T) {
	path := os.Getenv("TAOOT_PUPPET_VIEW_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_PUPPET_VIEW_REFERENCE to local character compositor reference")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path  string
		Cases []struct {
			Ident    string
			Variant  int
			Pixels   string
			Commands []DrawCommand
			Lo, Hi   uint32
		}
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	pixels, palette := make([]byte, 512*264), make([]byte, 1024)
	for i := range pixels {
		pixels[i] = byte(i)
	}
	for i := range palette {
		palette[i] = byte(i * 17)
		if i%4 == 3 {
			palette[i] = 255
		}
	}
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			s := NewSession(func(string) ([]byte, error) { return data, nil }, new(HostAudio))
			defer s.Close()
			if !s.PuppetCtrl.OpenPuppetFile("test.pup") {
				t.Fatal("open puppet failed")
			}
			p := s.PuppetCtrl.Puppet
			v := NewPuppetView(s, nil)
			for i := 0; i < 6; i++ {
				p.Bevels = append(p.Bevels, Bevel{Text: "Answer " + string(rune('1'+i)), ID: float64(i)})
			}
			for _, c := range entry.Cases {
				s.PuppetCtrl.Base(c.Ident)
				if c.Ident == "default" {
					s.PuppetCtrl.Base("")
				}
				gamma := .65
				if c.Variant == 5 {
					gamma = 1.2
				}
				v.Gamma.Set(gamma, AllGammaChannels)
				p.Subtitle = "A long subtitle that must wrap across two lines while the character speaks to the player."
				if c.Variant == 0 {
					p.Subtitle = ""
				}
				if c.Variant == 6 {
					p.Subtitle = strings.Repeat("日本語の字幕、長い文章でも画面内に表示されるように改行する。", 3)
				}
				s.PuppetParams[7] = 1
				if c.Variant == 2 {
					s.PuppetParams[7] = 0
				}
				p.Chosen, p.Press = nil, nil
				if c.Variant == 3 {
					chosen := 2
					p.Chosen = &chosen
				}
				if c.Variant == 4 {
					p.Press = &PuppetPress{Index: 1, Until: 100}
				}
				backdrop := &PuppetBackdrop{Pixels: pixels, Palette: palette, Width: 512, Height: 264}
				if c.Variant == 1 || c.Variant == 5 {
					backdrop = nil
				}
				rgba := make([]byte, 512*384*4)
				for i := range rgba {
					rgba[i] = 37
				}
				if err := v.Composite(rgba, backdrop); err != nil {
					t.Fatal(err)
				}
				h := sha256.Sum256(rgba)
				if hex.EncodeToString(h[:]) != c.Pixels {
					t.Fatalf("%s variant %d pixels differ: got %x want %s", c.Ident, c.Variant, h, c.Pixels)
				}
				ctx := NewDrawContext(512, 384)
				ctx.Measure = func(text, font string) float64 { return float64(len(script.UTF16Units(text)) * 7) }
				v.DrawOverlay(ctx)
				if !reflect.DeepEqual(ctx.Commands, c.Commands) {
					a, _ := json.Marshal(ctx.Commands)
					b, _ := json.Marshal(c.Commands)
					t.Fatalf("%s variant %d overlay differs\ngot %s\nwant %s", c.Ident, c.Variant, a, b)
				}
				sig := new(DrawSignature).Reset()
				v.DrawSignature(sig)
				if sig.Lo != c.Lo || sig.Hi != c.Hi {
					t.Fatalf("%s variant %d signature differs: got %+v want %d/%d", c.Ident, c.Variant, sig, c.Lo, c.Hi)
				}
			}
		})
	}
}
