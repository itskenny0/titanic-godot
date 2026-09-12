package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"strings"
	"unicode/utf16"
)

type builtinContext struct{ s *Session }

func (c builtinContext) r(name string, fn func([]script.Value, *script.Frame) (script.Value, error)) {
	if err := c.s.Interp.Register(name, func(_ *script.Interpreter, a []script.Value, _ *script.Expr, f *script.Frame) (script.Value, error) {
		return fn(a, f)
	}); err != nil {
		panic(err)
	}
}
func (c builtinContext) v(name string, fn func([]script.Value, *script.Frame) script.Value) {
	c.r(name, func(a []script.Value, f *script.Frame) (script.Value, error) { return fn(a, f), nil })
}
func (c builtinContext) action(name string, fn func([]script.Value, *script.Frame) error) {
	c.r(name, func(a []script.Value, f *script.Frame) (script.Value, error) { return script.Num(0), fn(a, f) })
}
func (c builtinContext) log(line string) {
	if c.s.CurrentBinding != nil {
		c.s.CurrentBinding.Log(line)
	} else {
		c.s.Log(line)
	}
}
func (c builtinContext) task() (*Task, error) {
	t := c.s.Executor.Current()
	if t == nil {
		return nil, fmt.Errorf("waiting builtin called outside a game task")
	}
	return t, nil
}
func (c builtinContext) yieldFrame() error {
	if c.s.HasRealFrames {
		t, err := c.task()
		if err != nil {
			return err
		}
		c.s.RealYieldSeq++
		t.NextFrame()
	}
	return nil
}
func arg(a []script.Value, i int, def script.Value) script.Value {
	if i >= 0 && i < len(a) {
		return a[i]
	}
	return def
}
func strArg(a []script.Value, i int) string               { return arg(a, i, script.Str("")).String() }
func numArg(a []script.Value, i int, def float64) float64 { return arg(a, i, script.Num(def)).Num() }
func stringIndex(list []string, index float64) string {
	if index != math.Trunc(index) || index < 0 || index >= float64(len(list)) {
		return ""
	}
	return list[int(index)]
}
func (c builtinContext) star(name string) *df.Actor {
	lower := strings.ToLower(name)
	if b := c.s.CurrentBinding; b != nil {
		for _, star := range b.Set.Actors {
			if strings.ToLower(star.Identifier) == lower {
				return &star
			}
		}
	}
	if c.s.StarRegistry.Has(lower) {
		star := c.s.StarRegistry.Get(lower)
		return &star
	}
	return nil
}
func registerSessionBuiltins(s *Session) {
	c := builtinContext{s}
	registerCoreBuiltins(c)
	registerDispatchBuiltins(c)
	registerTimingBuiltins(c)
	registerPuppetBuiltins(c)
	registerAudioBuiltins(c)
	registerPointerBuiltins(c)
	registerHelperBuiltins(c)
	registerPropBuiltins(c)
	registerSceneBuiltins(c)
	registerActorBuiltins(c)
	registerSaveBuiltins(c)
	registerPluginBuiltins(c)
}
func registerCoreBuiltins(c builtinContext) {
	c.v("random", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(math.Floor(c.s.Random()*numArg(a, 0, 0)) + 1)
	})
	c.v("sqrt", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(math.Floor(math.Sqrt(numArg(a, 0, 0))))
	})
	c.v("stringtonum", func(a []script.Value, _ *script.Frame) script.Value { return script.Num(numArg(a, 0, 0)) })
	c.v("substring", func(a []script.Value, _ *script.Frame) script.Value {
		s, n := strings.ToLower(strArg(a, 0)), strings.ToLower(strArg(a, 1))
		i := strings.Index(s, n)
		if i < 0 {
			return script.Num(-1)
		}
		return script.Num(float64(len(utf16.Encode([]rune(s[:i]))) + 1))
	})
	c.v("true", func([]script.Value, *script.Frame) script.Value { return script.Num(1) })
	c.v("false", func([]script.Value, *script.Frame) script.Value { return script.Num(0) })
}
func registerDispatchBuiltins(c builtinContext) {
	s := c.s
	exprAt := func(args []*script.Expr, i int) *script.Expr {
		if i < len(args) {
			return args[i]
		}
		return nil
	}
	for _, cmd := range []string{"sendtoprop", "sendtoactor", "sendtoscene", "sendtoset", "sendtoshop", "sendtoshopfx", "sendtopuppet", "sendtocast", "sendtostage", "sendtoflat", "sendtoboot", "sendtopost", "sendtoserver", "sendtoactorfx", "sendtocastfx", "sendtopropfx", "sendtostagefx", "sendtopuppetfx", "sendtoflatfx", "sendtopostfx", "sendtoserverfx", "sendtobootfx"} {
		s.Interp.SpecialForms[cmd] = func(ip *script.Interpreter, args []*script.Expr, f *script.Frame) (script.Value, error) {
			var target string
			deferred := exprAt(args, 1)
			if len(args) == 1 && args[0] != nil && args[0].Kind == "call" {
				target = "main.stg"
				if strings.HasPrefix(cmd, "sendtoboot") {
					target = "boot"
				} else if s.StageScript != nil {
					target = s.StageScript.Name
				}
				deferred = args[0]
			} else {
				v, err := ip.Eval(exprAt(args, 0), f)
				if err != nil {
					return script.Value{}, err
				}
				target = v.String()
			}
			if deferred == nil || deferred.Kind != "call" {
				c.log(cmd + ": no deferred call argument")
				return script.Num(0), nil
			}
			values, err := ip.Args(deferred.Args, f)
			if err != nil {
				return script.Value{}, err
			}
			return s.SendEvent(cmd, target, deferred.Text, values, f.Context.Me, f)
		}
	}
	for _, cmd := range []string{"sendtopainting", "sendtopaintingfx", "sendtobutton", "sendtobuttonfx"} {
		s.Interp.SpecialForms[cmd] = func(ip *script.Interpreter, args []*script.Expr, f *script.Frame) (script.Value, error) {
			n := 2
			if strings.HasPrefix(cmd, "sendtopainting") {
				n = 3
			}
			names := make([]string, n)
			for i := range names {
				v, err := ip.Eval(exprAt(args, i), f)
				if err != nil {
					return script.Value{}, err
				}
				names[i] = v.String()
			}
			deferred := exprAt(args, n)
			if deferred == nil || deferred.Kind != "call" {
				c.log(cmd + ": no deferred call argument")
				return script.Num(0), nil
			}
			values, err := ip.Args(deferred.Args, f)
			if err != nil {
				return script.Value{}, err
			}
			if n == 3 {
				if s.CurrentBinding == nil {
					return script.Num(0), nil
				}
				return script.Bool(s.CurrentBinding.PaintingEvent(names[0], names[1], names[2], deferred.Text, values, f)), nil
			}
			return s.StageCtrl.SendToButton(names[0], names[1], deferred.Text, values, f.Context.Me, f)
		}
	}
	c.action("cursor", func(a []script.Value, _ *script.Frame) error { s.CursorName = strArg(a, 0); return nil })
	c.action("message", func(a []script.Value, _ *script.Frame) error {
		parts := []string{}
		for _, v := range a {
			parts = append(parts, v.String())
		}
		c.log("msg: " + strings.Join(parts, " "))
		return nil
	})
}
func registerTimingBuiltins(c builtinContext) {
	s := c.s
	c.action("delay", func(a []script.Value, _ *script.Frame) error {
		task, err := c.task()
		if err != nil {
			return err
		}
		task.Sleep(numArg(a, 0, 0) * 50 / 3)
		return nil
	})
	c.action("makeloop", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.MakeLoop(strArg(a, 0), strArg(a, 1), strArg(a, 2), numArg(a, 3, 1))
		return nil
	})
	c.action("stoploop", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.StopLoop(strArg(a, 0), strArg(a, 1))
		return nil
	})
	c.action("pauseloop", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.PauseLoop(strArg(a, 0), strArg(a, 1), arg(a, 2, script.Num(1)).Truthy())
		return nil
	})
	c.v("isloop", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(s.Scheduler.IsLoop(strArg(a, 0), strArg(a, 1)))
	})
	c.v("countloops", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(len(s.Scheduler.Loops))) })
	c.v("indextoloop", func(a []script.Value, _ *script.Frame) script.Value {
		idx := numArg(a, 0, 0) - 1
		if idx == math.Trunc(idx) && idx >= 0 && idx < float64(len(s.Scheduler.Loops)) {
			return script.Str(s.Scheduler.Loops[int(idx)].Name)
		}
		return script.Str("")
	})
	c.action("makecricket", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.MakeCricket(strArg(a, 0), numArg(a, 1, 0), numArg(a, 2, 0), numArg(a, 3, 1), numArg(a, 4, 0), numArg(a, 5, -1))
		return nil
	})
	c.action("stopcricket", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.StopCricket(arg(a, 0, script.Str("all")).String())
		return nil
	})
	c.action("pausecricket", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.PauseCricket(arg(a, 0, script.Str("all")).String(), arg(a, 1, script.Num(1)).Truthy())
		return nil
	})
	c.v("iscricket", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(s.Scheduler.IsCricket(strArg(a, 0)))
	})
	c.v("countcrickets", func([]script.Value, *script.Frame) script.Value {
		return script.Num(float64(len(s.Scheduler.Crickets)))
	})
	c.v("indextocricket", func(a []script.Value, _ *script.Frame) script.Value {
		idx := numArg(a, 0, 0) - 1
		if idx == math.Trunc(idx) && idx >= 0 && idx < float64(len(s.Scheduler.Crickets)) {
			return script.Str(s.Scheduler.Crickets[int(idx)].Name)
		}
		return script.Str("")
	})
	c.action("soundloop", func(a []script.Value, _ *script.Frame) error {
		s.Scheduler.SoundLoop(strArg(a, 0), arg(a, 1, script.Num(1)).Truthy())
		return nil
	})
	c.action("forceupdate", func(_ []script.Value, f *script.Frame) error {
		task, err := c.task()
		if err != nil {
			return err
		}
		if !s.HasRealFrames {
			if err = s.TickTime(s.Executor.Now() + EngineStepMS); err != nil {
				return err
			}
			s.NextFrame(task)
			return nil
		}
		s.RealYieldSeq++
		deadline := TicksAt(s.Executor.Now()) + math.Max(1, jsRound(s.Clock.FrameRate))
		for {
			task.NextFrame()
			me := ""
			if f != nil {
				me = f.Context.Me
			}
			s.Scheduler.PumpFrameLoops(me)
			if TicksAt(s.Executor.Now()) >= deadline {
				break
			}
		}
		return nil
	})
}
func registerPuppetBuiltins(c builtinContext) {
	s := c.s
	p := s.PuppetCtrl
	c.v("openpuppetfile", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(p.OpenPuppetFile(strArg(a, 0)))
	})
	c.action("closepuppetfile", func([]script.Value, *script.Frame) error { p.ClosePuppetFile(); return nil })
	c.v("currentpuppet", func([]script.Value, *script.Frame) script.Value {
		if p.Puppet == nil {
			return script.Str("none")
		}
		if p.Puppet.Pup.PupName != "" {
			return script.Str(p.Puppet.Pup.PupName)
		}
		return script.Str(p.Puppet.Name)
	})
	c.action("puppetspeak", func(a []script.Value, _ *script.Frame) error {
		task, err := c.task()
		if err != nil {
			return err
		}
		p.Speak(task, strArg(a, 0))
		return nil
	})
	c.action("puppetclear", func([]script.Value, *script.Frame) error { p.Clear(); return nil })
	c.action("puppetbevel", func(a []script.Value, _ *script.Frame) error {
		p.Bevel(df.DecodeMacRoman(strArg(a, 0)), numArg(a, 1, 0))
		return nil
	})
	c.r("puppetevent", func([]script.Value, *script.Frame) (script.Value, error) {
		task, err := c.task()
		if err != nil {
			return script.Value{}, err
		}
		return script.Num(p.Event(task)), nil
	})
	c.v("countpuppets", func([]script.Value, *script.Frame) script.Value {
		if p.Puppet == nil {
			return script.Num(0)
		}
		return script.Num(float64(p.Puppet.Scripts.Len()))
	})
	c.v("indextopuppet", func(a []script.Value, _ *script.Frame) script.Value {
		if p.Puppet == nil {
			return script.Str("")
		}
		return script.Str(stringIndex(p.Puppet.Scripts.keys, numArg(a, 0, 0)-1))
	})
	c.action("puppetbase", func(a []script.Value, _ *script.Frame) error { p.Base(strArg(a, 0)); return nil })
	c.v("puppetvisible", func(a []script.Value, _ *script.Frame) script.Value {
		if p.Puppet == nil {
			return script.Num(0)
		}
		if len(a) == 0 {
			return script.Bool(p.Puppet.Visible)
		}
		p.Puppet.Visible = a[0].Truthy()
		return script.Num(0)
	})
	c.v("puppetparam", func(a []script.Value, _ *script.Frame) script.Value {
		slot := int(numArg(a, 0, 0))
		if len(a) < 2 {
			v, ok := s.PuppetParams[slot]
			if !ok && slot == 7 {
				v = 1
			}
			return script.Num(v)
		}
		s.PuppetParams[slot] = a[1].Num()
		return script.Num(0)
	})
	c.action("puppetscramble", func([]script.Value, *script.Frame) error { p.Scramble(); return nil })
	// These are explicit no-ops in the pinned engine too.
	for _, name := range []string{"puppetsubtitle", "puppetgrab"} {
		c.v(name, func([]script.Value, *script.Frame) script.Value { return script.Num(0) })
	}
}
