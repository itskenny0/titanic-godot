package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestSpriteRuntimeReference(t *testing.T) {
	path := os.Getenv("TAOOT_SPRITE_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_SPRITE_RUNTIME_REFERENCE to owned-data rendering hashes")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type rectangle struct {
		X, Y float64
		W, H int
	}
	type propCase struct {
		Path, Group, State, Pixels, Hits string
		Mode, Frame                      int
		Variant, Order                   []int
		Rect                             *rectangle
	}
	type actorCase struct {
		Path, Member, Pose, Pixels, Hits string
		Mode                             int
		Rect                             *rectangle
		OnScreen                         bool
	}
	var corpus struct {
		Props    []propCase
		Actors   []actorCase
		Geometry []struct {
			Cam        WorldCamera
			XYZ        [3]float64
			Projection *Projection
			Bearing    int
		}
		Signature [][2]uint32
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for i, e := range corpus.Geometry {
		got := ProjectPoint(e.Cam, e.XYZ[0], e.XYZ[1], e.XYZ[2])
		if !reflect.DeepEqual(got, e.Projection) || Bearing(e.XYZ[0], e.XYZ[1]) != e.Bearing {
			t.Fatalf("geometry %d: got %+v want %+v", i, got, e.Projection)
		}
	}
	sig := new(DrawSignature).Reset()
	values := []any{float64(0), float64(1), float64(-1), float64(2147483648), -.5, math.Pi, "Titanic", "𐀀é", true, false}
	for i, v := range values {
		switch v := v.(type) {
		case float64:
			sig.Num(v)
		case string:
			sig.Str(v)
		case bool:
			sig.Bool(v)
		}
		if [2]uint32{sig.Lo, sig.Hi} != corpus.Signature[i] {
			t.Fatalf("signature %d differs", i)
		}
	}
	cam := WorldCamera{F: 96, CX: 32, CY: 24, ClipW: 60, ClipH: 44}
	palette := make([]byte, 1024)
	for i := range palette {
		palette[i] = byte(i*71 + 13)
	}
	occ := &Occlusion{Z: make([]byte, 64*48), W: 64, H: 48, Scale: 16, Levels: 256, GroundBias: 3}
	for i := range occ.Z {
		if i%7 != 0 {
			occ.Z[i] = 255
		}
	}
	fingerprint := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	checkRect := func(t *testing.T, got *SpriteRect, want *rectangle) {
		t.Helper()
		var actual *rectangle
		if got != nil {
			actual = &rectangle{got.X, got.Y, got.W, got.H}
		}
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("rect got %+v want %+v", actual, want)
		}
	}
	blank := func() []byte {
		b := make([]byte, 64*48*4)
		for i := range b {
			b[i] = 17
		}
		return b
	}
	shops := map[string]*df.Shop{}
	casts := map[string]*df.Cast{}
	for _, e := range corpus.Props {
		t.Run(e.Path+"/"+e.Group+"/"+e.State+"/"+string(rune('0'+e.Mode)), func(t *testing.T) {
			shp := shops[e.Path]
			if shp == nil {
				b, err := os.ReadFile(e.Path)
				if err != nil {
					t.Fatal(err)
				}
				shp, err = df.ReadShop(b)
				if err != nil {
					t.Fatal(err)
				}
				shops[e.Path] = shp
			}
			r := new(PropRuntime)
			r.AddShop("test", shp)
			p := r.Get(e.Group)
			p.Visible = true
			p.StateName = e.State
			st := p.State()
			if st == nil {
				t.Fatal("missing state")
			}
			p.Deg = script.Num([]float64{-.5, 63.5, 128, 257}[e.Mode])
			p.FrameIdx = e.Mode % len(st.Frames)
			if e.Mode == 3 {
				p.FrameOrder = PlaySequence(st, DegreeVariantFrames(st, p.Deg.Number))
			}
			if got := DegreeVariantFrames(st, p.Deg.Number); !reflect.DeepEqual(got, e.Variant) {
				t.Fatalf("variant %v != %v", got, e.Variant)
			}
			if p.CurrentFrameIdx(st) != e.Frame || !reflect.DeepEqual(p.FrameOrder, e.Order) {
				t.Fatal("frame/play order differs")
			}
			p.WorldSpace = e.Mode != 0
			p.Directional = e.Mode == 2
			p.WorldX = 96
			p.WorldY = 4
			p.WorldZ = 2
			p.Scale = 62.5
			p.Zclip = float64(e.Mode * 5)
			f, err := p.frame(st, p.CurrentFrameIdx(st))
			if err != nil {
				t.Fatal(err)
			}
			p.AnchorX = float64(f.PosXraw - f.Width/2 + 32)
			p.AnchorY = float64(f.PosYraw - f.Height/2 + 24)
			var rect *SpriteRect
			if p.WorldSpace {
				rect, err = r.WorldRect(PropDrawEntry{p, *ProjectPoint(cam, p.WorldX, p.WorldY, p.WorldZ)}, cam)
			} else {
				rect, err = p.ScreenRect()
			}
			if err != nil {
				t.Fatal(err)
			}
			checkRect(t, rect, e.Rect)
			pixels, hits := blank(), make([]byte, 64*48)
			if err = r.Composite(pixels, 64, 48, palette, math.Inf(-1), &cam, false, occ); err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 48; y++ {
				for x := 0; x < 64; x++ {
					hit, err := r.PropAt(x, y, &cam, false, occ)
					if err != nil {
						t.Fatal(err)
					}
					if hit != nil {
						hits[y*64+x] = 1
					}
				}
			}
			if fingerprint(pixels) != e.Pixels {
				t.Fatal("rendered pixels differ")
			}
			if fingerprint(hits) != e.Hits {
				t.Fatal("hit map differs")
			}
		})
	}
	for _, e := range corpus.Actors {
		t.Run(e.Path+"/"+e.Member+"/"+e.Pose+"/"+string(rune('0'+e.Mode)), func(t *testing.T) {
			cst := casts[e.Path]
			if cst == nil {
				b, err := os.ReadFile(e.Path)
				if err != nil {
					t.Fatal(err)
				}
				cst, err = df.ReadCast(b)
				if err != nil {
					t.Fatal(err)
				}
				casts[e.Path] = cst
			}
			r := new(ActorRuntime)
			r.AddCast("test", cst)
			a := r.Get(e.Member)
			a.Visible = true
			a.PoseName = e.Pose
			p := a.Pose()
			if len(p.Play) > 0 {
				a.Step = e.Mode % len(p.Play)
			}
			a.Deg = []float64{-.5, 63.5, 128, 257}[e.Mode]
			a.WorldSpace = e.Mode != 0
			a.WorldX = 96
			a.WorldY = 4
			a.WorldZ = 2
			if e.Mode != 0 {
				a.Scale = 62.5
			}
			a.Zclip = float64(e.Mode * 5)
			f, err := r.FrameFor(a, nil)
			if err != nil {
				t.Fatal(err)
			}
			a.AnchorX, a.AnchorY = 32, 24
			if f != nil {
				a.AnchorX = float64(f.PosXraw - f.Width/2 + 32)
				a.AnchorY = float64(f.PosYraw - f.Height/2 + 24)
			}
			var rect *SpriteRect
			if a.WorldSpace {
				rect, err = r.Rect(ActorDrawEntry{a, *ProjectPoint(cam, a.WorldX, a.WorldY, a.WorldZ)}, cam)
			} else {
				rect, err = r.ScreenRect(a)
			}
			if err != nil {
				t.Fatal(err)
			}
			checkRect(t, rect, e.Rect)
			on, err := r.OnScreen(a, cam)
			if err != nil || on != e.OnScreen {
				t.Fatal("onscreen differs", err)
			}
			pixels, hits := blank(), make([]byte, 64*48)
			if err = r.Composite(pixels, 64, 48, palette, cam, occ); err != nil {
				t.Fatal(err)
			}
			if err = r.CompositeScreen(pixels, 64, 48, palette); err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 48; y++ {
				for x := 0; x < 64; x++ {
					hit, err := r.ActorAt(x, y, &cam, occ)
					if err != nil {
						t.Fatal(err)
					}
					if hit != nil {
						hits[y*64+x] = 1
					}
				}
			}
			if fingerprint(pixels) != e.Pixels {
				t.Fatal("rendered pixels differ")
			}
			if fingerprint(hits) != e.Hits {
				t.Fatal("hit map differs")
			}
		})
	}
	t.Logf("Compared %d prop scenes, %d actor scenes, and %d projection cases", len(corpus.Props), len(corpus.Actors), len(corpus.Geometry))
}
