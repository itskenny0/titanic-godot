package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"strings"
)

type SetScripts struct {
	Set            *df.Set
	Session        *Session
	Main           *script.Instance
	Scenes         []*script.Instance
	Objects        orderedMap[*script.Instance]
	LastSceneIdx   int
	lifecycleDepth int
	Log            func(string)
}

func NewSetScripts(set *df.Set, session *Session) *SetScripts {
	b := &SetScripts{Set: set, Session: session, LastSceneIdx: -1, Log: func(string) {}}
	session.CurrentBinding = b
	session.Binding = b
	name := strings.ToLower(set.SetName)
	if name == "" {
		name = session.CurrentSetFile
	}
	session.SetCurrentSetName(name)
	session.Interp.OnUnknown = func(name string, args []script.Value) {
		parts := []string{}
		for _, arg := range args {
			parts = append(parts, arg.String())
		}
		b.Log("? " + name + "(" + strings.Join(parts, ", ") + ")")
	}
	instance := func(loc int, name string) *script.Instance {
		if loc == 0 {
			return nil
		}
		return session.InstanceFrom(set.File.Data(loc), name)
	}
	b.Main = instance(set.MainScript, set.SetName)
	own := func(inst *script.Instance) *script.Instance {
		if inst != nil {
			inst.Parent = b.Main
		}
		return inst
	}
	for _, scene := range set.Scenes {
		b.Scenes = append(b.Scenes, own(instance(scene.LocationScript, scene.SceneName)))
		for v, view := range scene.Views {
			for o, object := range view.Objects {
				inst := own(instance(object.LocationScript, object.Identifier))
				if inst != nil {
					b.Objects.Set(fmt.Sprintf("%d:%d:%d", scene.Index, v, o), inst)
				}
			}
		}
	}
	return b
}
func (b *SetScripts) MainScript() *script.Instance { return b.Main }
func (b *SetScripts) InLifecycle() bool            { return b.lifecycleDepth > 0 }
func (b *SetScripts) scene(index int) *script.Instance {
	if index < 0 || index >= len(b.Scenes) {
		return nil
	}
	return b.Scenes[index]
}
func (b *SetScripts) Fire(inst *script.Instance, handler string, args []script.Value, target string) *script.Value {
	if inst == nil {
		return nil
	}
	res, err := b.Session.Interp.Run(inst, handler, args, script.CallContext{Me: inst.Name, Target: target}, nil)
	if err != nil {
		b.Log(fmt.Sprintf("script error in %s.%s: %v", inst.Name, handler, err))
		return nil
	}
	if res.Handled && !res.Passed {
		return &res.Value
	}
	return nil
}
func (b *SetScripts) fireLifecycle(handler string, sceneIdx int) {
	if b.Session.RestoringSave {
		return
	}
	b.lifecycleDepth++
	defer func() { b.lifecycleDepth-- }()
	chain := append([]*script.Instance{b.scene(sceneIdx), b.Main, b.Session.StageScript}, b.Session.BootScripts...)
	for _, inst := range chain {
		if inst == nil {
			continue
		}
		res, err := b.Session.Interp.Run(inst, handler, nil, script.CallContext{Me: inst.Name}, nil)
		if err != nil {
			b.Log(fmt.Sprintf("script error in %s.%s: %v", inst.Name, handler, err))
			continue
		}
		if res.Handled && !res.Passed {
			return
		}
	}
}
func (b *SetScripts) OpenSet() { b.fireLifecycle("openset", -1) }
func (b *SetScripts) CloseSet() {
	if b.LastSceneIdx >= 0 {
		b.CloseScene(b.LastSceneIdx)
	}
	b.fireLifecycle("closeset", -1)
}
func (b *SetScripts) OpenScene(index int) {
	b.LastSceneIdx = index
	b.fireLifecycle("openscene", index)
}
func (b *SetScripts) CloseScene(index int) { b.fireLifecycle("closescene", index); b.LastSceneIdx = -1 }
func (b *SetScripts) ViewChanged(sceneIdx int) {
	b.lifecycleDepth++
	defer func() { b.lifecycleDepth-- }()
	for _, inst := range b.Session.BootScripts {
		res, err := b.Session.Interp.Run(inst, "closescene", nil, script.CallContext{Me: inst.Name}, nil)
		if err != nil {
			b.Log(fmt.Sprintf("script error in %s.closescene: %v", inst.Name, err))
			continue
		}
		if res.Consumed {
			return
		}
	}
	b.ViewSettled(sceneIdx)
}
func (b *SetScripts) ViewSettled(sceneIdx int) {
	b.lifecycleDepth++
	defer func() { b.lifecycleDepth-- }()
	for _, inst := range append([]*script.Instance{b.scene(sceneIdx), b.Main, b.Session.StageScript}, b.Session.BootScripts...) {
		if inst == nil {
			continue
		}
		res, err := b.Session.Interp.Run(inst, "openscene", nil, script.CallContext{Me: inst.Name}, nil)
		if err != nil {
			b.Log(fmt.Sprintf("script error in %s.openscene (view change): %v", inst.Name, err))
			continue
		}
		if res.Handled && !res.Passed {
			return
		}
	}
}
func (b *SetScripts) ObjectScript(scene, view, object int) *script.Instance {
	return b.Objects.Get(fmt.Sprintf("%d:%d:%d", scene, view, object))
}
func (b *SetScripts) PaintingEvent(sceneName, viewName, paintName, handler string, args []script.Value, parent *script.Frame) bool {
	for si, scene := range b.Set.Scenes {
		if !strings.EqualFold(scene.SceneName, sceneName) {
			continue
		}
		for vi, view := range scene.Views {
			if !strings.EqualFold(view.ViewName, viewName) {
				continue
			}
			for oi, object := range view.Objects {
				if strings.EqualFold(object.Identifier, paintName) {
					return b.fireChain(si, vi, oi, handler, paintName, args, parent)
				}
			}
			return false
		}
		return false
	}
	return false
}
func (b *SetScripts) fireChain(scene, view, object int, handler, identifier string, args []script.Value, parent *script.Frame) bool {
	for _, inst := range []*script.Instance{b.ObjectScript(scene, view, object), b.scene(scene), b.Main, b.Session.StageScript} {
		if inst == nil {
			continue
		}
		res, err := b.Session.Interp.Run(inst, handler, args, script.CallContext{Me: inst.Name, Target: identifier}, parent)
		if err != nil {
			b.Log(fmt.Sprintf("script error in %s.%s: %v", inst.Name, handler, err))
			continue
		}
		if res.Consumed || res.Handled && !res.Passed {
			return true
		}
	}
	return false
}
func (b *SetScripts) MouseDown(scene, view, object int, identifier string) bool {
	return b.fireChain(scene, view, object, "mousedown", identifier, []script.Value{script.Str(identifier)}, nil)
}
func (b *SetScripts) SetCursor(scene, view, object int, identifier string) string {
	b.Session.CursorName = ""
	b.fireChain(scene, view, object, "setcursor", identifier, []script.Value{script.Str(identifier)}, nil)
	return b.Session.CursorName
}
func (b *SetScripts) KeyDown(scene int, key string) bool {
	chain := []*script.Instance{b.scene(scene), b.Main}
	for _, inst := range b.Session.BootScripts {
		if hasHandler(inst, "keydown") {
			chain = []*script.Instance{inst}
			break
		}
	}
	for _, inst := range chain {
		if inst == nil {
			continue
		}
		res, err := b.Session.Interp.Run(inst, "keydown", []script.Value{script.Str(key)}, script.CallContext{Me: inst.Name, Target: key}, nil)
		if err != nil {
			b.Log(fmt.Sprintf("script error in %s.keydown: %v", inst.Name, err))
			continue
		}
		if res.Consumed {
			return true
		}
	}
	return false
}
func (b *SetScripts) FindInstance(name string) *script.Instance {
	lower := strings.ToLower(name)
	s := b.Session
	if prop := s.PropScripts.Get(lower); prop != nil {
		return prop
	}
	if b.Main != nil && strings.ToLower(b.Main.Name) == lower {
		return b.Main
	}
	for i, scene := range b.Set.Scenes {
		if strings.ToLower(scene.SceneName) == lower {
			return b.scene(i)
		}
	}
	for _, inst := range b.Objects.All() {
		if strings.ToLower(inst.Name) == lower {
			return inst
		}
	}
	if s.StageScript != nil && strings.ToLower(s.StageScript.Name) == lower {
		return s.StageScript
	}
	if shop := s.ShopMain(lower); shop != nil {
		return shop
	}
	if flat := s.FlatScripts.Get(lower); flat != nil {
		return flat
	}
	if lower == "boot" {
		return s.Boot()
	}
	return nil
}
