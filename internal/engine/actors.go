package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"sort"
	"strings"
)

type LoadedCast struct {
	Name string
	Cst  *df.Cast
	spriteCache
	memberIDs map[*df.CastMember]uint64
}
type ActorInstance struct {
	Name, SetName, PoseName, StarName                                                        string
	Visible, WorldSpace, StarPending                                                         bool
	WorldX, WorldY, WorldZ, AnchorX, AnchorY, Dist, Deg, Scale, Zclip, Speed, Turn, LastTick float64
	Step                                                                                     int
	Owner, Value                                                                             script.Value
	Member                                                                                   *df.CastMember
	Cast                                                                                     *LoadedCast
}

func NewActor(member *df.CastMember, cast *LoadedCast) *ActorInstance {
	return &ActorInstance{Member: member, Cast: cast, WorldSpace: true, PoseName: "stand", Turn: 16, Owner: script.Str("none")}
}
func (a *ActorInstance) Pose() *df.CastPose {
	for i := range a.Member.Poses {
		p := &a.Member.Poses[i]
		if p.Name == a.PoseName {
			return p
		}
	}
	if len(a.Member.Poses) > 0 {
		return &a.Member.Poses[0]
	}
	return nil
}

type ActorRuntime struct {
	Actors     orderedMap[*ActorInstance]
	Casts      orderedMap[*LoadedCast]
	CurrentSet string
}

func (r *ActorRuntime) AddCast(name string, cst *df.Cast) *LoadedCast {
	cast := &LoadedCast{Name: strings.ToLower(name), Cst: cst, spriteCache: spriteCache{file: cst.File}, memberIDs: map[*df.CastMember]uint64{}}
	r.Casts.Set(cast.Name, cast)
	for i := range cst.Members {
		m := &cst.Members[i]
		cast.memberIDs[m] = newDrawID()
		if !r.Actors.Has(m.Name) {
			a := NewActor(m, cast)
			a.Name = m.Name
			r.Actors.Set(m.Name, a)
		}
	}
	return cast
}
func (r *ActorRuntime) RemoveCast(name string) {
	cast := r.Casts.Get(strings.ToLower(name))
	if cast == nil {
		return
	}
	r.Casts.Delete(cast.Name)
	keys := []string{}
	for k, a := range r.Actors.All() {
		if a.Cast == cast {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		r.Actors.Delete(k)
	}
}
func (r *ActorRuntime) Get(name string) *ActorInstance { return r.Actors.Get(strings.ToLower(name)) }
func (r *ActorRuntime) Remove(name string)             { r.Actors.Delete(strings.ToLower(name)) }
func (r *ActorRuntime) Instance(src, dst string) {
	s := r.Get(src)
	if s == nil || r.Get(dst) != nil {
		return
	}
	a := NewActor(s.Member, s.Cast)
	a.Name = dst
	r.Actors.Set(strings.ToLower(dst), a)
}
func (r *ActorRuntime) SettleStars(stars []StarPoint, walking func(string) bool) int {
	moved := 0
	for name, a := range r.Actors.All() {
		if !a.StarPending || strings.ToLower(a.SetName) != r.CurrentSet || walking != nil && walking(name) {
			continue
		}
		for _, s := range stars {
			if strings.ToLower(s.Identifier) != strings.ToLower(a.StarName) {
				continue
			}
			a.WorldX, a.WorldY, a.WorldZ = s.PositionX, s.PositionZ, s.PositionY
			a.StarPending = false
			moved++
			break
		}
	}
	return moved
}
func (r *ActorRuntime) AdvanceAnimation() {
	for _, a := range r.Actors.All() {
		p := a.Pose()
		if p != nil && len(p.Play) > 0 {
			a.Step = (a.Step + 1) % len(p.Play)
		}
	}
}
func angleApart(a, b int) int { d := (a - b) & 255; return min(d, 256-d) }
func (r *ActorRuntime) FrameFor(a *ActorInstance, cam *WorldCamera) (*df.Sprite, error) {
	p := a.Pose()
	if p == nil || len(p.Play) == 0 || a.Step < 0 {
		return nil, nil
	}
	idx := p.Play[a.Step%len(p.Play)]
	if idx < 0 || idx >= len(p.Steps) || len(p.Steps[idx]) == 0 {
		return nil, nil
	}
	step := p.Steps[idx]
	deg := a.Deg
	if cam != nil {
		deg -= float64(Bearing(cam.X-a.WorldX, cam.Y-a.WorldY))
	}
	rel := int(int32JS(deg) & 255)
	cf := step[0]
	best := angleApart(cf.Angle, rel)
	for _, f := range step {
		d := angleApart(f.Angle, rel)
		if d < best {
			best, cf = d, f
		}
	}
	return a.Cast.Frame(cf.Location)
}
func actorRefScale(a *ActorInstance) float64 {
	p := a.Pose()
	if p != nil && len(p.Steps) > 0 && len(p.Steps[0]) > 0 && p.Steps[0][0].RefScale != 0 {
		return float64(p.Steps[0][0].RefScale)
	}
	return 96
}

type ActorDrawEntry struct {
	A    *ActorInstance
	Proj Projection
}

func (r *ActorRuntime) DrawList(cam WorldCamera) []ActorDrawEntry {
	out := []ActorDrawEntry{}
	for _, a := range r.Actors.All() {
		if !a.Visible || a.Scale <= 0 || a.SetName != "" && a.SetName != r.CurrentSet {
			continue
		}
		proj := ProjectPoint(cam, a.WorldX, a.WorldY, a.WorldZ)
		if proj != nil {
			out = append(out, ActorDrawEntry{a, *proj})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Proj.Depth > out[j].Proj.Depth })
	return out
}
func (r *ActorRuntime) Rect(e ActorDrawEntry, cam WorldCamera) (*SpriteRect, error) {
	f, err := r.FrameFor(e.A, &cam)
	return worldSprite(f, e.Proj, float64(e.A.Scale*actorRefScale(e.A))/(float64(1000*e.Proj.Depth))), err
}
func (r *ActorRuntime) ScreenRect(a *ActorInstance) (*SpriteRect, error) {
	f, err := r.FrameFor(a, nil)
	return screenSprite(f, a.AnchorX, a.AnchorY), err
}
func (r *ActorRuntime) OnScreen(a *ActorInstance, cam WorldCamera) (bool, error) {
	if !a.Visible || a.SetName != "" && a.SetName != r.CurrentSet {
		return false, nil
	}
	if !a.WorldSpace {
		rect, err := r.ScreenRect(a)
		return rect.intersects(cam.ClipW, cam.ClipH), err
	}
	if a.Scale <= 0 {
		return false, nil
	}
	proj := ProjectPoint(cam, a.WorldX, a.WorldY, a.WorldZ)
	if proj == nil {
		return false, nil
	}
	rect, err := r.Rect(ActorDrawEntry{a, *proj}, cam)
	return rect.intersects(cam.ClipW, cam.ClipH), err
}
func (r *ActorRuntime) ScreenDrawList() ([]*ActorInstance, error) {
	out := []*ActorInstance{}
	for _, a := range r.Actors.All() {
		if !a.Visible || a.WorldSpace {
			continue
		}
		f, err := r.FrameFor(a, nil)
		if err != nil {
			return nil, err
		}
		if f != nil && (a.SetName == "" || a.SetName == r.CurrentSet) {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Dist > out[j].Dist })
	return out, nil
}
func (r *ActorRuntime) CompositeScreen(rgba []byte, w, h int, palette []byte, hd ...*HDSurface) error {
	list, err := r.ScreenDrawList()
	if err != nil {
		return err
	}
	for _, a := range list {
		rect, err := r.ScreenRect(a)
		if err != nil {
			return err
		}
		compositeSprite(rect, rgba, w, h, palette, w, h, nil, 0, hd...)
	}
	return nil
}
func (r *ActorRuntime) Composite(rgba []byte, w, h int, palette []byte, cam WorldCamera, occ *Occlusion, hd ...*HDSurface) error {
	for _, e := range r.DrawList(cam) {
		if err := r.CompositeOne(e, rgba, w, h, palette, cam, occ, hd...); err != nil {
			return err
		}
	}
	return nil
}
func (r *ActorRuntime) CompositeOne(e ActorDrawEntry, rgba []byte, w, h int, palette []byte, cam WorldCamera, occ *Occlusion, hd ...*HDSurface) error {
	rect, err := r.Rect(e, cam)
	if err != nil {
		return err
	}
	compositeSprite(rect, rgba, w, h, palette, cam.ClipW, cam.ClipH, occ, occlusionLevel(e.Proj.Depth, e.A.Zclip, occ), hd...)
	return nil
}
func (r *ActorRuntime) ActorAt(x, y int, cam *WorldCamera, occ *Occlusion) (*ActorInstance, error) {
	return r.ActorAtPoint(float64(x), float64(y), cam, occ)
}
func (r *ActorRuntime) ActorAtPoint(x, y float64, cam *WorldCamera, occ *Occlusion) (*ActorInstance, error) {
	screen, err := r.ScreenDrawList()
	if err != nil {
		return nil, err
	}
	for i := len(screen) - 1; i >= 0; i-- {
		rect, err := r.ScreenRect(screen[i])
		if err != nil {
			return nil, err
		}
		if rect.sample(x, y) >= 0 {
			return screen[i], nil
		}
	}
	if cam == nil {
		return nil, nil
	}
	list := r.DrawList(*cam)
	for i := len(list) - 1; i >= 0; i-- {
		e := list[i]
		rect, err := r.Rect(e, *cam)
		if err != nil {
			return nil, err
		}
		if rect.sample(x, y) >= 0 && !SceneryOccludesPoint(occ, x, y, occlusionLevel(e.Proj.Depth, e.A.Zclip, occ)) {
			return e.A, nil
		}
	}
	return nil, nil
}
func (r *ActorRuntime) DrawSignature(s *DrawSignature) {
	s.Num(float64(r.Actors.Len())).Str(r.CurrentSet)
	for _, a := range r.Actors.All() {
		s.Bool(a.Visible)
		if !a.Visible {
			continue
		}
		s.ID(a.Cast.memberIDs[a.Member]).Str(a.SetName).Str(a.PoseName).Num(a.WorldX).Num(a.WorldY).Num(a.WorldZ).Num(a.Deg).Num(float64(a.Step)).Num(a.Scale).Num(a.Zclip).Bool(a.WorldSpace).Num(a.AnchorX).Num(a.AnchorY).Num(a.Dist)
	}
}
