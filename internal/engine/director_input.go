package engine

import (
	"fmt"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
)

// A host input task exists before its Go body runs. The reference starts the
// async handler before adding that handler to the busy set, so entry guards must
// exclude their own task. Other running scripts still lock input normally.
func (d *ScreenDirector) entryScriptBusy() bool {
	current := d.Session.Executor.Current()
	for _, task := range d.Session.inflight.All() {
		if task != current && !task.Done() {
			return true
		}
	}
	return false
}
func (d *ScreenDirector) KeyDown(key string, special bool) (bool, error) {
	s := d.Session
	if d.Movies.Playing() {
		_, err := d.Movies.Key(key, special)
		return true, err
	}
	if s.PuppetCtrl.Key(key, special) {
		return true, nil
	}
	if d.roomAnimating() || d.entryScriptBusy() {
		s.Events.Post(QueuedEvent{Kind: "keydown", Key: key, Special: special}, true)
		return true, nil
	}
	if d.Busy() {
		return false, nil
	}
	if s.Executor.Current() == nil {
		s.Track("key:"+key, false, func(*Task) error { _, err := d.KeyDown(key, special); return err })
		return true, nil
	}
	target := s.StageCtrl.KeydownTarget()
	if !s.ViewShowing() && target != nil {
		_, err := s.Interp.Run(target, "keydown", []script.Value{script.Str(key)}, script.CallContext{Me: target.Name}, nil)
		if err != nil {
			d.Log("stage keydown: " + err.Error())
		}
		return true, nil
	}
	if d.room != nil {
		return d.room.RoomKeyDown(key)
	}
	for _, router := range s.BootScripts {
		if !hasHandler(router, "keydown") {
			continue
		}

		result, err := s.Interp.Run(router, "keydown", []script.Value{script.Str(key)}, script.CallContext{Me: router.Name, Target: key}, nil)
		if err != nil {
			d.Log(fmt.Sprintf("script error in %s.keydown: %v", router.Name, err))
		}
		return result.Consumed, nil
	}
	return false, nil
}
func (d *ScreenDirector) Click(x, y float64) error {
	s := d.Session
	if s.Executor.Current() == nil {
		busy := d.InputLocked()
		s.SetPointer(x, y)
		s.Track("click", false, func(task *Task) error {
			if err := d.press(task, x, y, busy); err != nil {
				return err
			}
			d.Release(x, y)
			return nil
		})
		return nil
	}
	if err := d.Press(x, y); err != nil {
		return err
	}
	d.Release(x, y)
	return nil
}
func (d *ScreenDirector) Press(x, y float64) error {
	s := d.Session
	s.SetPointer(x, y)
	busy := d.Busy() || d.entryScriptBusy()
	if task := s.Executor.Current(); task != nil {
		return d.press(task, x, y, busy)
	}
	s.Track("pointer press", false, func(task *Task) error { return d.press(task, x, y, busy) })
	return nil
}
func (d *ScreenDirector) press(task *Task, x, y float64, busy bool) error {
	room := d.room
	if room != nil {
		prev := room.ArmNavHooks()
		defer func() {
			if d.room != nil {
				d.room.DisarmNavHooks(&prev)
			}
		}()
	}
	return d.clickDispatch(task, x, y, busy)
}
func (d *ScreenDirector) Release(x, y float64) {
	if d.Conversing() {
		d.Session.PuppetCtrl.Release(d.PuppetView.BevelAt(x, y))
	}
}
func (d *ScreenDirector) clickDispatch(task *Task, x, y float64, busy bool) error {
	s := d.Session
	if d.Movies.Playing() {
		return d.Movies.Click(x, y)
	}
	if d.Conversing() {
		s.PuppetCtrl.Press(task, d.PuppetView.BevelAt(x, y), y < PuppetArtHeight)
		return nil
	}
	if d.eventsLocked() {
		return nil
	}
	if busy {
		if !s.PollingInput() {
			s.Events.Post(QueuedEvent{Kind: "mousedown", X: x, Y: y}, false)
		}
		return nil
	}
	for _, inst := range s.BootScripts {
		if !hasHandler(inst, "mousedown") {
			continue
		}
		_, err := s.Interp.Run(inst, "mousedown", []script.Value{script.Num(float64(s.PointerPoint()))}, script.CallContext{Me: inst.Name}, nil)
		if err != nil {
			d.Log(fmt.Sprintf("script error in %s.mousedown: %v", inst.Name, err))
		}
		return nil
	}
	if handled, err := d.clickProp(x, y); handled || err != nil {
		return err
	}
	if d.room != nil {
		if handled, err := d.room.RoomClickAt(x, y); handled || err != nil {
			return err
		}
	}
	if s.StageOpen() {
		if handled, err := s.StageCtrl.ClickAtPoint(x, y); handled || err != nil {
			return err
		}
	}
	d.clickFlatSurface()
	return nil
}
func (d *ScreenDirector) PropUnder(x, y float64) (*PropInstance, error) {
	showing := d.Session.ViewShowing()
	var cam *WorldCamera
	var occ *Occlusion
	if showing {
		cam, occ = d.roomCamera(), d.roomOcclusion()
	}
	return d.Session.Props.PropAtPoint(x, y, cam, showing, occ)
}
func (d *ScreenDirector) clickProp(x, y float64) (bool, error) {
	prop, err := d.PropUnder(x, y)
	if err != nil || prop == nil {
		return false, err
	}
	s := d.Session
	name := prop.Name
	if name == "" {
		name = prop.Group.Name
	}
	own, main := s.PropScripts.Get(strings.ToLower(prop.Group.Name)), s.ShopMain(prop.Shop.Name)
	chain := []*script.Instance{}
	for _, inst := range []*script.Instance{own, main} {
		if hasHandler(inst, "mousedown") {
			chain = append(chain, inst)
		}
	}
	if len(chain) == 0 {
		return false, nil
	}

	for _, inst := range chain {
		result, err := s.Interp.Run(inst, "mousedown", []script.Value{script.Num(float64(s.PointerPoint()))}, script.CallContext{Me: name, Target: name}, nil)
		if err != nil {
			d.Log(fmt.Sprintf("script error in %s.mousedown: %v", name, err))
			break
		}
		if result.Consumed || result.Handled && !result.Passed {
			break
		}
	}
	d.Log("click prop " + name)
	return true, nil
}
func (d *ScreenDirector) clickFlatSurface() {
	s := d.Session
	flat := s.FlatScripts.Get(strings.ToLower(s.StageCtrl.CurrentFlat))

	for _, inst := range []*script.Instance{flat, s.StageScript} {
		if !hasHandler(inst, "mousedown") {
			continue
		}
		result, err := s.Interp.Run(inst, "mousedown", []script.Value{script.Str("")}, script.CallContext{Me: inst.Name}, nil)
		if err != nil {
			d.Log(fmt.Sprintf("script error in %s.mousedown: %v", inst.Name, err))
			continue
		}
		if result.Consumed || result.Handled && !result.Passed {
			return
		}
	}
}
func (d *ScreenDirector) flatRegionAt(x, y float64) *df.StageRegion {
	for _, r := range d.Session.StageCtrl.CurrentFlatRegions() {
		if x >= float64(r.Left) && x <= float64(r.Right) && y >= float64(r.Top) && y <= float64(r.Bottom) {
			return &r
		}
	}
	return nil
}
func (d *ScreenDirector) HitTestAt(x, y float64) (HitTarget, error) {
	prop, err := d.PropUnder(x, y)
	if err != nil {
		return HitTarget{}, err
	}
	if prop != nil {
		name := prop.Name
		if name == "" {
			name = prop.Group.Name
		}
		return HitTarget{Name: name, Type: "prop"}, nil
	}
	if d.room != nil {
		hit, err := d.room.RoomHitTest(x, y)
		if err != nil {
			return HitTarget{}, err
		}
		if hit != nil {
			return *hit, nil
		}
	}
	s := d.Session
	if s.StageOpen() && s.StageCtrl.CurrentFlat != "none" {
		if r := d.flatRegionAt(x, y); r != nil {
			return HitTarget{Name: r.Name, Type: "button"}, nil
		}
		return HitTarget{Name: s.StageCtrl.CurrentFlat, Type: "flat"}, nil
	}
	return HitTarget{Type: "none"}, nil
}
func (d *ScreenDirector) serviceCursor() {
	if d.OnCursor == nil {
		return
	}
	s := d.Session
	gate := fmt.Sprintf("%s|%t|%t|%s|%s", d.Movies.PlayingFile(), d.Conversing(), d.eventsLocked(), s.StageCtrl.CurrentFlat, s.SetName)
	if gate == d.cursorGate {
		return
	}
	d.cursorGate = gate
	s.Track("cursor", true, func(*Task) error {
		name, err := d.Hover(s.PointerX, s.PointerY)
		if err == nil && d.OnCursor != nil {
			d.OnCursor(name)
		}
		return nil
	})
}
func (d *ScreenDirector) Hover(x, y float64) (string, error) {
	s := d.Session
	s.SetPointer(x, y)
	if s.CursorHidden() {
		return "none", nil
	}
	if d.Movies.Playing() {
		if d.Movies.ClickableAt(x, y) {
			return "touch", nil
		}
		return "", nil
	}
	if d.Conversing() {
		if d.PuppetView.BevelAt(x, y) >= 0 {
			return "touch", nil
		}
		return "", nil
	}
	if d.eventsLocked() {
		return "watch", nil
	}
	hit, err := d.HitTestAt(x, y)
	if err != nil {
		return "", err
	}
	point := script.Num(float64(s.PointerPoint()))
	caller := "boot script"
	if b := s.Boot(); b != nil {
		caller = b.Name
	}
	s.CursorName = ""
	switch hit.Type {
	case "actor", "prop", "scene", "flat":
		_, _ = s.SendEvent("sendto"+hit.Type, hit.Name, "setcursor", []script.Value{point}, caller, nil)
	case "button":
		_, _ = s.StageCtrl.SendToButton(s.StageCtrl.CurrentFlat, hit.Name, "setcursor", []script.Value{point}, caller, nil)
	case "painting":
		if d.room != nil {
			d.room.SendRoomPainting(hit.Name, "setcursor", point)
		}
	}
	return s.CursorName, nil
}

func (d *ScreenDirector) eventsLocked() bool {
	value, _ := d.Session.Interp.Globals.Get("lockevents")
	return value.Truthy()
}
