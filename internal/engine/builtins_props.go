package engine

import (
	"github.com/itskenny0/titanic-godot/internal/script"
	"strings"
)

// Accessors return the empty value for missing objects, and zero after a setter.
func accessor[T any](c builtinContext, name string, empty script.Value, resolve func(string) *T, get func(*T) script.Value, set func(*T, script.Value, string, *script.Frame) error) {
	c.r(name, func(a []script.Value, f *script.Frame) (script.Value, error) {
		key := strArg(a, 0)
		p := resolve(key)
		if p == nil {
			return empty, nil
		}
		if len(a) < 2 {
			return get(p), nil
		}
		return script.Num(0), set(p, a[1], key, f)
	})
}
func registerPropBuiltins(c builtinContext) {
	s := c.s
	acc := func(name string, empty script.Value, get func(*PropInstance) script.Value, set func(*PropInstance, script.Value) error) {
		accessor(c, name, empty, s.Props.Get, get, func(p *PropInstance, v script.Value, _ string, _ *script.Frame) error { return set(p, v) })
	}
	c.v("propexists", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(s.Props.Get(strArg(a, 0)) != nil)
	})
	c.v("propis3d", func([]script.Value, *script.Frame) script.Value { return script.Num(0) })
	c.action("propdelete", func(a []script.Value, _ *script.Frame) error { s.Props.Remove(strArg(a, 0)); return nil })
	c.action("prophide", func(a []script.Value, _ *script.Frame) error {
		on := arg(a, 1, script.Num(1)).Truthy()
		name := strings.ToLower(strArg(a, 0))
		if name == "all" {
			for _, p := range s.Props.Props.All() {
				if !on || p.Visible {
					p.Hidden = on
				}
			}
		} else if p := s.Props.Get(name); p != nil {
			p.Hidden = on
		}
		return nil
	})
	acc("propvisible", script.Num(0), func(p *PropInstance) script.Value { return script.Bool(p.Visible) }, func(p *PropInstance, v script.Value) error {
		was := p.Visible
		p.Visible = v.Truthy()
		if !was && p.Visible {
			return s.EraseTextUnderProp(p)
		}
		return nil
	})
	acc("propview", script.Str(""), func(p *PropInstance) script.Value {
		name := p.StateName
		if name == "" {
			if st := p.State(); st != nil {
				name = strings.ToLower(st.Identifier)
			}
		}
		return script.Str(name)
	}, func(p *PropInstance, v script.Value) error {
		p.StateName = strings.ToLower(v.String())
		p.LastTick = 0
		st := p.State()
		var variant []int
		if st != nil {
			variant = DegreeVariantFrames(st, propertyNumberOrZero(p.Deg))
		}
		picked := p.DegVariants && p.DegEvent == float64(s.Interp.CurrentEvent)
		if st != nil && (IsDegreeSelector(st) || (p.DegVariants && variant == nil && (picked || len(st.Frames) <= 2))) {
			p.FrameOrder = nil
			p.FrameIdx = FrameIndexForDegree(st, propertyNumberOrZero(p.Deg))
			p.FrameLocked = true
			p.Animating = false
		} else {
			p.FrameIdx = 0
			p.FrameLocked = false
			p.FrameOrder = nil
			if st != nil {
				p.FrameOrder = PlaySequence(st, variant)
			}
			p.Animating = st != nil && p.FrameCount(st) > 1
		}
		return nil
	})
	c.v("propxy", func(a []script.Value, _ *script.Frame) script.Value {
		p := s.Props.Get(strArg(a, 0))
		if p == nil || len(a) < 2 {
			return script.Num(0)
		}
		if len(a) < 3 {
			if a[1].Num() == 2 {
				return script.Num(p.AnchorY)
			}
			return script.Num(p.AnchorX)
		}
		p.AnchorX = propertyNumberOrZero(a[1])
		p.AnchorY = propertyNumberOrZero(a[2])
		p.ScreenPlaced = true
		p.WorldSpace = false
		return script.Num(0)
	})
	c.v("propxyz", func(a []script.Value, _ *script.Frame) script.Value {
		p := s.Props.Get(strArg(a, 0))
		if p == nil || len(a) < 2 {
			return script.Num(0)
		}
		if len(a) < 3 {
			switch a[1].Num() {
			case 1:
				return script.Num(p.WorldX)
			case 2:
				return script.Num(p.WorldY)
			case 3:
				return script.Num(p.WorldZ)
			case 4:
				return script.Num(float64(PackPoint(p.WorldX, p.WorldY)))
			}
			return script.Num(0)
		}
		BecomeWorldProp(p, false)
		p.WorldX = a[1].Num()
		p.WorldY = a[2].Num()
		p.WorldZ = numArg(a, 3, 0)
		p.StarPending = false
		return script.Num(0)
	})
	acc("propset", script.Str(""), func(p *PropInstance) script.Value { return script.Str(p.SetName) }, func(p *PropInstance, v script.Value) error {
		p.SetName = strings.ToLower(v.String())
		if p.SetName != "" {
			BecomeWorldProp(p, false)
		}
		return nil
	})
	for _, name := range []string{"propscale", "propzclip", "propdist", "propspeed"} {
		field := func(p *PropInstance) *float64 {
			switch name {
			case "propscale":
				return &p.Scale
			case "propzclip":
				return &p.Zclip
			case "propdist":
				return &p.Dist
			default:
				return &p.Speed
			}
		}
		acc(name, script.Num(0), func(p *PropInstance) script.Value { return script.Num(*field(p)) }, func(p *PropInstance, v script.Value) error { *field(p) = propertyNumberOrZero(v); return nil })
	}
	acc("propowner", script.Str(""), func(p *PropInstance) script.Value { return p.Owner }, func(p *PropInstance, v script.Value) error { p.Owner = v; return nil })
	c.action("propinstance", func(a []script.Value, _ *script.Frame) error {
		s.Props.Instance(strArg(a, 0), strArg(a, 1))
		return nil
	})
	acc("propdeg", script.Num(0), func(p *PropInstance) script.Value { return p.Deg }, func(p *PropInstance, v script.Value) error {
		p.Deg = v
		if p.WorldSpace {
			p.Directional = true
			return nil
		}
		p.DegVariants = true
		p.DegEvent = float64(s.Interp.CurrentEvent)
		st := p.State()
		if st != nil && p.Animating {
			if variant := DegreeVariantFrames(st, propertyNumberOrZero(v)); variant != nil {
				p.FrameOrder = PlaySequence(st, variant)
				return nil
			}
		}
		if st != nil && len(st.Frames) > 0 {
			p.FrameOrder = nil
			p.FrameIdx = FrameIndexForDegree(st, propertyNumberOrZero(v))
			p.FrameLocked = true
		}
		return nil
	})
	c.v("countprops", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(s.Props.Props.Len())) })
	c.v("indextoprop", func(a []script.Value, _ *script.Frame) script.Value {
		name := stringIndex(s.Props.Props.keys, numArg(a, 0, 0)-1)
		s.LastResult = ""
		if p := s.Props.Props.Get(name); p != nil {
			s.LastResult = p.Shop.Name
		}
		return script.Str(name)
	})
	c.action("error", func(a []script.Value, _ *script.Frame) error {
		parts := make([]string, len(a))
		for i, v := range a {
			parts[i] = v.String()
		}
		c.log("script error(): " + strings.Join(parts, ", "))
		return nil
	})
	acc("propvalue", script.Num(0), func(p *PropInstance) script.Value { return p.Value }, func(p *PropInstance, v script.Value) error { p.Value = v; return nil })
	c.v("starxyz", func(a []script.Value, _ *script.Frame) script.Value {
		star := c.star(strArg(a, 0))
		if star == nil {
			c.log("starxyz: no star \"" + strArg(a, 0) + "\" in " + s.SetName)
			return script.Num(0)
		}
		switch numArg(a, 1, 1) {
		case 1:
			return script.Num(float64(star.PositionX))
		case 2:
			return script.Num(float64(star.PositionZ))
		case 3:
			return script.Num(float64(star.PositionY))
		case 4:
			return script.Num(float64(PackPoint(float64(star.PositionX), float64(star.PositionZ))))
		}
		return script.Num(0)
	})
	acc("propstar", script.Str(""), func(p *PropInstance) script.Value { return script.Str(p.StarName) }, func(p *PropInstance, v script.Value) error {
		star := c.star(v.String())
		if star != nil {
			BecomeWorldProp(p, false)
			p.WorldX = float64(star.PositionX)
			p.WorldY = float64(star.PositionZ)
			p.WorldZ = float64(star.PositionY)
			p.Deg = script.Num(float64(star.Rotation8 & 255))
		}
		p.StarName = strings.ToLower(v.String())
		p.StarPending = star == nil && strings.Contains(p.StarName, ".")
		return nil
	})
}
