package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"regexp"
	"strings"
)

func registerHelperBuiltins(c builtinContext) {
	s := c.s
	c.v("findword", func(a []script.Value, _ *script.Frame) script.Value {
		text, sep := strArg(a, 0), strArg(a, 1)
		idx := propertyNumberOrZero(arg(a, 2, script.Num(0)))
		if sep == "" {
			units := script.UTF16Units(text)
			if idx < 1 || idx > float64(len(units)) {
				return script.Str("")
			}
			if idx != math.Trunc(idx) {
				return script.Num(0)
			}
			return script.Str(script.StringFromUTF16(units[int(idx)-1 : int(idx)]))
		}
		if idx == 0 {
			idx = 1
		}
		return script.Str(stringIndex(strings.Split(text, sep), idx-1))
	})
	c.r("putword", func(a []script.Value, _ *script.Frame) (script.Value, error) {
		text, sep, word := strArg(a, 0), strArg(a, 1), strArg(a, 3)
		idx := propertyNumberOrZero(arg(a, 2, script.Num(0)))
		if sep == "" {
			units := script.UTF16Units(text)
			if idx >= 1 && idx <= float64(len(units)) {
				at := int(idx - 1)
				return script.Str(script.StringFromUTF16(units[:at]) + word + script.StringFromUTF16(units[at:])), nil
			}
			if idx == float64(len(units)+1) {
				return script.Str(text + word), nil
			}
			return script.Str(""), nil
		}
		at := math.Max(1, idx) - 1
		// Limit growth from corrupt scripts before allocating an unbounded string.
		if math.IsInf(at, 0) || at > 1<<20 {
			return script.Value{}, fmt.Errorf("putword: output exceeds string limit")
		}
		parts := []string{}
		if text != "" {
			parts = strings.Split(text, sep)
		}
		for float64(len(parts)) <= at {
			parts = append(parts, "")
		}
		if at == math.Trunc(at) {
			parts[int(at)] = word
		}
		return script.Str(strings.Join(parts, sep)), nil
	})
	c.v("stringlength", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(len(script.UTF16Units(strArg(a, 0)))))
	})
	c.v("variable", func(a []script.Value, f *script.Frame) script.Value {
		key := strArg(a, 0)
		if len(a) < 2 {
			if f != nil {
				return s.Interp.GetVar(key, f)
			}
			v, _ := s.Interp.Globals.Get(key)
			return v
		}
		if f != nil && f.Locals.Has(key) {
			f.Locals.Set(key, a[1])
		} else {
			s.Interp.SetGlobal(key, a[1])
		}
		return script.Num(0)
	})
	c.v("numtostring", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(script.Num(numArg(a, 0, 0)).String())
	})
	c.v("lowmemory", func([]script.Value, *script.Frame) script.Value { return script.Num(0) })
	c.v("heapsize", func([]script.Value, *script.Frame) script.Value {
		if s.LowMemory {
			return script.Num(4 * 1024 * 1024)
		}
		return script.Num(64 * 1024 * 1024)
	})
	for _, name := range []string{"stageparam", "setparam"} {
		params := map[float64]script.Value{}
		c.v(name, func(a []script.Value, _ *script.Frame) script.Value {
			key := numArg(a, 0, 0)
			if len(a) < 2 {
				return params[key]
			}
			params[key] = a[1]
			return script.Num(0)
		})
	}
	axis := func(a []script.Value, _ *script.Frame) script.Value {
		lis := s.Listener()
		if lis == nil {
			return script.Num(0)
		}
		switch numArg(a, 0, 1) {
		case 1:
			return script.Num(lis.X)
		case 2:
			return script.Num(lis.Y)
		case 4:
			return script.Num(float64(PackPoint(lis.X, lis.Y)))
		}
		return script.Num(0)
	}
	c.v("cameraxyz", axis)
	c.v("playerxyz", axis)
	c.v("currentdeg", func([]script.Value, *script.Frame) script.Value {
		lis := s.Listener()
		if lis == nil {
			return script.Num(-1)
		}
		return script.Num(float64(int32JS(lis.Deg) & 255))
	})
	c.v("calcdeg", func(a []script.Value, _ *script.Frame) script.Value {
		p, q := numArg(a, 0, 0), numArg(a, 1, 0)
		return script.Num(float64(Bearing(float64(PointX(q))-float64(PointX(p)), float64(PointY(q))-float64(PointY(p)))))
	})
	c.v("calcmod", func(a []script.Value, _ *script.Frame) script.Value {
		m := numArg(a, 1, 0)
		if m == 0 {
			return script.Num(0)
		}
		return script.Num(math.Mod(math.Mod(numArg(a, 0, 0), m)+m, m))
	})
	c.v("calcdist", func(a []script.Value, _ *script.Frame) script.Value {
		p, q := numArg(a, 0, 0), numArg(a, 1, 0)
		return script.Num(jsRound(math.Hypot(float64(PointX(q))-float64(PointX(p)), float64(PointY(q))-float64(PointY(p)))))
	})
	for _, name := range []string{"calcvectx", "calcvecty"} {
		c.v(name, func(a []script.Value, _ *script.Frame) script.Value {
			angle := float64(int32JS(numArg(a, 0, 0))&255) * 2 * math.Pi / 256
			trig := math.Cos
			if name == "calcvecty" {
				trig = math.Sin
			}
			entry := jsRound(16384 * trig(angle))
			n := math.Trunc(entry * float64(int16(int32JS(numArg(a, 1, 0)))) / 16384)
			return script.Num(float64(int16(int32JS(n))))
		})
	}
	c.v("currentcd", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) > 0 {
			s.MountedCD = a[0].String()
		}
		return script.Str(s.MountedCD)
	})
	host := func(a []script.Value, i int) string { return df.DecodeMacRoman(strArg(a, i)) }
	c.action("notedialog", func(a []script.Value, _ *script.Frame) error { return s.OnNoteDialog(host(a, 0)) })
	c.r("questiondialog", func(a []script.Value, _ *script.Frame) (script.Value, error) {
		on, err := s.OnQuestionDialog(host(a, 0))
		return script.Bool(on), err
	})
	c.r("textdialog", func(a []script.Value, _ *script.Frame) (script.Value, error) {
		text, err := s.OnTextDialog(host(a, 0), host(a, 1))
		return script.Str(df.Latin1(df.EncodeMacRoman(text, 255))), err
	})
	c.action("quit", func([]script.Value, *script.Frame) error { return s.OnQuit() })
	c.v("machinetype", func([]script.Value, *script.Frame) script.Value { return script.Str("win") })
	c.v("freemem", func([]script.Value, *script.Frame) script.Value { return script.Num(3 * 1024 * 1024) })
	c.v("sysmem", func([]script.Value, *script.Frame) script.Value { return script.Num(8 * 1024 * 1024) })
	c.v("tick", func([]script.Value, *script.Frame) script.Value { return script.Num(TicksAt(s.Executor.Now())) })
	c.v("frame", func([]script.Value, *script.Frame) script.Value { return script.Num(s.Clock.FrameCounter) })
	for _, name := range []string{"menuvisible", "keyaborts"} {
		value := script.Num(0)
		c.v(name, func(a []script.Value, _ *script.Frame) script.Value {
			if len(a) > 0 {
				value = script.Bool(a[0].Truthy())
			}
			return value
		})
	}
	c.v("countbevels", func([]script.Value, *script.Frame) script.Value {
		if s.PuppetCtrl.Puppet == nil {
			return script.Num(0)
		}
		return script.Num(float64(len(s.PuppetCtrl.Puppet.Bevels)))
	})
	paths := [9]string{}
	disc := regexp.MustCompile(`(?i)^titanic([12]):`)
	c.v("path", func(a []script.Value, _ *script.Frame) script.Value {
		idx := numArg(a, 0, 0)
		if len(a) < 2 {
			return script.Str(stringIndex(paths[:], idx))
		}
		if idx >= 1 && idx <= 8 {
			value := a[1].String()
			if idx == math.Trunc(idx) {
				paths[int(idx)] = value
			}
			match := disc.FindStringSubmatch(value)
			if len(match) > 1 && s.OnDiscChange != nil {
				s.OnDiscChange(int(match[1][0] - '0'))
			}
		}
		return script.Num(0)
	})
}
