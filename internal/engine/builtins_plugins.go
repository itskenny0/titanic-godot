package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/script"
	"strings"
)

func registerPluginBuiltins(c builtinContext) {
	s := c.s
	xray := func(a []script.Value) script.Value {
		if len(a) == 0 {
			s.XRay = nil
			return script.Num(0)
		}
		if a[0].IsString {
			hidden, base, mask, light := strArg(a, 0), strArg(a, 1), strArg(a, 2), strArg(a, 3)
			if hidden == "" || mask == "" {
				missing := "mask prop"
				if hidden == "" {
					missing = "hidden flat"
				}
				c.log("plugin(\"xray\"): armed with no " + missing)
				return script.Num(0)
			}
			s.XRay = &XRayReveal{Hidden: hidden, Base: base, Mask: mask, Light: light}
			if base == "" {
				base = "(current flat)"
			}
			c.log("xray: " + hidden + " through " + mask + " over " + base)
			return script.Num(0)
		}
		if s.XRay != nil {
			pt := a[0].Num()
			s.XRay.X = float64(PointX(pt))
			s.XRay.Y = float64(PointY(pt))
			s.XRay.Aimed = true
		}
		return script.Num(0)
	}
	camera := func(a []script.Value) (script.Value, error) {
		s.Photos.Open(s.Executor.Current())
		if len(a) <= 1 {
			return script.Num(CameraOK), nil
		}
		id := numArg(a, 1, 0)
		if len(a) >= 3 {
			if s.Photos.Get(id) != nil {
				return script.Num(CameraIDTaken), nil
			}
			pt := numArg(a, 2, 0)
			var photo *RGBAFrame
			if s.GrabPhoto != nil {
				photo = s.GrabPhoto(float64(PointX(pt)), float64(PointY(pt)))
			}
			if photo == nil {
				c.log("plugin(\"camera\"): nothing to photograph; the shot is empty")
				return script.Num(CameraOK), nil
			}
			return script.Num(float64(s.Photos.Save(id, photo))), nil
		}
		photo := s.Photos.Get(id)
		if photo == nil {
			return script.Num(CameraNoPhoto), nil
		}
		s.PhotoOverlay = &PhotoOverlay{Photo: photo, X: PhotoX, Y: PhotoY}
		s.Fade.Queue = nil
		s.Fade.Snapshot = nil
		s.Fade.Level = 0
		s.Fade.PendingReveal = false
		return script.Num(CameraOK), nil
	}
	for _, name := range []string{"plugin", "pluginfx"} {
		c.r(name, func(a []script.Value, _ *script.Frame) (script.Value, error) {
			plugin := strings.ToLower(strArg(a, 0))
			var rest []script.Value
			if len(a) > 1 {
				rest = a[1:]
			}
			switch plugin {
			case "xray":
				return xray(rest), nil
			case "camera":
				return camera(rest)
			case "scrollflat":
				c.log("plugin(\"scrollflat\"): the smooth turn is not implemented; expected minMemory to route around it")
			default:
				c.log(fmt.Sprintf("%s(%q): unknown plugin", name, plugin))
			}
			return script.Num(0), nil
		})
	}
}
