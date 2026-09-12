package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"strings"
)

func (s *Session) BootPlan() *BootPlan {
	if s.plan != nil {
		return s.plan
	}
	s.plan = &BootPlan{Resources: []string{}, Casts: []string{}, Volumes: []string{}}
	data, err := s.Read("bootfile")
	if err != nil || data == nil {
		return s.plan
	}
	plan := ReadBootPlan(data)
	s.plan = &plan
	return s.plan
}
func (s *Session) loadBootScripts() {
	if len(s.BootScripts) > 0 {
		return
	}
	data, err := s.Read("bootfile")
	if err != nil || data == nil {
		return
	}
	file, err := df.ReadFile(data)
	if err != nil {
		s.Log("bootfile: " + err.Error())
		return
	}
	counts := []string{}
	for i := 1; i < len(file.Containers); i++ {
		inst := s.InstanceFrom(file.Data(i), fmt.Sprintf("boot%d", i))
		if inst != nil {
			s.BootScripts = append(s.BootScripts, inst)
			counts = append(counts, fmt.Sprint(len(inst.Script.Handlers)))
		}
	}
	s.Log("boot scripts loaded (" + strings.Join(counts, "+") + " handlers)")
}
func (s *Session) seedBootGlobals() {
	if s.Interp.Globals.Has("handitem") {
		return
	}
	for _, name := range []string{"handitem", "savestage1", "savestage2", "savestage3", "saveflat1", "saveflat2", "saveflat3", "jumpset", "playerdeath", "loopsound"} {
		s.Interp.Globals.Set(name, script.Str(""))
	}
	s.Interp.Globals.Set("seldir", script.Str("north"))
	for _, name := range []string{"twocount", "threecount", "fourcount", "fivecount"} {
		s.Interp.Globals.Set(name, script.Num(1))
	}
	s.Interp.Globals.Set("themevolume", script.Num(24))
}
func (s *Session) loadBootLibrary() { s.loadBootScripts(); s.RefreshFallbacks(); s.seedBootGlobals() }
func (s *Session) EnsureBooted() {
	if s.coreLoaded {
		return
	}
	s.loadBootLibrary()
	s.LoadBootResources()
	inven := s.ShopMains.Get("inven.shp")
	if hasHandler(inven, "initprops") && !s.Interp.Globals.Has("__propsinit") {
		s.Interp.Globals.Set("__propsinit", script.Num(1))
		s.FireHandler(inven, "initprops", "inven.shp", "initprops")
	}
	if lever := s.Props.Get("themetoggle"); lever != nil {
		v, _ := s.Interp.Globals.Get("themevolume")
		lever.Deg = script.Num(math.Max(0, math.Min(5, math.Floor(propertyNumber(v)/8/6))))
	}
	s.coreLoaded = len(s.BootScripts) > 0
}
func (s *Session) BootedByGame() bool {
	closed := false
	for _, file := range s.BootPlan().Resources {
		if strings.HasSuffix(file, ".shp") && s.ShopMains.Has(strings.ToLower(file)) {
			s.CloseShop(file)
			closed = true
		}
	}
	if closed {
		s.Interp.Globals.Delete("__propsinit")
	}
	s.loadBootLibrary()
	s.coreLoaded = len(s.BootScripts) > 0
	return s.coreLoaded
}
func (s *Session) PrepareRestart(task *Task) error {
	if err := s.Settle(task, 1000); err != nil {
		return err
	}
	s.Scheduler.Reset()
	for _, c := range []AudioChannel{SoundChannel, VoiceChannel, ThemeChannel} {
		s.Audio.Halt(c)
	}
	s.CurrentThemeName = "none"
	s.PuppetCtrl.ClosePuppetFile()
	s.Fade.Queue = nil
	s.Fade.Snapshot = nil
	s.Fade.PendingReveal = false
	s.Fade.Blanked = false
	s.Fade.Level = 1
	s.CursorDepth = 0
	s.Wipe.End()
	s.StageCtrl.ResetOverlayStack()
	s.SetVisible = true
	s.coreLoaded = false
	return nil
}
func (s *Session) OpenTrackFile(file string) bool {
	key := strings.ToLower(file)
	data, err := s.Read(key)
	if err != nil || data == nil {
		s.Log(fmt.Sprintf("opentrackfile: %q not available", file))
		return false
	}
	if err = s.AudioLib.OpenBank(key, data); err != nil {
		return false
	}
	return true
}
func (s *Session) InstanceCastScript(src, dst string) {
	from := s.CastScripts.Get(strings.ToLower(src))
	if from == nil {
		return
	}
	key := strings.ToLower(dst)
	s.CastScripts.Set(key, &script.Instance{Name: key, Script: from.Script, Parent: from.Parent})
	s.instancedActors[key] = true
}
func (s *Session) DropInstancedScript(name string) {
	key := strings.ToLower(name)
	if !s.instancedActors[key] {
		return
	}
	delete(s.instancedActors, key)
	s.CastScripts.Delete(key)
}
func (s *Session) OpenCastFile(file string) bool {
	key := strings.ToLower(file)
	if s.CastMains.Has(key) {
		return true
	}
	data, err := s.Read(key)
	if err != nil || data == nil {
		s.Log(fmt.Sprintf("opencastfile: %q not available", file))
		return false
	}
	cast, err := df.ReadCast(data)
	if err != nil {
		s.Log(fmt.Sprintf("opencastfile: %s: %v", file, err))
		return false
	}
	s.Actors.AddCast(key, cast)
	main := s.InstanceFrom(cast.File.Data(cast.MainScriptLocation), key)
	s.CastMains.Set(key, main)
	for _, m := range cast.Members {
		inst := s.InstanceFrom(cast.File.Data(m.ScriptLocation), m.Name)
		if inst != nil {
			inst.Parent = main
			s.CastScripts.Set(m.Name, inst)
		}
	}
	s.FireHandler(main, "opencast", key, "opencast "+key)
	for _, m := range cast.Members {
		s.FireHandler(s.CastScripts.Get(m.Name), "openactor", m.Name, "")
	}
	s.Log(fmt.Sprintf("cast loaded: %s (%d characters)", key, len(cast.Members)))
	return true
}
func (s *Session) CloseCastFile(file string) {
	key := strings.ToLower(file)
	cast := s.Actors.Casts.Get(key)
	if cast != nil {
		for _, m := range cast.Cst.Members {
			s.CastScripts.Delete(m.Name)
		}
		for name, a := range s.Actors.Actors.All() {
			if a.Cast == cast {
				s.DropInstancedScript(name)
			}
		}
	}
	s.CastMains.Delete(key)
	s.Actors.RemoveCast(key)
}
func (s *Session) ShopMain(name string) *script.Instance {
	return s.ShopMains.Get(strings.ToLower(name))
}
func (s *Session) OpenShop(file string) bool {
	key := strings.ToLower(file)
	if s.ShopMains.Has(key) {
		s.FireHandler(s.ShopMains.Get(key), "openshop", key, "openshop "+key+" (re-entry)")
		return true
	}
	data, err := s.Read(key)
	if err != nil || data == nil {
		s.Log(fmt.Sprintf("openshopfile: %q not available", file))
		return false
	}
	shop, err := df.ReadShop(data)
	if err != nil {
		s.Log(fmt.Sprintf("openshopfile: %s: %v", file, err))
		return false
	}
	loaded := s.Props.AddShop(key, shop)
	switch key {
	case "inven.shp", "house.shp", "inven.prp", "house.prp":
		loaded.Persistent = true
	}
	main := s.InstanceFrom(shop.File.Data(shop.MainScriptLocation), key)
	s.ShopMains.Set(key, main)
	for _, g := range shop.Groups {
		inst := s.InstanceFrom(shop.File.Data(g.ScriptContainerLocation), g.Name)
		if inst != nil {
			inst.Parent = main
			s.PropScripts.Set(strings.ToLower(g.Name), inst)
		}
	}
	for _, g := range shop.Groups {
		s.FireHandler(s.PropScripts.Get(strings.ToLower(g.Name)), "openprop", g.Name, "openprop "+g.Name)
	}
	s.FireHandler(main, "openshop", key, "openshop "+key)
	s.Log(fmt.Sprintf("shop loaded: %s (%d props)", key, len(shop.Groups)))
	return true
}
func (s *Session) CloseShop(file string) {
	key := strings.ToLower(file)
	if main := s.ShopMains.Get(key); main != nil {
		_, _ = s.Interp.Run(main, "closeshop", nil, script.CallContext{Me: key}, nil)
	}
	s.ShopMains.Delete(key)
	if shop := s.Props.Shops.Get(key); shop != nil {
		for _, g := range shop.Shp.Groups {
			s.PropScripts.Delete(strings.ToLower(g.Name))
		}
	}
	s.Props.RemoveShop(key)
}
func (s *Session) LoadBootResources() {
	for _, file := range s.BootPlan().Resources {
		data, err := s.Read(file)
		if err != nil || data == nil {
			continue
		}
		switch {
		case strings.HasSuffix(file, ".trk"):
			_ = s.AudioLib.OpenBank(file, data)
		case strings.HasSuffix(file, ".shp"):
			s.OpenShop(file)
		case strings.HasSuffix(file, ".cst"):
			s.OpenCastFile(file)
		case strings.HasSuffix(file, ".stg"):
			s.StageCtrl.OpenStageFile(file)
		}
	}
}
