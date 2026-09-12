package script

import (
	"fmt"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

type literal string

func fixture(t *testing.T, parts ...any) *Instance {
	t.Helper()
	tokens := make([]df.Token, 0, len(parts))
	ops := make(map[string]uint16)
	for id, name := range df.Opcodes {
		ops[name] = id
	}
	for _, p := range parts {
		var token df.Token
		switch v := p.(type) {
		case int:
			token = df.Token{Kind: df.TokenInteger, Number: uint32(v)}
		case literal:
			token = df.Token{Kind: df.TokenString, Text: string(v)}
		case string:
			if v == "\n" {
				token = df.Token{Kind: df.TokenBreak}
			} else if id, ok := ops[v]; ok {
				token = df.Token{Kind: df.TokenOpcode, Text: v, Opcode: id}
			} else {
				token = df.Token{Kind: df.TokenVariable, Text: v}
			}
		default:
			t.Fatalf("bad fixture token %v", v)
		}
		tokens = append(tokens, token)
	}
	s, err := Parse(tokens)
	if err != nil {
		t.Fatal(err)
	}
	return &Instance{Name: "fixture", Script: s}
}
func run(t *testing.T, i *Interpreter, s *Instance, name string, args ...Value) Result {
	t.Helper()
	r, err := i.Run(s, name, args, CallContext{Me: "door", Target: "handle"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestInterpreterScopeControlAndCoercion(t *testing.T) {
	s := fixture(t,
		"code", "Main", "(", "arg", ")", "\n",
		"global", "total", "\n", "total", "=", 0, "\n",
		"for", "counter", "=", 3, "to", 1, "step", "-", 1, "\n", "total", "=", "total", "+", "counter", "\n", "endfor", "\n",
		"if", 0, "&", "error", "(", literal("must not run"), ")", "\n", "return", 99, "\n", "endif", "\n",
		"switch", "arg", "\n", "case", literal("DECKA"), "\n", "case", literal("deckbd"), "\n",
		"return", "total", "@", literal(":"), "@", "me", "@", literal(":"), "@", "target", "\n", "endswitch", "\n", "return", 0, "\n", "endcode",
		"code", "cleanup", "(", ")", "\n", "dumpglobal", "total", "\n", "endcode")
	i := NewInterpreter()
	if err := i.Register("error", func(*Interpreter, []Value, *Expr, *Frame) (Value, error) {
		return Value{}, fmt.Errorf("short circuit failed")
	}); err != nil {
		t.Fatal(err)
	}
	r := run(t, i, s, "main", Str("decka"))
	if r.Value != Str("6:door:handle") || r.Passed || !r.Handled {
		t.Fatalf("result: %+v", r)
	}
	if v, ok := i.Globals.Get("TOTAL"); !ok || v != Num(6) {
		t.Fatal("global assignment/case", v)
	}
	if i.Globals.Has("counter") {
		t.Fatal("undeclared assignment escaped locals")
	}
	run(t, i, s, "cleanup")
	if i.Globals.Len() != 0 {
		t.Fatal("dumpglobal retained variable")
	}
	if Equal(Str("uparrow"), Num(0)) || !Equal(Str("HELLO"), Str("hello")) || Str("-17px").Num() != -17 || Str("0x10").Num() != 0 {
		t.Fatal("value coercion differs")
	}
}
func TestInterpreterDispatchConsumption(t *testing.T) {
	s := fixture(t,
		"code", "keydown", "(", ")", "\n", "helper", "(", ")", "\n", "passcode", "\n", "endcode",
		"code", "helper", "(", ")", "\n", "exitcode", "\n", "endcode",
		"code", "mousedown", "(", ")", "\n", "sendtoscene", "(", literal("room"), ",", "mousedown", "(", ")", ")", "\n", "endcode")
	child := fixture(t, "code", "mousedown", "(", ")", "\n", "exitcode", "\n", "endcode")
	i := NewInterpreter()
	r := run(t, i, s, "keydown")
	if r.Consumed || !r.Passed {
		t.Fatal("helper consumed unrelated event", r)
	}
	i.SpecialForms["sendtoscene"] = func(i *Interpreter, args []*Expr, f *Frame) (Value, error) {
		result, err := i.Run(child, args[1].Text, nil, f.Context, f)
		return result.Value, err
	}
	r = run(t, i, s, "mousedown")
	if !r.Consumed {
		t.Fatal("routed event did not consume")
	}
	if i.IsRunning(s, "mousedown") || i.IsRunning(child, "mousedown") {
		t.Fatal("finished handler still live")
	}
}
func TestInterpreterAbandonAndRunaway(t *testing.T) {
	s := fixture(t, "code", "main", "(", ")", "\n", "global", "state", "\n", "state", "=", 1, "\n", "delay", "(", 1, ")", "\n", "state", "=", 2, "\n", "endcode")
	i := NewInterpreter()
	_ = i.Register("delay", func(i *Interpreter, _ []Value, _ *Expr, _ *Frame) (Value, error) { i.Abandon(); return Value{}, nil })
	run(t, i, s, "main")
	v, _ := i.Globals.Get("state")
	if v != Num(1) {
		t.Fatal("abandoned script changed replacement game")
	}
	loop := fixture(t, "code", "main", "(", ")", "\n", "while", "true", "\n", "endwhile", "\n", "endcode")
	if _, err := i.Run(loop, "main", nil, CallContext{}, nil); err == nil {
		t.Fatal("runaway accepted")
	}
	if i.IsRunning(loop, "main") {
		t.Fatal("failed handler still live")
	}
}
func TestParserKeepsHandlersAfterMissingClosers(t *testing.T) {
	s := fixture(t, "code", "first", "(", ")", "\n", "if", "true", "\n", "exitcode", "\n", "code", "second", "(", ")", "\n", "return", 7, "\n", "endcode")
	if len(s.Script.Order) != 2 {
		t.Fatal("missing closer swallowed next handler")
	}
	r := run(t, NewInterpreter(), s, "SECOND")
	if r.Value != Num(7) {
		t.Fatal(r)
	}
}
