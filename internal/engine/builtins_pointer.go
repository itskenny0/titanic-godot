package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"strings"
)

func registerPointerBuiltins(c builtinContext) {
	s := c.s
	c.v("makepoint", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(PackPoint(numArg(a, 0, 0), numArg(a, 1, 0))))
	})
	c.v("pointx", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(PointX(numArg(a, 0, 0))))
	})
	c.v("pointy", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(PointY(numArg(a, 0, 0))))
	})
	c.v("mouse", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(s.PointerPoint())) })
	c.v("hittest", func(a []script.Value, _ *script.Frame) script.Value {
		p := numArg(a, 0, 0)
		hit := s.HitTestAt(float64(PointX(p)), float64(PointY(p)))
		s.LastResult = hit.Type
		return script.Str(hit.Name)
	})
	c.v("result", func([]script.Value, *script.Frame) script.Value { return script.Str(s.LastResult) })
	c.v("pointinactor", func(a []script.Value, _ *script.Frame) script.Value {
		p := numArg(a, 1, 0)
		hit := s.HitTestAt(float64(PointX(p)), float64(PointY(p)))
		return script.Bool(hit.Type == "actor" && strings.EqualFold(hit.Name, strArg(a, 0)))
	})
	for _, name := range []string{"pointinset", "pointinstage"} {
		c.v(name, func(a []script.Value, _ *script.Frame) script.Value {
			p := numArg(a, 0, 0)
			test := s.PointInSet
			if name == "pointinstage" {
				test = s.PointInStage
			}
			return script.Bool(test(float64(PointX(p)), float64(PointY(p))))
		})
	}
	c.r("button", func([]script.Value, *script.Frame) (script.Value, error) {
		s.InputPolled()
		if err := c.yieldFrame(); err != nil {
			return script.Value{}, err
		}
		return script.Bool(s.PointerDown), nil
	})
	c.action("flushevents", func([]script.Value, *script.Frame) error { s.Events.Flush(); return nil })
	c.v("mousedown", func(a []script.Value, _ *script.Frame) script.Value { return script.Num(numArg(a, 0, 0)) })
	c.action("drawstring", func(a []script.Value, _ *script.Frame) error {
		p := numArg(a, 1, 0)
		e := TextOverlay{Text: df.DecodeMacRoman(strArg(a, 0)), X: float64(PointX(p)), Y: float64(PointY(p)), Size: numArg(a, 3, 12), Color: numArg(a, 2, 0)}
		for i, old := range s.TextOverlay {
			if old.X == e.X && old.Y == e.Y && old.Size == e.Size {
				s.TextOverlay[i] = e
				return nil
			}
		}
		s.TextOverlay = append(s.TextOverlay, e)
		return nil
	})
	c.v("stringwidth", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(s.TextWidth(df.DecodeMacRoman(strArg(a, 0)), numArg(a, 2, 12)))
	})
	c.r("stilldown", func([]script.Value, *script.Frame) (script.Value, error) {
		task, err := c.task()
		if err != nil {
			return script.Value{}, err
		}
		s.InputPolled()
		task.Sleep(16)
		if s.HasRealFrames {
			s.RealYieldSeq++
		}
		return script.Bool(s.PointerDown), nil
	})
	c.v("pointinbutton", func(a []script.Value, _ *script.Frame) script.Value {
		r := s.StageCtrl.FlatRegion(strArg(a, 0), strArg(a, 1))
		if r == nil {
			return script.Num(0)
		}
		p := numArg(a, 2, 0)
		x, y := int(PointX(p)), int(PointY(p))
		return script.Bool(x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom)
	})
	c.r("pointinprop", func(a []script.Value, _ *script.Frame) (script.Value, error) {
		p := s.Props.Get(strArg(a, 0))
		if p == nil {
			return script.Num(0), nil
		}
		st := p.State()
		if st == nil || len(st.Frames) == 0 {
			return script.Num(0), nil
		}
		idx := int(math.Min(float64(p.FrameIdx), float64(len(st.Frames)-1)))
		if idx < 0 {
			return script.Num(0), nil
		}
		f, err := p.Shop.Frame(st.Frames[idx])
		if err != nil {
			return script.Value{}, err
		}
		x0, y0 := p.AnchorX-float64(f.PosXraw), p.AnchorY-float64(f.PosYraw)
		pt := numArg(a, 1, 0)
		x, y := float64(PointX(pt)), float64(PointY(pt))
		return script.Bool(x >= x0 && x < x0+float64(f.Width) && y >= y0 && y < y0+float64(f.Height)), nil
	})
}
