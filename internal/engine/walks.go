package engine

import (
	"math"
	"slices"
	"strings"
)

type WalkPoint struct{ X, Y, Z, Cum float64 }
type RoutePoint struct{ X, Y, Z, FromPrev float64 }
type Walk struct {
	SX, SY, SZ, DX, DY, DZ, Dist, Progress float64
	Paused, TurnOnly                       bool
	ArriveStar                             *string
	TurnTo                                 *float64
	Path                                   []WalkPoint
}

func (s *Scheduler) StartWalk(name string, tx, ty, tz float64, arriveStar *string) {
	a := s.Actors.Get(name)
	if a == nil {
		return
	}
	dx, dy, dz := tx-a.WorldX, ty-a.WorldY, tz-a.WorldZ
	dist := math.Max(1, math.Floor(math.Hypot(math.Hypot(dx, dy), dz)))
	turn := float64(Bearing(dx, dy))
	s.Walks.Set(strings.ToLower(name), &Walk{SX: a.WorldX, SY: a.WorldY, SZ: a.WorldZ, DX: dx, DY: dy, DZ: dz, Dist: dist, ArriveStar: arriveStar, TurnTo: &turn})
}
func (s *Scheduler) RestoreWalk(name string, w Walk) bool {
	key := strings.ToLower(name)
	if s.Actors.Get(key) == nil {
		return false
	}
	w.Dist = math.Max(1, w.Dist)
	s.Walks.Set(key, &w)
	return true
}
func (s *Scheduler) StartWalkPath(name string, points []RoutePoint, arriveStar *string) {
	a := s.Actors.Get(name)
	if a == nil {
		return
	}
	if len(points) < 2 {
		if len(points) == 1 {
			p := points[0]
			s.StartWalk(name, p.X, p.Y, p.Z, arriveStar)
		}
		return
	}
	path := make([]WalkPoint, len(points))
	cum := 0.
	for i, p := range points {
		if i > 0 {
			cum += p.FromPrev
		}
		path[i] = WalkPoint{p.X, p.Y, p.Z, cum}
	}
	turn := float64(Bearing(path[1].X-path[0].X, path[1].Y-path[0].Y))
	s.Walks.Set(strings.ToLower(name), &Walk{SX: a.WorldX, SY: a.WorldY, SZ: a.WorldZ, Dist: math.Max(1, math.Floor(cum)), ArriveStar: arriveStar, TurnTo: &turn, Path: path})
}
func (s *Scheduler) StartTurn(name string, deg float64) {
	a := s.Actors.Get(name)
	if a == nil {
		return
	}
	target := float64(int32JS(deg) & 255)
	if float64(int32JS(a.Deg)&255) == target {
		return
	}
	key := strings.ToLower(name)
	if s.Walks.Has(key) {
		a.Deg = target
		return
	}
	star := a.StarName
	s.Walks.Set(key, &Walk{SX: a.WorldX, SY: a.WorldY, SZ: a.WorldZ, Dist: 1, ArriveStar: &star, TurnTo: &target, TurnOnly: true})
}
func stepDegree(cur, target, by float64) float64 {
	step := math.Max(1, math.Abs(math.Trunc(by)))
	diff := float64(int32JS(target-cur) & 255)
	if diff == 0 || diff <= step || 256-diff <= step {
		return target
	}
	if diff >= 128 {
		step = -step
	}
	return float64(int32JS(cur+step) & 255)
}
func (s *Scheduler) StopWalk(name string) {
	key := strings.ToLower(name)
	a := s.Actors.Get(key)
	if s.Walks.Has(key) {
		s.Walks.Delete(key)
		if a != nil && a.PoseName == "walk" {
			a.PoseName = "stand"
			a.Step = 0
		}
	}
}
func (s *Scheduler) IsWalk(name string) bool { return s.Walks.Has(strings.ToLower(name)) }
func (s *Scheduler) Turning(name *string) bool {
	if name != nil {
		w := s.Walks.Get(strings.ToLower(*name))
		return w != nil && w.TurnOnly
	}
	for _, w := range s.Walks.All() {
		if w.TurnOnly {
			return true
		}
	}
	return false
}
func (s *Scheduler) AnyoneMoving() bool { return s.Walks.Len() > 0 }
func (s *Scheduler) PauseWalk(name string, on bool) {
	if w := s.Walks.Get(strings.ToLower(name)); w != nil {
		w.Paused = on
	}
}
func (s *Scheduler) serviceWalks() {
	arrived := []string{}
	// Removing a completed walk must not skip the following entry.
	for _, key := range slices.Clone(s.Walks.keys) {
		w := s.Walks.Get(key)
		if w == nil || w.Paused {
			continue
		}
		a := s.Actors.Get(key)
		if a == nil {
			s.Walks.Delete(key)
			continue
		}
		if w.TurnTo != nil {
			turned := stepDegree(a.Deg, *w.TurnTo, a.Turn)
			done := turned == *w.TurnTo
			a.Deg = turned
			if done && w.TurnOnly {
				s.Walks.Delete(key)
				continue
			}
			if done {
				w.TurnTo = nil
				s.Host.Track("actor endturn", false, func(task *Task) error { return s.Host.SendEvent(task, "sendtoactor", key, "endturn", "walk") })
			}
			continue
		}
		w.Progress += math.Max(1, a.Speed)
		t := math.Min(1, w.Progress/w.Dist)
		if w.Path != nil && len(w.Path) >= 2 {
			at := t * w.Path[len(w.Path)-1].Cum
			i := 1
			for i < len(w.Path)-1 && w.Path[i].Cum < at {
				i++
			}
			from, to := w.Path[i-1], w.Path[i]
			leg := to.Cum - from.Cum
			u := 1.
			if leg > 0 {
				u = math.Min(1, math.Max(0, (at-from.Cum)/leg))
			}
			a.WorldX = jsRound(from.X + (to.X-from.X)*u)
			a.WorldY = jsRound(from.Y + (to.Y-from.Y)*u)
			a.WorldZ = jsRound(from.Z + (to.Z-from.Z)*u)
			a.Deg = float64(Bearing(to.X-from.X, to.Y-from.Y))
		} else {
			a.WorldX = jsRound(w.SX + w.DX*t)
			a.WorldY = jsRound(w.SY + w.DY*t)
			a.WorldZ = jsRound(w.SZ + w.DZ*t)
		}
		if t >= 1 {
			s.Walks.Delete(key)
			if a.PoseName == "walk" {
				a.PoseName = "stand"
				a.Step = 0
			}
			if w.ArriveStar != nil {
				a.StarName = *w.ArriveStar
			}
			arrived = append(arrived, key)
		}
	}
	for _, key := range arrived {
		if s.Host.HasHandler("actor", key, "endwalk") {
			s.Host.Track("actor endwalk", false, func(task *Task) error { return s.Host.SendEvent(task, "sendtoactor", key, "endwalk", "walk") })
		}
	}
}
