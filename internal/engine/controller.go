package engine

import (
	"fmt"
	"math"
	"sort"
)

// Controller targets describe the existing mouse interface. Activating one still
// goes through normal press/release dispatch, including script and input locks.
type ControllerTarget struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	AimX  float64 `json:"aim_x"`
	AimY  float64 `json:"aim_y"`
}
type ControllerSurface struct {
	Context string             `json:"context"`
	Key     string             `json:"key"`
	Targets []ControllerTarget `json:"targets"`
}

func (p *Player) ControllerSurface(includeRoom bool) ControllerSurface {
	out := ControllerSurface{Context: "busy", Targets: []ControllerTarget{}}
	if p.Host == nil || !p.Ready || p.Paused {
		return out
	}
	d, s := p.Host.Director, p.Host.Session
	out.Key = fmt.Sprintf("%s/%s/%s/%s/%s", s.CurrentSetFile, s.CurrentSceneName(), s.CurrentViewName(), s.StageCtrl.Name, s.StageCtrl.CurrentFlat)
	add := func(id, label string, x, y, w, h float64) {
		x1, y1 := math.Min(ScreenWidth, x+w), math.Min(ScreenHeight, y+h)
		x, y = math.Max(0, x), math.Max(0, y)
		if x1 <= x || y1 <= y {
			return
		}
		out.Targets = append(out.Targets, ControllerTarget{id, label, x, y, x1 - x, y1 - y, math.Floor((x + x1) / 2), math.Floor((y + y1) / 2)})
	}
	if d.Movies.Playing() {
		out.Context = "movie"
		out.Key = d.Movies.PlayingFile()
		for i, r := range d.Movies.WaitingRegions() {
			seg := d.Movies.active.seg
			add(fmt.Sprintf("movie:%d", i), r.Target, float64(seg.OriginX+min(r.X0, r.X1)), float64(seg.OriginY+min(r.Y0, r.Y1)), float64(max(r.X0, r.X1)-min(r.X0, r.X1)+1), float64(max(r.Y0, r.Y1)-min(r.Y0, r.Y1)+1))
		}
		return out
	}
	if d.AwaitingChoice() {
		out.Context = "dialogue"
		rects := d.ChoiceRects()
		for i, c := range d.Choices() {
			if i < len(rects) {
				r := rects[i]
				add(fmt.Sprintf("choice:%d:%g", i, c.ID), c.Text, r.X, r.Y, r.W, r.H)
			}
		}
		return out
	}
	if d.InputLocked() {
		return out
	}
	out.Context = "panel"
	if s.ViewShowing() {
		out.Context = "room"
		if !includeRoom {
			return out
		}
	}
	// Only retain points that the ordinary hit tester resolves to this object.
	// This prevents selecting transparent or occluded parts of a sprite.
	hitTarget := func(kind, name string, x, y, w, h float64, sprite *SpriteRect) {
		n := len(out.Targets)
		add(kind+":"+name, name, x, y, w, h)
		if len(out.Targets) == n {
			return
		}
		target := &out.Targets[n]
		points := [][2]float64{{target.AimX, target.AimY}}
		for row := 0; row < 7; row++ {
			for col := 0; col < 7; col++ {
				points = append(points, [2]float64{math.Floor(target.X + (float64(col)+0.5)*target.W/7), math.Floor(target.Y + (float64(row)+0.5)*target.H/7)})
			}
		}
		for _, pt := range points {
			if sprite != nil && sprite.sample(pt[0], pt[1]) < 0 {
				continue
			}
			hit, err := d.HitTestAt(pt[0], pt[1])
			if err == nil && hit.Type == kind && hit.Name == name {
				target.AimX, target.AimY = pt[0], pt[1]
				return
			}
		}
		out.Targets = out.Targets[:n]
	}
	spriteTarget := func(kind, name string, r *SpriteRect) {
		if r != nil {
			hitTarget(kind, name, r.X, r.Y, float64(r.W), float64(r.H), r)
		}
	}
	for _, prop := range s.Props.ScreenDrawList(s.ViewShowing()) {
		name := prop.Name
		if name == "" {
			name = prop.Group.Name
		}
		if r, err := prop.ScreenRect(); err == nil {
			spriteTarget("prop", name, r)
		}
	}
	if s.ViewShowing() && p.Host.Viewer != nil {
		v := p.Host.Viewer
		if cam := v.WorldCamera(); cam != nil {
			for _, e := range s.Props.WorldDrawList(*cam) {
				if r, err := s.Props.WorldRect(e, *cam); err == nil {
					spriteTarget("prop", e.P.Name, r)
				}
			}
			for _, e := range s.Actors.DrawList(*cam) {
				if r, err := s.Actors.Rect(e, *cam); err == nil {
					spriteTarget("actor", e.A.Name, r)
				}
			}
		}
		for _, o := range v.View().Objects {
			x, y := float64(min(o.StartRegionX, o.EndRegionX)), float64(min(o.StartRegionY, o.EndRegionY))
			hitTarget("painting", o.Identifier, x, y, float64(max(o.StartRegionX, o.EndRegionX))-x+1, float64(max(o.StartRegionY, o.EndRegionY))-y+1, nil)
		}
	}
	if s.StageOpen() && s.StageCtrl.CurrentFlat != "none" {
		for _, r := range s.StageCtrl.CurrentFlatRegions() {
			hitTarget("button", r.Name, float64(r.Left), float64(r.Top), float64(r.Right-r.Left+1), float64(r.Bottom-r.Top+1), nil)
		}
	}
	sort.SliceStable(out.Targets, func(i, j int) bool {
		a, b := out.Targets[i], out.Targets[j]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	return out
}
