package engine

import (
	"github.com/itskenny0/titanic-godot/internal/script"
	"reflect"
	"testing"
)

func dispatchInstance(name, handler, ending string) *script.Instance {
	body := []script.Stmt{{Kind: "call", Value: &script.Expr{Kind: "call", Text: "record"}}}
	if ending != "" {
		body = append(body, script.Stmt{Kind: ending})
	}
	return &script.Instance{Name: name, Script: &script.Script{Handlers: map[string]*script.Handler{handler: {Name: handler, Body: body}}, Order: []string{handler}}}
}
func TestEventDispatchContainmentAndTarget(t *testing.T) {
	interp := script.NewInterpreter()
	props, actors := new(PropRuntime), new(ActorRuntime)
	d := NewScriptDispatch(interp, props, actors)
	var calls [][3]string
	interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		calls = append(calls, [3]string{f.Instance.Name, f.Context.Me, f.Context.Target})
		return script.Num(0), nil
	})
	props.AddShop("room.shp", testShop())
	props.Instance("door", "DoorCopy")
	main := dispatchInstance("room.shp", "mousedown", "")
	door := dispatchInstance("door", "mousedown", "passcode")
	door.Parent = main
	d.PropScripts.Set("door", door)
	d.ShopMains.Set("room.shp", main)
	d.StageScript = dispatchInstance("main.stg", "mousedown", "")
	d.BootScripts = []*script.Instance{dispatchInstance("boot1", "mousedown", "")}
	d.RefreshFallbacks()
	if _, err := d.SendEvent("sendtoprop", "DoorCopy", "mousedown", nil, "pointer", nil); err != nil {
		t.Fatal(err)
	}
	want := [][3]string{{"door", "DoorCopy", "DoorCopy"}, {"room.shp", "door", "DoorCopy"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatal("prop copy or parent context differs", calls)
	}
	calls = nil
	actor := dispatchInstance("cast-door", "mousedown", "")
	d.CastScripts.Set("door", actor)
	if _, err := d.SendEvent("sendtoactor", "door", "mousedown", nil, "pointer", nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0][0] != "cast-door" {
		t.Fatal("actor dispatch resolved a prop with the same name", calls)
	}
	calls = nil
	if _, err := d.SendEvent("sendtoprop", "absent", "mousedown", nil, "pointer", nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0][0] != "boot1" {
		t.Fatal("missing object did not reach boot fallback", calls)
	}
	if d.FindGlobalInstance("ROOM") != main {
		t.Fatal("extension-free lookup failed")
	}
}
func TestKeyDispatchAndConsumption(t *testing.T) {
	interp := script.NewInterpreter()
	d := NewScriptDispatch(interp, new(PropRuntime), new(ActorRuntime))
	calls := []string{}
	interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		calls = append(calls, f.Instance.Name)
		return script.Num(0), nil
	})
	d.StageScript = dispatchInstance("stage", "keydown", "")
	d.BootScripts = []*script.Instance{dispatchInstance("boot1", "keydown", ""), dispatchInstance("boot2", "keydown", "")}
	d.RefreshFallbacks()
	if _, err := d.SendEvent("sendtostage", "stage", "keydown", nil, "keyboard", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"stage", "boot1", "boot2"}) {
		t.Fatal("handled key stopped before boot libraries", calls)
	}
	calls = nil
	d.StageScript.Script.Handlers["keydown"].Body = append(d.StageScript.Script.Handlers["keydown"].Body, script.Stmt{Kind: "exitcode"})
	if _, err := d.SendEvent("sendtostage", "stage", "keydown", nil, "keyboard", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"stage"}) {
		t.Fatal("consumed key escaped into fallback scripts", calls)
	}
}
func TestStageFallbackSuppression(t *testing.T) {
	interp := script.NewInterpreter()
	d := NewScriptDispatch(interp, new(PropRuntime), new(ActorRuntime))
	calls := []string{}
	interp.Register("record", func(_ *script.Interpreter, _ []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		calls = append(calls, f.Instance.Name)
		return script.Num(0), nil
	})
	d.StageScript = dispatchInstance("stage", "openstage", "passcode")
	d.BootScripts = []*script.Instance{dispatchInstance("boot1", "openstage", "")}
	d.SuppressStageBootFallback = true
	if _, err := d.SendEvent("sendtostage", "stage", "openstage", nil, "stage", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"stage"}) {
		t.Fatal("stage initialization re-entered boot", calls)
	}
}
