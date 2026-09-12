package engine

import (
	"testing"

	"github.com/itskenny0/titanic-godot/internal/script"
)

func TestInterpreterContinuationsShareStateSerially(t *testing.T) {
	e := NewExecutor()
	defer e.Close()
	vm := script.NewInterpreter()
	count := func() float64 { v, _ := vm.Globals.Get("count"); return v.Num() }
	increment := script.Stmt{Kind: "assign", Name: "count", Value: &script.Expr{Kind: "binary", Text: "+", Left: &script.Expr{Kind: "variable", Text: "count"}, Right: &script.Expr{Kind: "number", Number: 1}}}
	inst := &script.Instance{Name: "scene", Script: &script.Script{Handlers: map[string]*script.Handler{"main": {Name: "main", Body: []script.Stmt{{Kind: "global", Names: []string{"count"}}, increment, {Kind: "call", Value: &script.Expr{Kind: "call", Text: "wait", Args: []*script.Expr{{Kind: "number", Number: 50}}}}, increment}}}}}
	if err := vm.Register("wait", func(_ *script.Interpreter, args []script.Value, _ *script.Expr, _ *script.Frame) (script.Value, error) {
		e.Current().Sleep(args[0].Num())
		return script.Value{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	start := func() *Task {
		return e.Start("scene.main", func(task *Task) error { _, err := vm.Run(inst, "main", nil, script.CallContext{}, nil); return err })
	}
	a, b := start(), start()
	e.Pump(0, true, 100)
	if a.Done() || b.Done() || count() != 2 || !vm.IsRunning(inst, "main") {
		t.Fatalf("suspended state: %+v %+v %v", a.Err(), b.Err(), count())
	}
	e.Pump(50, true, 100)
	if a.Err() != nil || b.Err() != nil || !a.Done() || !b.Done() || count() != 4 || vm.IsRunning(inst, "main") {
		t.Fatal("continuation state was lost")
	}
	start()
	e.Pump(50, true, 100)
	vm.Abandon()
	e.CancelAll()
	if vm.IsRunning(inst, "main") {
		t.Fatal("cancelled script retained a live handler")
	}
}
