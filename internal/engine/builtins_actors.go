package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"regexp"
	"slices"
	"strings"
)

func registerActorBuiltins(c builtinContext) {
	s := c.s
	acc := func(name string, empty script.Value, get func(*ActorInstance) script.Value, set func(*ActorInstance, script.Value) error) {
		accessor(c, name, empty, s.Actors.Get, get, func(a *ActorInstance, v script.Value, _ string, _ *script.Frame) error { return set(a, v) })
	}
	c.v("opencastfile", func(a []script.Value, _ *script.Frame) script.Value { return script.Bool(s.OpenCastFile(strArg(a, 0))) })
	c.action("closecastfile", func(a []script.Value, _ *script.Frame) error { s.CloseCastFile(strArg(a, 0)); return nil })
	c.v("actorexists", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(s.Actors.Get(strArg(a, 0)) != nil)
	})
	c.r("actordist", func(args []script.Value, _ *script.Frame) (script.Value, error) {
		a := s.Actors.Get(strArg(args, 0))
		if len(args) > 1 {
			if a != nil {
				a.Dist = args[1].Num()
			}
			return script.Num(0), nil
		}
		lis := s.Listener()
		if a == nil || !a.Visible || lis == nil || !s.SetVisible || s.PuppetCtrl.Puppet != nil && s.PuppetCtrl.Puppet.Visible {
			return script.Num(32000), nil
		}
		if cam := s.ActiveCamera(); cam != nil {
			seen, err := s.Actors.OnScreen(a, *cam)
			if err != nil {
				return script.Value{}, err
			}
			if !seen {
				return script.Num(32000), nil
			}
		}
		return script.Num(jsRound(math.Hypot(a.WorldX-lis.X, a.WorldY-lis.Y))), nil
	})
	c.action("actorinstance", func(args []script.Value, _ *script.Frame) error {
		src, dst := strArg(args, 0), strArg(args, 1)
		if dst == "" || s.Actors.Get(dst) != nil {
			return nil
		}
		s.Actors.Instance(src, dst)
		if s.Actors.Get(dst) != nil {
			s.InstanceCastScript(src, dst)
		}
		return nil
	})
	c.action("actordelete", func(a []script.Value, _ *script.Frame) error {
		name := strArg(a, 0)
		s.Actors.Remove(name)
		s.DropInstancedScript(name)
		return nil
	})
	dropAttention := func(name string) {
		who := strings.ToLower(name)
		v, _ := s.Interp.Globals.Get("curattention")
		if who != "" && strings.ToLower(v.String()) == who {
			s.Interp.SetGlobal("curattention", script.Str(""))
		}
	}
	s.Interp.WatchGlobals["curattention"] = true
	accessor(c, "actorvisible", script.Num(0), s.Actors.Get, func(a *ActorInstance) script.Value { return script.Bool(a.Visible) }, func(a *ActorInstance, v script.Value, name string, _ *script.Frame) error {
		a.Visible = v.Truthy()
		if !a.Visible {
			dropAttention(name)
		}
		return nil
	})
	c.action("actorhide", func(args []script.Value, _ *script.Frame) error {
		name := strArg(args, 0)
		if a := s.Actors.Get(name); a != nil {
			a.Visible = false
			dropAttention(name)
		}
		return nil
	})
	acc("actorset", script.Str(""), func(a *ActorInstance) script.Value { return script.Str(a.SetName) }, func(a *ActorInstance, v script.Value) error { a.SetName = strings.ToLower(v.String()); return nil })
	c.v("actorxyz", func(args []script.Value, _ *script.Frame) script.Value {
		a := s.Actors.Get(strArg(args, 0))
		if a == nil || len(args) < 2 {
			return script.Num(0)
		}
		if len(args) < 3 {
			switch args[1].Num() {
			case 1:
				return script.Num(a.WorldX)
			case 2:
				return script.Num(a.WorldY)
			case 3:
				return script.Num(a.WorldZ)
			case 4:
				return script.Num(float64(PackPoint(a.WorldX, a.WorldY)))
			}
			return script.Num(0)
		}
		a.WorldX = args[1].Num()
		a.WorldY = args[2].Num()
		a.WorldZ = numArg(args, 3, 0)
		a.StarPending = false
		a.WorldSpace = true
		return script.Num(0)
	})
	c.v("actorxy", func(args []script.Value, _ *script.Frame) script.Value {
		a := s.Actors.Get(strArg(args, 0))
		if a == nil || len(args) < 2 {
			return script.Num(0)
		}
		if len(args) < 3 {
			if args[1].Num() == 2 {
				return script.Num(a.AnchorY)
			}
			return script.Num(a.AnchorX)
		}
		a.AnchorX = args[1].Num()
		a.AnchorY = args[2].Num()
		a.WorldSpace = false
		return script.Num(0)
	})
	acc("actoris3d", script.Num(0), func(a *ActorInstance) script.Value { return script.Bool(a.WorldSpace) }, func(a *ActorInstance, v script.Value) error { a.WorldSpace = v.Truthy(); return nil })
	acc("actorstar", script.Str(""), func(a *ActorInstance) script.Value { return script.Str(a.StarName) }, func(a *ActorInstance, v script.Value) error {
		star := c.star(v.String())
		if star != nil {
			a.WorldX = float64(star.PositionX)
			a.WorldY = float64(star.PositionZ)
			a.WorldZ = float64(star.PositionY)
			a.Deg = float64(star.Rotation8 & 255)
			a.WorldSpace = true
		}
		a.StarName = strings.ToLower(v.String())
		a.StarPending = star == nil && strings.Contains(a.StarName, ".")
		return nil
	})
	acc("actordeg", script.Num(0), func(a *ActorInstance) script.Value { return script.Num(a.Deg) }, func(a *ActorInstance, v script.Value) error { a.Deg = float64(int32JS(v.Num()) & 255); return nil })
	acc("actorpose", script.Str(""), func(a *ActorInstance) script.Value { return script.Str(a.PoseName) }, func(a *ActorInstance, v script.Value) error {
		a.PoseName = strings.ToLower(v.String())
		a.Step = 0
		return nil
	})
	for _, name := range []string{"actorscale", "actorzclip", "actorspeed", "actorturn"} {
		field := func(a *ActorInstance) *float64 {
			switch name {
			case "actorscale":
				return &a.Scale
			case "actorzclip":
				return &a.Zclip
			case "actorspeed":
				return &a.Speed
			default:
				return &a.Turn
			}
		}
		acc(name, script.Num(0), func(a *ActorInstance) script.Value { return script.Num(*field(a)) }, func(a *ActorInstance, v script.Value) error { *field(a) = v.Num(); return nil })
	}
	acc("actorvalue", script.Num(0), func(a *ActorInstance) script.Value { return a.Value }, func(a *ActorInstance, v script.Value) error { a.Value = v; return nil })
	acc("actorowner", script.Str(""), func(a *ActorInstance) script.Value { return a.Owner }, func(a *ActorInstance, v script.Value) error { a.Owner = v; return nil })
	c.v("countactors", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(s.Actors.Actors.Len())) })
	c.v("indextoactor", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.Actors.Actors.keys, numArg(a, 0, 0)-1))
	})
	c.v("countcasts", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(s.Actors.Casts.Len())) })
	c.v("indextocast", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.Actors.Casts.keys, numArg(a, 0, 0)-1))
	})
	coord := regexp.MustCompile(`^-?\d+$`)
	c.action("walktostar", func(args []script.Value, _ *script.Frame) error {
		name, dst := strArg(args, 0), strArg(args, 1)
		star := c.star(dst)
		a := s.Actors.Get(name)
		parts := strings.Split(dst, ",")
		valid := len(parts) >= 2
		for _, p := range parts {
			valid = valid && coord.MatchString(strings.TrimSpace(p))
		}
		if star == nil && a != nil && valid {
			a.StarName = "walktoxyz"
			z := 0.
			if len(parts) > 2 {
				z = propertyNumber(script.Str(parts[2]))
			}
			custom := "custom"
			s.Scheduler.StartWalk(name, propertyNumber(script.Str(parts[0])), propertyNumber(script.Str(parts[1])), z, &custom)
			return nil
		}
		if a == nil || star == nil {
			c.log("walktostar: " + name + " -> \"" + dst + "\" not found")
			return nil
		}
		a.StarName = "defer"
		arrival := strings.ToLower(dst)
		s.Scheduler.StartWalk(name, float64(star.PositionX), float64(star.PositionZ), float64(star.PositionY), &arrival)
		return nil
	})
	c.action("walktoxyz", func(args []script.Value, _ *script.Frame) error {
		name := strArg(args, 0)
		a := s.Actors.Get(name)
		if a == nil {
			return nil
		}
		a.StarName = "walktoxyz"
		custom := "custom"
		s.Scheduler.StartWalk(name, numArg(args, 1, 0), numArg(args, 2, 0), numArg(args, 3, 0), &custom)
		return nil
	})
	c.action("walkonpath", func(args []script.Value, _ *script.Frame) error {
		name, from, to := strArg(args, 0), strings.ToLower(strArg(args, 1)), strings.ToLower(strArg(args, 2))
		a := s.Actors.Get(name)
		if a == nil {
			return nil
		}
		dest := c.star(to)
		if dest == nil {
			c.log("walkonpath: star \"" + strArg(args, 2) + "\" not found")
			return nil
		}
		ok, err := c.startPathWalk(a, name, from, to)
		if err != nil || ok {
			return err
		}
		a.StarName = "defer"
		if from != "resume" {
			if start := c.star(from); start != nil {
				a.WorldX = float64(start.PositionX)
				a.WorldY = float64(start.PositionZ)
				a.WorldZ = float64(start.PositionY)
			}
		}
		s.Scheduler.StartWalk(name, float64(dest.PositionX), float64(dest.PositionZ), float64(dest.PositionY), &to)
		return nil
	})
	c.v("iswalk", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(len(a) > 0 && s.Scheduler.IsWalk(strArg(a, 0)))
	})
	c.action("stopwalk", func(a []script.Value, _ *script.Frame) error {
		if len(a) > 0 {
			s.Scheduler.StopWalk(a[0].String())
		}
		return nil
	})
	c.action("pausewalk", func(a []script.Value, _ *script.Frame) error {
		if len(a) > 0 {
			s.Scheduler.PauseWalk(a[0].String(), arg(a, 1, script.Num(1)).Truthy())
		}
		return nil
	})
	c.v("countwalks", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(s.Scheduler.Walks.Len())) })
	c.v("indextowalk", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.Scheduler.Walks.keys, numArg(a, 0, 0)-1))
	})
	c.v("walkdest", func(a []script.Value, _ *script.Frame) script.Value {
		w := s.Scheduler.Walks.Get(strings.ToLower(strArg(a, 0)))
		if w == nil {
			return script.Str("None")
		}
		if w.ArriveStar == nil {
			return script.Str("custom")
		}
		return script.Str(*w.ArriveStar)
	})
	c.action("turntodeg", func(args []script.Value, _ *script.Frame) error {
		name := strArg(args, 0)
		if s.Actors.Get(name) != nil {
			s.Scheduler.StartTurn(name, numArg(args, 1, 0))
		}
		return nil
	})
}
func (c builtinContext) startPathWalk(a *ActorInstance, name, from, to string) (bool, error) {
	b := c.s.CurrentBinding
	if b == nil {
		return false, nil
	}
	named := from != "resume"
	for _, rec := range b.Set.StarPaths {
		pa, pb := strings.ToLower(rec.A), strings.ToLower(rec.B)
		match := pa == to || pb == to
		if named {
			match = pa == from && pb == to || pb == from && pa == to
		}
		if !match {
			continue
		}
		path, err := df.ReadStarPath(b.Set.File, rec.Container, b.Set.Version)
		if err != nil {
			return false, err
		}
		points := make([]RoutePoint, len(path))
		for i, p := range path {
			points[i] = RoutePoint{X: float64(p.X), Y: float64(p.Z), Z: float64(p.Y), FromPrev: float64(p.FromPrev)}
		}
		if pa == to {
			slices.Reverse(points)
			for i := len(points) - 1; i > 0; i-- {
				points[i].FromPrev = points[i-1].FromPrev
			}
			if len(points) > 0 {
				points[0].FromPrev = 0
			}
		}
		if !named && len(points) >= 2 {
			points = resumeRoute(points, a.WorldX, a.WorldY, a.WorldZ)
		}
		if len(points) >= 2 {
			a.StarName = "walkonpath"
			c.s.Scheduler.StartWalkPath(name, points, &to)
			return true, nil
		}
		return false, nil
	}
	return false, nil
}
func resumeRoute(points []RoutePoint, x, y, z float64) []RoutePoint {
	best, dist := 0, math.Inf(1)
	distance := func(x, y, z float64) float64 { return math.Floor(math.Sqrt(x*x + y*y + z*z)) }
	for i, p := range points {
		d := distance(x-p.X, y-p.Y, z-p.Z)
		if dist > d {
			best, dist = i, d
		}
	}
	if len(points)-best == 1 {
		best = len(points) - 2
	}
	rest := slices.Clone(points[best:])
	rest[0] = RoutePoint{X: x, Y: y, Z: z}
	for i := 1; i < len(rest); i++ {
		p, q := &rest[i], rest[i-1]
		p.FromPrev = distance(p.X-q.X, p.Y-q.Y, p.Z-q.Z)
	}
	return rest
}
