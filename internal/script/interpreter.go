package script

import (
	"fmt"
	"math"
	"strings"
)

type Instance struct {
	Name   string
	Script *Script
	Parent *Instance
}
type CallContext struct{ Me, Target string }
type chain struct{ consumed bool }
type Frame struct {
	Instance          *Instance
	Context           CallContext
	Handler, Dispatch string
	Locals            Values
	Epoch, Event      uint64
	depth             int
	chain             *chain
}
type Signal struct {
	Kind  string
	Value Value
}
type Result struct {
	Value                     Value
	Passed, Handled, Consumed bool
}
type Builtin func(*Interpreter, []Value, *Expr, *Frame) (Value, error)
type SpecialForm func(*Interpreter, []*Expr, *Frame) (Value, error)

// The host schedules calls cooperatively. A builtin may yield through the host
// scheduler, but simultaneous goroutines must not mutate an Interpreter.
type Interpreter struct {
	Globals         Values
	Builtins        map[string]Builtin
	SpecialForms    map[string]SpecialForm
	Fallbacks       []*Instance
	WatchGlobals    map[string]bool
	OnGlobalChange  func(string, Value, Value)
	OnUnknown       func(string, []Value)
	RealYieldSeq    func() uint64
	CurrentEvent    uint64
	epoch, sequence uint64
	live            map[*Instance]map[string]int
	unknown         map[string]bool
}

func NewInterpreter() *Interpreter {
	return &Interpreter{Builtins: make(map[string]Builtin), SpecialForms: make(map[string]SpecialForm), WatchGlobals: make(map[string]bool), live: make(map[*Instance]map[string]int), unknown: make(map[string]bool), RealYieldSeq: func() uint64 { return 0 }}
}
func (i *Interpreter) Register(name string, b Builtin) error {
	name = strings.ToLower(name)
	if _, exists := i.Builtins[name]; exists {
		return fmt.Errorf("builtin registered twice: %s", name)
	}
	i.Builtins[name] = b
	return nil
}
func (i *Interpreter) IsRunning(inst *Instance, handler string) bool {
	return i.live[inst][strings.ToLower(handler)] > 0
}
func (i *Interpreter) Abandon() { i.epoch++ }
func (i *Interpreter) Run(inst *Instance, handler string, args []Value, ctx CallContext, parent *Frame) (Result, error) {
	h := inst.Script.Handlers[strings.ToLower(handler)]
	if h == nil {
		return Result{Passed: true}, nil
	}
	f := &Frame{Instance: inst, Context: ctx, Handler: handler, Dispatch: handler, Epoch: i.epoch, chain: &chain{}}
	if parent != nil {
		f.Dispatch = parent.Dispatch
		f.chain = parent.chain
		f.depth = parent.depth + 1
	}
	if f.depth >= 64 {
		return Result{}, fmt.Errorf("dispatch cycle: %s.%s at depth %d", inst.Name, handler, f.depth)
	}
	for n, name := range h.Params {
		v := Value{}
		if n < len(args) {
			v = args[n]
		}
		f.Locals.Set(name, v)
	}
	i.sequence++
	f.Event = i.sequence
	previousEvent := i.CurrentEvent
	i.CurrentEvent = f.Event
	defer func() { i.CurrentEvent = previousEvent }()
	key := strings.ToLower(handler)
	if i.live[inst] == nil {
		i.live[inst] = make(map[string]int)
	}
	i.live[inst][key]++
	defer func() {
		i.live[inst][key]--
		if i.live[inst][key] == 0 {
			delete(i.live[inst], key)
		}
		if len(i.live[inst]) == 0 {
			delete(i.live, inst)
		}
	}()
	sig, err := i.Block(h.Body, f)
	if err != nil {
		return Result{}, err
	}
	result := Result{Handled: true, Passed: sig.Kind == "passcode", Consumed: f.chain.consumed}
	if sig.Kind == "return" {
		result.Value = sig.Value
	}
	return result, nil
}
func (i *Interpreter) Block(stmts []Stmt, f *Frame) (Signal, error) {
	for _, s := range stmts {
		if f.Epoch != i.epoch {
			return Signal{Kind: "abandoned"}, nil
		}
		sig, err := i.statement(s, f)
		if err != nil || sig.Kind != "" {
			return sig, err
		}
	}
	return Signal{}, nil
}
func (i *Interpreter) statement(s Stmt, f *Frame) (Signal, error) {
	normal := Signal{}
	switch s.Kind {
	case "noop":
		return normal, nil
	case "global", "dumpglobal", "local", "dumplocal":
		for _, name := range s.Names {
			switch s.Kind {
			case "global":
				if !i.Globals.Has(name) {
					i.Globals.Set(name, Value{})
				}
			case "dumpglobal":
				i.Globals.Delete(name)
			default:
				if !f.Locals.Has(name) {
					f.Locals.Set(name, Value{})
				}
			}
		}
		return normal, nil
	case "assign":
		v, err := i.Eval(s.Value, f)
		if err == nil {
			i.SetVar(s.Name, v, f)
		}
		return normal, err
	case "call":
		_, err := i.Call(s.Value, f)
		return normal, err
	case "if":
		v, err := i.Eval(s.Condition, f)
		if err != nil {
			return normal, err
		}
		if v.Truthy() {
			return i.Block(s.Body, f)
		}
		return i.Block(s.Else, f)
	case "switch":
		v, err := i.Eval(s.Value, f)
		if err != nil {
			return normal, err
		}
		for n, c := range s.Cases {
			match, err := i.Eval(c.Match, f)
			if err != nil {
				return normal, err
			}
			if Equal(v, match) {
				for n < len(s.Cases)-1 && len(s.Cases[n].Body) == 0 {
					n++
				}
				return i.Block(s.Cases[n].Body, f)
			}
		}
	case "while":
		guard := 0
		lastYield := i.RealYieldSeq()
		for {
			v, err := i.Eval(s.Condition, f)
			if err != nil {
				return normal, err
			}
			if !v.Truthy() {
				break
			}
			if f.Epoch != i.epoch {
				return Signal{Kind: "abandoned"}, nil
			}
			sig, err := i.Block(s.Body, f)
			if err != nil || sig.Kind != "" {
				return sig, err
			}
			if y := i.RealYieldSeq(); y != lastYield {
				lastYield = y
				guard = 0
			} else {
				guard++
				if guard > 100000 {
					return normal, fmt.Errorf("while loop runaway (100k iterations)")
				}
			}
		}
	case "for":
		from, err := i.Eval(s.From, f)
		if err != nil {
			return normal, err
		}
		to, err := i.Eval(s.To, f)
		if err != nil {
			return normal, err
		}
		step := Num(1)
		if s.Step != nil {
			step, err = i.Eval(s.Step, f)
			if err != nil {
				return normal, err
			}
		}
		delta := step.Num()
		if delta == 0 {
			return normal, fmt.Errorf("for loop with step 0")
		}
		for n := from.Num(); (delta > 0 && n <= to.Num()) || (delta < 0 && n >= to.Num()); n += delta {
			i.SetVar(s.Name, Num(n), f)
			sig, err := i.Block(s.Body, f)
			if err != nil || sig.Kind != "" {
				return sig, err
			}
		}
	case "exitcode":
		if f.Handler == f.Dispatch {
			f.chain.consumed = true
		}
		return Signal{Kind: "exitcode"}, nil
	case "passcode":
		return Signal{Kind: "passcode"}, nil
	case "return":
		v, err := i.Eval(s.Value, f)
		return Signal{Kind: "return", Value: v}, err
	default:
		return normal, fmt.Errorf("unknown statement %s", s.Kind)
	}
	return normal, nil
}
func (i *Interpreter) Eval(e *Expr, f *Frame) (Value, error) {
	if e == nil {
		return Value{}, nil
	}
	switch e.Kind {
	case "number":
		return Num(e.Number), nil
	case "string":
		return Str(e.Text), nil
	case "bool":
		return Bool(e.Bool), nil
	case "me":
		return Str(f.Context.Me), nil
	case "target":
		return Str(f.Context.Target), nil
	case "variable":
		return i.GetVar(e.Text, f), nil
	case "call":
		return i.Call(e, f)
	case "unary":
		v, err := i.Eval(e.Left, f)
		if err != nil {
			return Value{}, err
		}
		if e.Text == "not" {
			return Bool(!v.Truthy()), nil
		}
		return Num(-v.Num()), nil
	case "binary":
		l, err := i.Eval(e.Left, f)
		if err != nil {
			return Value{}, err
		}
		if e.Text == "&" && !l.Truthy() {
			return Num(0), nil
		}
		if e.Text == "|" && l.Truthy() {
			return Num(1), nil
		}
		r, err := i.Eval(e.Right, f)
		if err != nil {
			return Value{}, err
		}
		switch e.Text {
		case "&", "|":
			return Bool(r.Truthy()), nil
		case "@":
			return Str(l.String() + r.String()), nil
		case "+":
			return Num(l.Num() + r.Num()), nil
		case "-":
			return Num(l.Num() - r.Num()), nil
		case "*":
			return Num(l.Num() * r.Num()), nil
		case "/":
			return Num(math.Trunc(l.Num() / r.Num())), nil
		case "=":
			return Bool(Equal(l, r)), nil
		case "!=":
			return Bool(!Equal(l, r)), nil
		case ">":
			return Bool(l.Num() > r.Num()), nil
		case "<":
			return Bool(l.Num() < r.Num()), nil
		case ">=":
			return Bool(l.Num() >= r.Num()), nil
		case "<=":
			return Bool(l.Num() <= r.Num()), nil
		}
	}
	return Value{}, fmt.Errorf("unknown expression %s %s", e.Kind, e.Text)
}
func (i *Interpreter) Call(call *Expr, f *Frame) (Value, error) {
	name := strings.ToLower(call.Text)
	if call.Opcode == 0 && f.Instance.Script.Handlers[name] != nil {
		args, err := i.Args(call.Args, f)
		if err != nil {
			return Value{}, err
		}
		r, err := i.Run(f.Instance, call.Text, args, f.Context, f)
		return r.Value, err
	}
	if special := i.SpecialForms[name]; special != nil {
		return special(i, call.Args, f)
	}
	args, err := i.Args(call.Args, f)
	if err != nil {
		return Value{}, err
	}
	if builtin := i.Builtins[name]; builtin != nil {
		return builtin(i, args, call, f)
	}
	if call.Opcode == 0 {
		for p := f.Instance.Parent; p != nil; p = p.Parent {
			if p.Script.Handlers[name] != nil {
				r, err := i.Run(p, call.Text, args, f.Context, f)
				return r.Value, err
			}
		}
		for _, p := range i.Fallbacks {
			if p.Script.Handlers[name] != nil {
				r, err := i.Run(p, call.Text, args, f.Context, f)
				return r.Value, err
			}
		}
	}
	if !i.unknown[name] {
		i.unknown[name] = true
		if i.OnUnknown != nil {
			i.OnUnknown(call.Text, args)
		}
	}
	return Value{}, nil
}
func (i *Interpreter) Args(exprs []*Expr, f *Frame) ([]Value, error) {
	out := make([]Value, len(exprs))
	for n, e := range exprs {
		v, err := i.Eval(e, f)
		if err != nil {
			return nil, err
		}
		out[n] = v
	}
	return out, nil
}
func (i *Interpreter) GetVar(name string, f *Frame) Value {
	if v, ok := f.Locals.Get(name); ok {
		return v
	}
	v, _ := i.Globals.Get(name)
	return v
}
func (i *Interpreter) SetVar(name string, v Value, f *Frame) {
	if f.Locals.Has(name) {
		f.Locals.Set(name, v)
	} else if i.Globals.Has(name) {
		i.SetGlobal(name, v)
	} else {
		f.Locals.Set(name, v)
	}
}
func (i *Interpreter) SetGlobal(name string, v Value) {
	from, ok := i.Globals.Get(name)
	if !ok {
		from = Str("")
	}
	i.Globals.Set(name, v)
	key := strings.ToLower(name)
	if i.WatchGlobals[key] && !Equal(from, v) && i.OnGlobalChange != nil {
		i.OnGlobalChange(key, from, v)
	}
}
