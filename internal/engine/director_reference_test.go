package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
)

type directorTestRoom struct {
	RoomLayer
	frame   *CachedFrame
	palette []byte
}

func (r *directorTestRoom) RoomFrame() *CachedFrame       { return r.frame }
func (r *directorTestRoom) RoomPalette() []byte           { return r.palette }
func (r *directorTestRoom) RoomPropPalette() []byte       { return r.palette }
func (r *directorTestRoom) BandPropPalette([]byte) []byte { return r.palette }
func (r *directorTestRoom) RoomVersion() int              { return 4 }
func (r *directorTestRoom) RoomAnimating() bool           { return false }
func (r *directorTestRoom) RoomCamera() *WorldCamera      { return nil }
func (r *directorTestRoom) RoomOcclusion() *Occlusion     { return nil }
func (r *directorTestRoom) RoomSignature(*DrawSignature)  {}
func (r *directorTestRoom) DrawRoomHotspots(*DrawContext) {}

func TestDirectorReference(t *testing.T) {
	b, err := os.ReadFile("../../tests/fixtures/director.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Input struct {
			Flat, Owner, Dir               string
			Room, Showing, Subtitle, Photo bool
			Fade, Gamma, Step, Span        float64
			Dim                            *ClutDim
			Xray                           *struct{ X, Y float64 }
		}
		Owner, Pixels  string
		Valid, Repaint bool
		Commands       []DrawCommand
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	for index, want := range corpus {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			input := want.Input
			s := NewSession(func(string) ([]byte, error) { return nil, nil }, new(HostAudio))
			defer s.Close()
			d := NewScreenDirector(s, 512, 384, nil)
			if input.Gamma != 0 {
				d.Gamma.Set(input.Gamma, AllGammaChannels)
			}
			raw := make([]byte, 2048)
			for i := 0; i < 256; i++ {
				raw[i*8+3], raw[i*8+5], raw[i*8+7] = byte(i), byte(i*3), byte(i*7)
			}
			base := df.PaletteRGBA(raw, 256, binary.LittleEndian)
			roomPal := d.Gamma.DisplayPalette(base)
			cur := &CachedFrame{Pixels: make([]byte, 512*264), Width: 512, Height: 264}
			for i := range cur.Pixels {
				cur.Pixels[i] = byte(i * 13)
			}
			if input.Room {
				d.SetRoom(&directorTestRoom{frame: cur, palette: roomPal})
				s.SetName = "room"
			}
			s.SetVisible = input.Showing
			flat := &FlatImage{Pixels: make([]byte, 512*384), Width: 512, Height: 384, Palette: base}
			for i := range flat.Pixels {
				flat.Pixels[i] = byte(i * 5)
				if input.Flat == "matte" {
					flat.Pixels[i] = 17
				}
			}
			if input.Flat != "none" {
				s.StageCtrl.Name = "test"
				s.StageCtrl.CurrentFlat = "main"
				s.StageCtrl.File = &df.Stage{}
				s.StageCtrl.images["test:main"] = flat
			}
			if input.Xray != nil {
				hidden := &FlatImage{Pixels: make([]byte, 512*384), Width: 512, Height: 384, Palette: base}
				for i := range hidden.Pixels {
					hidden.Pixels[i] = byte(i*9 + 33)
				}
				s.StageCtrl.images["test:hidden"] = hidden
				shop := s.Props.AddShop("mask.shp", testShop())
				seedSprites(shop)
				s.Props.Instance("door", "mask")
				s.XRay = &XRayReveal{Aimed: true, Hidden: "hidden", Mask: "mask", X: input.Xray.X, Y: input.Xray.Y}
			}
			d.stageDim = input.Dim
			for i := range d.Screen.Frame {
				d.Screen.Frame[i] = byte(i * 7)
			}
			s.TextOverlay = []TextOverlay{{Text: "Caption", X: 7, Y: 23, Size: 12, Color: 2}}
			s.Fade.Level = input.Fade
			rgba := make([]byte, 512*384*4)
			for i := range rgba {
				rgba[i] = byte(i*11 + 31)
			}
			switch input.Owner {
			case "held":
				s.Fade.PendingReveal = true
			case "faded":
				s.Fade.Snapshot = &RGBAFrame{RGBA: rgba, Width: 512, Height: 384}
			case "puppet":
				chosen := 0
				p := &PuppetState{Name: "test.pup", Visible: true, Pup: &df.Puppet{PaletteRaw: raw, File: &df.File{}}, Bevels: []Bevel{{Text: "Answer", ID: 1}}, Chosen: &chosen}
				if input.Subtitle {
					p.Subtitle = "The character speaks."
				}
				s.PuppetCtrl.Puppet = p
			case "movie":
				seg := &df.MovieSegment{File: &df.File{Order: binary.LittleEndian, Containers: []df.Container{{Data: []byte{1, 0, 2, 0, 4, 51, 97}}}}, Frames: []df.MovieFrame{{LocationFrame: 0}}, PaletteRaw: raw, OriginX: 17, OriginY: 31}
				d.Movies.active = &activeMovie{name: "test.mov", frames: NewMovieFrames(seg), seg: seg, palette: roomPal, paletteGeneration: d.Gamma.Generation}
			}
			if input.Dir != "" {
				to := make([]byte, len(rgba))
				for i := range to {
					to[i] = byte(i*17 + 19)
				}
				s.Wipe = WipeState{Dir: input.Dir, Step: input.Step, Steps: 6, Span: input.Span, From: &RGBAFrame{RGBA: rgba, Width: 512, Height: 384}, To: &RGBAFrame{RGBA: to, Width: 512, Height: 384}}
			}
			if input.Photo {
				s.PhotoOverlay = &PhotoOverlay{Photo: &RGBAFrame{RGBA: []byte{81, 82, 83, 255, 91, 92, 93, 255}, Width: 2, Height: 1}, X: 3, Y: 4}
			}
			ctx := NewDrawContext(512, 384)
			ctx.Measure = func(text, font string) float64 { return float64(len(script.UTF16Units(text)) * 7) }
			if err := d.Render(ctx); err != nil {
				t.Fatal(err)
			}
			version := ctx.Version
			if err := d.Render(ctx); err != nil {
				t.Fatal(err)
			}
			h := sha256.Sum256(d.Screen.Frame)
			if hex.EncodeToString(h[:]) != want.Pixels {
				t.Fatalf("pixels differ for %+v: got %x want %s", input, h, want.Pixels)
			}
			if d.ScreenOwner() != want.Owner || d.Screen.FrameValid != want.Valid || (ctx.Version != version) != want.Repaint {
				t.Fatal("screen ownership, validity or repaint differs", input, d.ScreenOwner(), want.Owner, ctx.Version, version, want.Repaint)
			}
			if len(ctx.Commands) != len(want.Commands) || len(want.Commands) > 0 && !reflect.DeepEqual(ctx.Commands, want.Commands) {
				a, _ := json.Marshal(ctx.Commands)
				b, _ := json.Marshal(want.Commands)
				t.Fatalf("overlay commands differ\ngot %s\nwant %s", a, b)
			}
		})
	}
}
