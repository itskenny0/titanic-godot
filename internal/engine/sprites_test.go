package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"reflect"
	"testing"
)

func testShop() *df.Shop {
	return &df.Shop{Groups: []df.PropGroup{{Name: "door", States: []df.PropState{{Identifier: "opening", Animated: true, Frames: []int{1, 2, 3}, RefScales: []int{96, 96, 96}, Degrees: []int{0, 0, 0}}}}, {Name: "button", States: []df.PropState{{Identifier: "idle", Frames: []int{1}, RefScales: []int{96}}}}}}
}
func seedSprites(shop *LoadedShop) {
	shop.frames = map[int]*df.Sprite{}
	for i := 1; i <= 3; i++ {
		shop.frames[i] = &df.Sprite{Width: 2, Height: 2, Indexed: []byte{byte(i), byte(i), byte(i), byte(i)}, Opaque: []byte{1, 0, 1, 1}}
	}
}
func TestPropAnimationAndInstances(t *testing.T) {
	r := new(PropRuntime)
	shop := r.AddShop("ROOM.SHP", testShop())
	seedSprites(shop)
	p := r.Get("DOOR")
	p.Visible, p.Animating = true, true
	p.LastTick = 10
	r.Tick(1000, 50)
	if p.FrameIdx != 1 || !p.Animating {
		t.Fatal("a late tick must advance only one frame")
	}
	r.Tick(1049, 50)
	if p.FrameIdx != 1 {
		t.Fatal("frame advanced early")
	}
	r.Tick(1050, 50)
	if p.FrameIdx != 2 || p.Animating {
		t.Fatal("animation must hold its last frame")
	}
	p.FrameIdx = 0
	p.Animating = true
	p.FrameLocked = true
	r.Tick(2000, 50)
	if p.FrameIdx != 0 {
		t.Fatal("locked prop advanced")
	}
	p.Deg = script.Str("63.5")
	p.Owner = script.Str("inventory")
	p.Value = script.Num(42)
	p.Scale = 1000
	p.WorldX = 60
	p.StateName = "opening"
	p.Dist = 12
	p.Directional = true
	p.SetName = "room"
	r.Instance("door", "DoorCopy")
	copy := r.Get("doorcopy")
	if copy == nil || copy.Group != p.Group || copy.Shop != shop || copy.Name != "DoorCopy" || copy.Deg != p.Deg || copy.StateName != p.StateName || copy.Dist != 12 || !copy.Directional || copy.SetName != "room" {
		t.Fatal("instance did not inherit the reference properties")
	}
	if copy.Owner.String() != "none" || copy.Value.Number != 0 || copy.WorldX != 0 || copy.Scale != 0 || copy.FrameIdx != 0 || copy.FrameLocked {
		t.Fatal("instance inherited properties that should reset")
	}
	button := r.Get("button")
	r.Instance("door", "button")
	if r.Get("button") != button {
		t.Fatal("instance replaced an existing group")
	}
	copy.StarPending = true
	copy.StarName = "anchor"
	copy.FrameLocked = true
	copy.FrameOrder = []int{2, 1}
	r.CurrentSet = "room"
	if r.SettleStars([]StarPoint{{"Anchor", 10, 20, 30}}, true) != 1 || copy.WorldX != 10 || copy.WorldY != 30 || copy.WorldZ != 20 || copy.Scale != 1000 || copy.FrameLocked || copy.FrameOrder != nil || copy.StarPending {
		t.Fatal("star placement differs")
	}
	r.RemoveShop("room.shp")
	if r.Props.Len() != 0 || r.Shops.Len() != 0 {
		t.Fatal("removed shop kept instances alive")
	}
}
func TestPropDrawingHitsAndInvalidation(t *testing.T) {
	r := new(PropRuntime)
	shop := r.AddShop("room", testShop())
	seedSprites(shop)
	a, b := r.Get("door"), r.Get("button")
	a.Visible, b.Visible = true, true
	a.AnchorX, a.AnchorY, b.AnchorX, b.AnchorY = 1, 1, 1, 1
	b.FrameIdx = 0
	hit, err := r.PropAt(1, 1, nil, false, nil)
	if err != nil || hit != b {
		t.Fatal("equal depth must select the later group", err)
	}
	hit, _ = r.PropAt(2, 1, nil, false, nil)
	if hit != nil {
		t.Fatal("transparent pixel intercepted input")
	}
	b.Dist = 10
	hit, _ = r.PropAt(1, 1, nil, false, nil)
	if hit != a {
		t.Fatal("screen depth ordering differs")
	}
	s := new(DrawSignature).Reset()
	r.DrawSignature(s)
	before := *s
	a.Owner = script.Str("changed")
	r.DrawSignature(s.Reset())
	if *s != before {
		t.Fatal("nonvisual state caused a redraw")
	}
	a.FrameIdx++
	r.DrawSignature(s.Reset())
	if *s == before {
		t.Fatal("animation did not invalidate drawing")
	}
	palette := make([]byte, 1024)
	palette[4], palette[5], palette[6] = 20, 30, 40
	palette[8], palette[9], palette[10] = 50, 60, 70
	pixels := make([]byte, 4*4*4)
	for i := range pixels {
		pixels[i] = 17
	}
	if err = r.Composite(pixels, 4, 4, palette, math.Inf(-1), nil, false, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pixels[20:24], []byte{50, 60, 70, 17}) {
		t.Fatal("RGB or destination alpha differs", pixels[20:24])
	}
	a.WorldSpace = true
	b.Visible = false
	a.WorldX = 96
	a.WorldY = 0
	a.WorldZ = 0
	a.Scale = 1000
	cam := WorldCamera{F: 96, CX: 1, CY: 1, ClipW: 4, ClipH: 4}
	occ := &Occlusion{W: 4, H: 4, Scale: 16, Z: make([]byte, 16)}
	hit, _ = r.PropAt(1, 1, &cam, false, occ)
	if hit != nil {
		t.Fatal("scenery must block a prop behind it")
	}
	a.Zclip = 96
	hit, _ = r.PropAt(1, 1, &cam, false, occ)
	if hit != a {
		t.Fatal("zclip did not bring the prop in front of scenery")
	}
}
func TestDegreeSelectionAndPropertyNumbers(t *testing.T) {
	st := &df.PropState{Frames: []int{1, 2, 3, 4}, Degrees: []int{0, 0, 1, 1}, PlayOrder: []int{1, 0}}
	if got := PlaySequence(st, DegreeVariantFrames(st, 1)); !reflect.DeepEqual(got, []int{3, 2}) {
		t.Fatal(got)
	}
	st.Degrees = []int{0, 64, 128, 192}
	if DegreeVariantFrames(st, 64) != nil {
		t.Fatal("angles treated as variants")
	}
	for _, c := range []struct {
		deg  float64
		want int
	}{{-0.5, 0}, {32, 0}, {32.5, 1}, {255.5, 0}, {-65, 3}} {
		if got := FrameIndexForDegree(st, c.deg); got != c.want {
			t.Fatal(c, got)
		}
	}
	if propertyNumber(script.Str("63.5")) != 63.5 || propertyNumber(script.Str("0xff")) != 255 || !math.IsNaN(propertyNumber(script.Str("12x"))) {
		t.Fatal("host property conversion used script integer parsing")
	}
}
func TestActorLifecycleAndAnimation(t *testing.T) {
	cst := &df.Cast{Members: []df.CastMember{{Name: "steward", Poses: []df.CastPose{{Name: "stand", Play: []int{0, 1}, Steps: [][]df.CastFrame{{{Location: 1, Angle: 0, RefScale: 96}, {Location: 2, Angle: 128, RefScale: 192}}, {{Location: 3, Angle: 0, RefScale: 96}}}}}}}}
	r := new(ActorRuntime)
	cast := r.AddCast("CAST", cst)
	cast.frames = map[int]*df.Sprite{1: {Width: 1, Height: 1, Indexed: []byte{1}, Opaque: []byte{1}}, 2: {Width: 1, Height: 1, Indexed: []byte{2}, Opaque: []byte{1}}, 3: {Width: 1, Height: 1, Indexed: []byte{3}, Opaque: []byte{1}}}
	a := r.Get("STEWARD")
	a.Visible = true
	a.Deg = 128
	a.WorldX = 30
	a.Scale = 1000
	r.Instance("steward", "Copy")
	copy := r.Get("copy")
	if copy == nil || copy.Name != "Copy" || copy.Member != a.Member || copy.Visible || copy.Deg != 0 || copy.WorldX != 0 || copy.Scale != 0 || copy.Turn != 16 || copy.PoseName != "stand" {
		t.Fatal("actor instance defaults differ")
	}
	f, err := r.FrameFor(a, nil)
	if err != nil || f.Indexed[0] != 2 || actorRefScale(a) != 96 {
		t.Fatal("actor facing or reference scale differs", err)
	}
	r.AdvanceAnimation()
	if a.Step != 1 || copy.Step != 1 {
		t.Fatal("invisible actor animation stopped")
	}
	r.AdvanceAnimation()
	if a.Step != 0 {
		t.Fatal("actor animation did not wrap")
	}
	a.SetName = "ROOM"
	a.StarName = "ANCHOR"
	a.StarPending = true
	r.CurrentSet = "room"
	stars := []StarPoint{{"anchor", 1, 2, 3}}
	if r.SettleStars(stars, func(string) bool { return true }) != 0 || !a.StarPending {
		t.Fatal("star placement interrupted walking")
	}
	if r.SettleStars(stars, nil) != 1 || a.WorldY != 3 || a.WorldZ != 2 {
		t.Fatal("actor star coordinates differ")
	}
	// Opening a second cast with the same names keeps existing actor instances.
	replacement := r.AddCast("cast", cst)
	if replacement == cast || r.Get("steward") != a {
		t.Fatal("cast replacement overwrote actor")
	}
	r.RemoveCast("CAST")
	if r.Get("steward") != a {
		t.Fatal("removing replacement deleted an actor from the old cast")
	}
}

func TestFractionalScreenPlacement(t *testing.T) {
	f := &df.Sprite{Width: 1, Height: 1, Indexed: []byte{1}, Opaque: []byte{1}}
	rect := screenSprite(f, .5, 0)
	palette := make([]byte, 1024)
	palette[4], palette[5], palette[6] = 10, 20, 30
	pixels := make([]byte, 16)
	compositeSprite(rect, pixels, 2, 2, palette, 2, 2, nil, 0)
	if !reflect.DeepEqual(pixels[:6], []byte{0, 0, 10, 20, 30, 0}) {
		t.Fatal("fractional flat-buffer writes differ", pixels)
	}
	if rect.sample(1, 0) != -1 {
		t.Fatal("fractional opaque-mask index became clickable")
	}
}
