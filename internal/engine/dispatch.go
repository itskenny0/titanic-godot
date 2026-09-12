package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"regexp"
	"slices"
	"strings"
)

// ScriptBinding is the script hierarchy of the currently loaded room.
type ScriptBinding interface {
	FindInstance(string) *script.Instance
	MainScript() *script.Instance
}
type ScriptDispatch struct {
	Interp                                                                     *script.Interpreter
	Props                                                                      *PropRuntime
	Actors                                                                     *ActorRuntime
	BootScripts                                                                []*script.Instance
	StageScript                                                                *script.Instance
	SuppressStageBootFallback                                                  bool
	Binding                                                                    ScriptBinding
	PuppetScripts, PropScripts, CastScripts, ShopMains, CastMains, FlatScripts orderedMap[*script.Instance]
	Log                                                                        func(string)
}

func NewScriptDispatch(interp *script.Interpreter, props *PropRuntime, actors *ActorRuntime) *ScriptDispatch {
	return &ScriptDispatch{Interp: interp, Props: props, Actors: actors, Log: func(string) {}}
}
func hasHandler(inst *script.Instance, handler string) bool {
	return inst != nil && inst.Script != nil && inst.Script.Handlers[handler] != nil
}
func (d *ScriptDispatch) Boot() *script.Instance {
	if len(d.BootScripts) > 0 {
		return d.BootScripts[0]
	}
	return nil
}
func (d *ScriptDispatch) RefreshFallbacks() {
	d.Interp.Fallbacks = nil
	if d.StageScript != nil {
		d.Interp.Fallbacks = append(d.Interp.Fallbacks, d.StageScript)
	}
	d.Interp.Fallbacks = append(d.Interp.Fallbacks, d.BootScripts...)
}
func (d *ScriptDispatch) InstanceFrom(data []byte, owner string) *script.Instance {
	if len(data) == 0 {
		return nil
	}
	tokens, err := df.DecodeScript(data)
	if err != nil || len(tokens) == 0 {
		return nil
	}
	ast, err := script.Parse(tokens)
	if err != nil {
		d.Log(fmt.Sprintf("parse error in %s: %v", owner, err))
		return nil
	}
	return &script.Instance{Name: owner, Script: ast}
}

var scriptExtension = regexp.MustCompile(`\.[a-z0-9]{1,4}$`)
var actorAddressee = regexp.MustCompile(`^sendtoactor(fx)?$`)
var propAddressee = regexp.MustCompile(`^sendtoprop(fx)?$`)
var bootAddressee = regexp.MustCompile(`^sendto(boot|post)(fx)?$`)
var objectAddressee = regexp.MustCompile(`^sendto(prop|actor|scene|flat)(fx)?$`)

func (d *ScriptDispatch) propScriptFor(name string) *script.Instance {
	if own := d.PropScripts.Get(name); own != nil {
		return own
	}
	if p := d.Props.Get(name); p != nil {
		group := strings.ToLower(p.Group.Name)
		if group != name {
			return d.PropScripts.Get(group)
		}
	}
	return nil
}
func (d *ScriptDispatch) FindGlobalInstance(name string) *script.Instance {
	lower := strings.ToLower(name)
	for _, inst := range []*script.Instance{d.PuppetScripts.Get(lower), d.propScriptFor(lower), d.CastScripts.Get(lower), d.ShopMains.Get(lower), d.CastMains.Get(lower), d.FlatScripts.Get(lower)} {
		if inst != nil {
			return inst
		}
	}
	if d.StageScript != nil && strings.ToLower(d.StageScript.Name) == lower {
		return d.StageScript
	}
	if lower == "boot" && d.Boot() != nil {
		return d.Boot()
	}
	want := scriptExtension.ReplaceAllString(lower, "")
	for _, table := range []*orderedMap[*script.Instance]{&d.ShopMains, &d.CastMains, &d.FlatScripts} {
		for key, inst := range table.All() {
			if scriptExtension.ReplaceAllString(strings.ToLower(key), "") == want {
				return inst
			}
		}
	}
	return nil
}
func (d *ScriptDispatch) resolveTarget(cmd, target, handler string) *script.Instance {
	var inst *script.Instance
	if cmd == "sendtoflat" {
		flat := d.FlatScripts.Get(strings.ToLower(target))
		if hasHandler(flat, handler) {
			inst = flat
		}
	}
	if inst == nil && actorAddressee.MatchString(cmd) {
		inst = d.CastScripts.Get(strings.ToLower(target))
	}
	if inst == nil && d.Binding != nil {
		inst = d.Binding.FindInstance(target)
	}
	if inst == nil {
		inst = d.FindGlobalInstance(target)
	}
	if inst == nil && cmd == "sendtostage" {
		inst = d.StageScript
	}
	if bootAddressee.MatchString(cmd) && !hasHandler(inst, handler) {
		for _, b := range d.BootScripts {
			if hasHandler(b, handler) {
				return b
			}
		}
		if inst == nil {
			inst = d.Boot()
		}
	}
	if inst == nil && cmd == "sendtoflat" {
		inst = d.StageScript
	}
	if inst == nil && cmd == "sendtoprop" {
		if p := d.Props.Get(target); p != nil {
			inst = d.ShopMains.Get(p.Shop.Name)
		}
	}
	if inst == nil && cmd == "sendtoactor" {
		if a := d.Actors.Get(target); a != nil {
			main := d.CastMains.Get(strings.ToLower(a.Cast.Name))
			if hasHandler(main, handler) {
				inst = main
			}
		}
	}
	return inst
}
func (d *ScriptDispatch) eventChain(cmd string, inst *script.Instance, handler string) []*script.Instance {
	chain := []*script.Instance{}
	if inst != nil {
		chain = append(chain, inst)
	}
	if cmd == "sendtoscene" || cmd == "sendtoset" {
		if d.Binding != nil {
			main := d.Binding.MainScript()
			if main != nil && main != inst {
				chain = append(chain, main)
			}
		}
		if d.StageScript != nil && d.StageScript != inst {
			chain = append(chain, d.StageScript)
		}
	}
	if cmd == "sendtostage" && !d.SuppressStageBootFallback {
		for _, b := range d.BootScripts {
			if b != inst && !slices.Contains(chain, b) {
				chain = append(chain, b)
			}
		}
	}
	if len(chain) == 0 && (cmd == "sendtoprop" || cmd == "sendtoactor") {
		for _, b := range d.BootScripts {
			if hasHandler(b, handler) && !d.Interp.IsRunning(b, handler) {
				chain = append(chain, b)
			}
		}
	}
	if cmd == "sendtoflat" && !slices.ContainsFunc(chain, func(i *script.Instance) bool { return hasHandler(i, handler) }) {
		for _, b := range d.BootScripts {
			if hasHandler(b, handler) && !d.Interp.IsRunning(b, handler) && !slices.Contains(chain, b) {
				chain = append(chain, b)
			}
		}
	}
	return chain
}

type HandlerChainResult struct {
	Value       script.Value
	Ran, Passed bool
	Visited     []*script.Instance
}

func (d *ScriptDispatch) RunChain(chain []*script.Instance, handler string, args []script.Value, context func(*script.Instance) script.CallContext, stopOnHandled bool, parent *script.Frame) (HandlerChainResult, error) {
	result := HandlerChainResult{Visited: []*script.Instance{}}
	for _, inst := range chain {
		if !hasHandler(inst, handler) {
			continue
		}
		result.Ran = true
		result.Visited = append(result.Visited, inst)
		res, err := d.Interp.Run(inst, handler, args, context(inst), parent)
		if err != nil {
			return result, err
		}
		result.Value, result.Passed = res.Value, res.Passed
		if res.Consumed || stopOnHandled && res.Handled && !res.Passed {
			break
		}
	}
	return result, nil
}
func (d *ScriptDispatch) SendEvent(cmd, target, handler string, args []script.Value, caller string, parent *script.Frame) (script.Value, error) {
	inst := d.resolveTarget(cmd, target, handler)
	chain := d.eventChain(cmd, inst, handler)
	if len(chain) == 0 {
		reason := "target not loaded"
		if inst != nil {
			reason = "no " + handler + " handler on " + inst.Name
		}
		d.Log(fmt.Sprintf("%s(%q, %s(..)): %s", cmd, target, handler, reason))
		return script.Num(0), nil
	}
	eventTarget := caller
	if objectAddressee.MatchString(cmd) {
		eventTarget = target
		if eventTarget == "" {
			eventTarget = caller
		}
	}
	keyEvent := handler == "keydown" || handler == "keyrepeat"
	if keyEvent {
		for _, b := range d.BootScripts {
			if !slices.Contains(chain, b) && !d.Interp.IsRunning(b, handler) {
				chain = append(chain, b)
			}
		}
	}
	propOwn := propAddressee.MatchString(cmd) && eventTarget != ""
	result, err := d.RunChain(chain, handler, args, func(link *script.Instance) script.CallContext {
		me := link.Name
		if propOwn && link == inst {
			me = eventTarget
		}
		return script.CallContext{Me: me, Target: eventTarget}
	}, !keyEvent, parent)
	if err != nil {
		return script.Value{}, err
	}
	if (!result.Ran || result.Passed) && inst != nil {
		libs := []*script.Instance{}
		seen := map[*script.Instance]bool{}
		for p := inst.Parent; p != nil && !seen[p]; p = p.Parent {
			seen[p] = true
			libs = append(libs, p)
		}
		if d.StageScript != nil && d.StageScript != inst {
			libs = append(libs, d.StageScript)
		}
		if (cmd == "sendtoactor" || cmd == "sendtoprop") && (parent == nil || parent.Dispatch != handler) {
			for _, b := range d.BootScripts {
				if !slices.Contains(libs, b) {
					libs = append(libs, b)
				}
			}
		}
		libs = slices.DeleteFunc(libs, func(i *script.Instance) bool { return slices.Contains(result.Visited, i) })
		fallback, err := d.RunChain(libs, handler, args, func(*script.Instance) script.CallContext {
			return script.CallContext{Me: inst.Name, Target: eventTarget}
		}, true, parent)
		if err != nil {
			return script.Value{}, err
		}
		if fallback.Ran {
			return fallback.Value, nil
		}
	}
	return result.Value, nil
}
func (d *ScriptDispatch) HasGlobal(handler string) bool {
	return slices.ContainsFunc(d.Interp.Fallbacks, func(i *script.Instance) bool { return hasHandler(i, handler) })
}
func (d *ScriptDispatch) RunGlobal(handler string, args []script.Value) (script.Value, error) {
	for _, inst := range d.Interp.Fallbacks {
		if hasHandler(inst, handler) {
			res, err := d.Interp.Run(inst, handler, args, script.CallContext{Me: inst.Name}, nil)
			return res.Value, err
		}
	}
	d.Log("runGlobal: no handler " + handler)
	return script.Num(0), nil
}
