package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"strings"
)

type RoomHit struct {
	Index  int
	Object *df.ObjectEntry
}

func (v *SetViewer) HitTest(x, y float64) *RoomHit {
	if v.Busy() {
		return nil
	}
	objects := v.View().Objects
	var best *RoomHit
	area := math.Inf(1)
	for i := range objects {
		o := &objects[i]
		x0, x1 := float64(min(o.StartRegionX, o.EndRegionX)), float64(max(o.StartRegionX, o.EndRegionX))
		y0, y1 := float64(min(o.StartRegionY, o.EndRegionY)), float64(max(o.StartRegionY, o.EndRegionY))
		if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
			a := float64((x1 - x0) * (y1 - y0))
			if a < area {
				area = a
				best = &RoomHit{i, o}
			}
		}
	}
	return best
}
func (v *SetViewer) PointInRoomImage(x, y float64) bool {
	return v.Session.ViewShowing() && v.current != nil && x >= 0 && y >= 0 && x < float64(v.current.Width) && y < float64(v.current.Height)
}
func (v *SetViewer) RoomHitTest(x, y float64) (*HitTarget, error) {
	if !v.PointInRoomImage(x, y) {
		return nil, nil
	}
	cam := v.WorldCamera()
	if cam != nil {
		actor, err := v.Session.Actors.ActorAtPoint(x, y, cam, v.RoomOcclusion())
		if err != nil {
			return nil, err
		}
		if actor != nil {
			who := actor.Name
			if who == "" {
				who = actor.Member.Name
			}
			return &HitTarget{who, "actor"}, nil
		}
	}
	if hit := v.HitTest(x, y); hit != nil {
		return &HitTarget{hit.Object.Identifier, "painting"}, nil
	}
	return &HitTarget{strings.ToLower(v.Scene().SceneName), "scene"}, nil
}
func (v *SetViewer) RoomClickAt(x, y float64) (bool, error) {
	if handled, err := v.clickActor(x, y); handled || err != nil {
		return handled, err
	}
	if !v.PointInRoomImage(x, y) {
		return false, nil
	}
	if v.clickHotspot(x, y) {
		return true, nil
	}
	name := strings.ToLower(v.Scene().SceneName)
	_, err := v.Session.SendEvent("sendtoscene", name, "mousedown", []script.Value{script.Num(float64(v.Session.PointerPoint()))}, name, nil)
	return true, err
}
func (v *SetViewer) clickActor(x, y float64) (bool, error) {
	s := v.Session
	if !s.ViewShowing() || v.current == nil || y >= float64(v.current.Height) {
		return false, nil
	}
	cam := v.WorldCamera()
	if cam == nil {
		return false, nil
	}
	actor, err := s.Actors.ActorAtPoint(x, y, cam, v.RoomOcclusion())
	if err != nil {
		return false, err
	}
	if actor == nil {
		return false, nil
	}
	who := actor.Name
	if who == "" {
		who = actor.Member.Name
	}
	inst := s.CastScripts.Get(who)
	if !hasHandler(inst, "mousedown") {
		return false, nil
	}
	if _, err = s.Interp.Run(inst, "mousedown", []script.Value{script.Str(who)}, script.CallContext{Me: who, Target: who}, nil); err != nil {
		v.Log(fmt.Sprintf("script error in %s.mousedown: %v", who, err))
	}
	v.Log("click actor " + who)
	return true, nil
}
func (v *SetViewer) clickHotspot(x, y float64) bool {
	if !v.Session.ViewShowing() || v.current == nil || y >= float64(v.current.Height) {
		return false
	}
	hit := v.HitTest(x, y)
	if hit == nil {
		return false
	}
	consumed := v.Scripts.MouseDown(v.SceneIdx, v.ViewIdx, hit.Index, hit.Object.Identifier)
	line := "click " + hit.Object.Identifier
	if !consumed {
		line += " (unhandled)"
	}
	v.Log(line)
	return true
}
func (v *SetViewer) SendRoomPainting(name, handler string, point script.Value) {
	v.Scripts.PaintingEvent(v.Scene().SceneName, v.View().ViewName, name, handler, []script.Value{point}, nil)
}
func (v *SetViewer) DrawRoomHotspots(ctx *DrawContext) {
	if !v.ShowHotspots || v.Animating() {
		return
	}
	ctx.Save()
	defer ctx.Restore()
	ctx.Stroke = "rgba(255, 220, 120, 0.9)"
	ctx.Fill = ctx.Stroke
	ctx.Font = "10px sans-serif"
	for _, o := range v.View().Objects {
		x, y := float64(min(o.StartRegionX, o.EndRegionX)), float64(min(o.StartRegionY, o.EndRegionY))
		w, h := math.Abs(float64(o.EndRegionX-o.StartRegionX)), math.Abs(float64(o.EndRegionY-o.StartRegionY))
		ctx.StrokeRect(x+.5, y+.5, w, h)
		ctx.FillText(o.Identifier, x+2, y+10)
	}
}

// MapFrame returns the map image and standpoint markers. The host can present
// it independently from the 512x384 game surface without resizing that surface.
type MapMarker struct {
	X, Y    float64
	Current bool
}

func (v *SetViewer) MapFrame() (*RGBAFrame, []MapMarker, error) {
	fb := new(df.FrameBuffer)
	f, err := df.DecodeFrame(v.Set.File.Data(v.Set.MapLight), fb, v.Set.File.Order)
	if err != nil {
		return nil, nil, err
	}
	palette := v.Gamma.DisplayPalette(df.PaletteRGBA(v.Set.PaletteRaw, 256, v.Set.File.Order))
	rgba := make([]byte, f.Width*f.Height*4)
	if err = df.IndexedRGBA(fb.Pixels[:f.Width*f.Height], palette, rgba); err != nil {
		return nil, nil, err
	}
	markers := make([]MapMarker, len(v.Set.Scenes))
	for i, scene := range v.Set.Scenes {
		markers[i] = MapMarker{float64(scene.XAxisMap), float64(scene.ZAxisMap), i == v.SceneIdx}
	}
	return &RGBAFrame{RGBA: rgba, Width: f.Width, Height: f.Height}, markers, nil
}
