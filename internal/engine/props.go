package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"slices"
	"sort"
	"strings"
)

func FrameIndexForDegree(st *df.PropState, deg float64) int {
	target := math.Mod(math.Mod(jsRound(deg), 256)+256, 256)
	best, dist := 0, math.Inf(1)
	for i, d := range st.Degrees {
		diff := math.Mod(math.Mod(float64(d)-target, 256)+256, 256)
		n := math.Min(diff, 256-diff)
		if n < dist {
			best, dist = i, n
		}
	}
	return best
}
func IsDegreeSelector(st *df.PropState) bool {
	return !st.Animated && len(st.Degrees) == len(st.Frames)
}
func DegreeVariantFrames(st *df.PropState, deg float64) []int {
	if len(st.Degrees) != len(st.Frames) || len(st.Frames) < 3 {
		return nil
	}
	groups := map[int][]int{}
	for i, d := range st.Degrees {
		if d >= 8 {
			return nil
		}
		groups[d] = append(groups[d], i)
	}
	if len(groups) < 2 {
		return nil
	}
	size := len(groups[st.Degrees[0]])
	if size < 2 {
		return nil
	}
	for _, g := range groups {
		if len(g) != size {
			return nil
		}
	}
	return groups[st.Degrees[FrameIndexForDegree(st, deg)]]
}
func PlaySequence(st *df.PropState, variant []int) []int {
	if st.PlayOrder == nil {
		return variant
	}
	if variant == nil {
		return st.PlayOrder
	}
	relative, absolute := true, true
	for _, i := range st.PlayOrder {
		if i < 0 || i >= len(variant) {
			relative = false
		}
		if !slices.Contains(variant, i) {
			absolute = false
		}
	}
	if relative {
		out := make([]int, len(st.PlayOrder))
		for i, v := range st.PlayOrder {
			out[i] = variant[v]
		}
		return out
	}
	if absolute {
		return st.PlayOrder
	}
	return variant
}

type LoadedShop struct {
	Name       string
	Shp        *df.Shop
	Persistent bool
	spriteCache
	groupIDs map[*df.PropGroup]uint64
}
type PropInstance struct {
	Name, StateName, StarName, SetName                                            string
	Visible, Hidden, ScreenPlaced, WorldSpace, StarPending, Directional           bool
	AnchorX, AnchorY, Dist, Speed, WorldX, WorldY, WorldZ, Scale, Zclip, LastTick float64
	Owner, Value, Deg                                                             script.Value
	FrameIdx                                                                      int
	FrameOrder                                                                    []int
	FrameLocked, DegVariants, Animating                                           bool
	DegEvent                                                                      float64
	Group                                                                         *df.PropGroup
	Shop                                                                          *LoadedShop
}

func NewProp(group *df.PropGroup, shop *LoadedShop) *PropInstance {
	return &PropInstance{Group: group, Shop: shop, AnchorX: 256, AnchorY: 192, Owner: script.Str("none"), DegEvent: -1}
}
func (p *PropInstance) State() *df.PropState {
	if p.StateName == "" {
		if len(p.Group.States) > 0 {
			return &p.Group.States[0]
		}
		return nil
	}
	for i := range p.Group.States {
		s := &p.Group.States[i]
		if strings.ToLower(s.Identifier) == p.StateName {
			return s
		}
	}
	return nil
}
func (p *PropInstance) FrameCount(st *df.PropState) int {
	if p.FrameOrder != nil {
		return len(p.FrameOrder)
	}
	return len(st.Frames)
}
func (p *PropInstance) CurrentFrameIdx(st *df.PropState) int {
	if p.FrameOrder == nil && IsDegreeSelector(st) {
		return FrameIndexForDegree(st, propertyNumberOrZero(p.Deg))
	}
	i := min(p.FrameIdx, p.FrameCount(st)-1)
	if p.FrameOrder != nil {
		if i < 0 {
			return -1
		}
		return p.FrameOrder[i]
	}
	return i
}
func (p *PropInstance) frame(st *df.PropState, idx int) (*df.Sprite, error) {
	if idx < 0 || idx >= len(st.Frames) {
		return nil, nil
	}
	return p.Shop.Frame(st.Frames[idx])
}
func (p *PropInstance) ScreenRect() (*SpriteRect, error) {
	st := p.State()
	if st == nil || len(st.Frames) == 0 {
		return nil, nil
	}
	f, err := p.frame(st, p.CurrentFrameIdx(st))
	return screenSprite(f, p.AnchorX, p.AnchorY), err
}
func BecomeWorldProp(p *PropInstance, v1 bool) {
	p.WorldSpace = true
	p.FrameLocked = false
	p.DegVariants = false
	p.FrameOrder = nil
	if v1 && p.Scale == 0 {
		p.Scale = 1000
	}
}

type PropRuntime struct {
	Props      orderedMap[*PropInstance]
	Shops      orderedMap[*LoadedShop]
	CurrentSet string
}

func (r *PropRuntime) AddShop(name string, shp *df.Shop) *LoadedShop {
	shop := &LoadedShop{Name: strings.ToLower(name), Shp: shp, spriteCache: spriteCache{file: shp.File}, groupIDs: map[*df.PropGroup]uint64{}}
	r.Shops.Set(shop.Name, shop)
	for i := range shp.Groups {
		g := &shp.Groups[i]
		shop.groupIDs[g] = newDrawID()
		p := NewProp(g, shop)
		p.Name = g.Name
		r.Props.Set(strings.ToLower(g.Name), p)
	}
	return shop
}
func (r *PropRuntime) RemoveShop(name string) {
	shop := r.Shops.Get(strings.ToLower(name))
	if shop == nil {
		return
	}
	r.Shops.Delete(shop.Name)
	keys := []string{}
	for key, p := range r.Props.All() {
		if p.Shop == shop {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		r.Props.Delete(key)
	}
}
func (r *PropRuntime) Get(name string) *PropInstance { return r.Props.Get(strings.ToLower(name)) }
func (r *PropRuntime) Remove(name string)            { r.Props.Delete(strings.ToLower(name)) }
func (r *PropRuntime) SettleStars(stars []StarPoint, v1 bool) int {
	moved := 0
	for _, p := range r.Props.All() {
		if !p.StarPending || p.SetName != r.CurrentSet {
			continue
		}
		for _, s := range stars {
			if strings.ToLower(s.Identifier) != p.StarName {
				continue
			}
			BecomeWorldProp(p, v1)
			p.WorldX, p.WorldY, p.WorldZ = s.PositionX, s.PositionZ, s.PositionY
			p.StarPending = false
			moved++
			break
		}
	}
	return moved
}
func (r *PropRuntime) Instance(src, dst string) {
	s := r.Get(src)
	if s == nil || r.Get(dst) != nil {
		return
	}
	p := NewProp(s.Group, s.Shop)
	p.Name = dst
	p.Visible, p.StateName, p.Deg, p.Dist, p.WorldSpace, p.Directional, p.SetName = s.Visible, s.StateName, s.Deg, s.Dist, s.WorldSpace, s.Directional, s.SetName
	r.Props.Set(strings.ToLower(dst), p)
}
func (r *PropRuntime) Tick(now, frameMs float64) {
	for _, p := range r.Props.All() {
		if !p.Visible || p.FrameLocked || !p.Animating {
			continue
		}
		st := p.State()
		if st == nil || p.FrameCount(st) < 2 || p.FrameIdx >= p.FrameCount(st)-1 {
			p.Animating = false
			continue
		}
		if p.LastTick == 0 {
			p.LastTick = now
		}
		if now-p.LastTick >= frameMs {
			p.LastTick = now
			p.FrameIdx++
			if p.FrameIdx >= p.FrameCount(st)-1 {
				p.Animating = false
			}
		}
	}
}
func (r *PropRuntime) ScreenDrawList(persistentOnly bool) []*PropInstance {
	out := []*PropInstance{}
	for _, p := range r.Props.All() {
		st := p.State()
		if p.Visible && !p.Hidden && !p.WorldSpace && st != nil && len(st.Frames) > 0 && (!persistentOnly || p.Shop.Persistent) {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Dist > out[j].Dist })
	return out
}

type PropDrawEntry struct {
	P    *PropInstance
	Proj Projection
}

func (r *PropRuntime) WorldDrawList(cam WorldCamera) []PropDrawEntry {
	out := []PropDrawEntry{}
	for _, p := range r.Props.All() {
		st := p.State()
		if !p.Visible || p.Hidden || !p.WorldSpace || st == nil || len(st.Frames) == 0 || p.Scale <= 0 || p.SetName != "" && p.SetName != r.CurrentSet {
			continue
		}
		proj := ProjectPoint(cam, p.WorldX, p.WorldY, p.WorldZ)
		if proj != nil {
			out = append(out, PropDrawEntry{p, *proj})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Proj.Depth > out[j].Proj.Depth })
	return out
}
func (r *PropRuntime) WorldRect(e PropDrawEntry, cam WorldCamera) (*SpriteRect, error) {
	p, st := e.P, e.P.State()
	if st == nil {
		return nil, nil
	}
	idx := p.CurrentFrameIdx(st)
	if p.Directional && len(st.Frames) >= 2 {
		idx = FrameIndexForDegree(st, float64(int32JS(propertyNumber(p.Deg)-float64(Bearing(cam.X-p.WorldX, cam.Y-p.WorldY)))&255))
	}
	f, err := p.frame(st, idx)
	scale := 96
	if idx >= 0 && idx < len(st.RefScales) {
		scale = st.RefScales[idx]
	}
	return worldSprite(f, e.Proj, p.Scale*float64(scale)/(1000*e.Proj.Depth)), err
}
func occlusionLevel(depth, zclip float64, occ *Occlusion) float64 {
	if occ == nil {
		return 0
	}
	return DepthLevel(depth-zclip+occ.GroundBias, occ)
}
func (r *PropRuntime) PropAt(x, y int, cam *WorldCamera, persistentOnly bool, occ *Occlusion) (*PropInstance, error) {
	return r.PropAtPoint(float64(x), float64(y), cam, persistentOnly, occ)
}
func (r *PropRuntime) PropAtPoint(x, y float64, cam *WorldCamera, persistentOnly bool, occ *Occlusion) (*PropInstance, error) {
	screen := r.ScreenDrawList(persistentOnly)
	for i := len(screen) - 1; i >= 0; i-- {
		rect, err := screen[i].ScreenRect()
		if err != nil {
			return nil, err
		}
		if rect.sample(x, y) >= 0 {
			return screen[i], nil
		}
	}
	if cam != nil {
		world := r.WorldDrawList(*cam)
		for i := len(world) - 1; i >= 0; i-- {
			e := world[i]
			rect, err := r.WorldRect(e, *cam)
			if err != nil {
				return nil, err
			}
			if rect.sample(x, y) >= 0 && !SceneryOccludesPoint(occ, x, y, occlusionLevel(e.Proj.Depth, e.P.Zclip, occ)) {
				return e.P, nil
			}
		}
	}
	return nil, nil
}
func (r *PropRuntime) CompositeWorldOne(e PropDrawEntry, rgba []byte, w, h int, palette []byte, cam WorldCamera, occ *Occlusion) error {
	rect, err := r.WorldRect(e, cam)
	if err != nil {
		return err
	}
	compositeSprite(rect, rgba, w, h, palette, cam.ClipW, cam.ClipH, occ, occlusionLevel(e.Proj.Depth, e.P.Zclip, occ))
	return nil
}
func (r *PropRuntime) Composite(rgba []byte, w, h int, palette []byte, minAnchorY float64, cam *WorldCamera, persistentOnly bool, occ *Occlusion) error {
	if cam != nil {
		for _, e := range r.WorldDrawList(*cam) {
			if err := r.CompositeWorldOne(e, rgba, w, h, palette, *cam, occ); err != nil {
				return err
			}
		}
	}
	for _, p := range r.ScreenDrawList(persistentOnly) {
		if p.AnchorY < minAnchorY {
			continue
		}
		rect, err := p.ScreenRect()
		if err != nil {
			return err
		}
		compositeSprite(rect, rgba, w, h, palette, w, h, nil, 0)
	}
	return nil
}
func (r *PropRuntime) DrawSignature(s *DrawSignature) {
	s.Num(float64(r.Props.Len())).Str(r.CurrentSet)
	for _, p := range r.Props.All() {
		s.Bool(p.Visible).Bool(p.Hidden)
		if !p.Visible {
			continue
		}
		s.ID(p.Shop.groupIDs[p.Group]).Bool(p.Shop.Persistent).Str(p.StateName).Num(float64(p.FrameIdx))
		n := -1
		if p.FrameOrder != nil {
			n = len(p.FrameOrder)
		}
		s.Num(float64(n))
		for _, i := range p.FrameOrder {
			s.Num(float64(i))
		}
		s.Num(p.AnchorX).Num(p.AnchorY).Num(p.Dist).Bool(p.WorldSpace).Bool(p.Directional).Value(p.Deg)
		if p.WorldSpace {
			s.Str(p.SetName).Num(p.WorldX).Num(p.WorldY).Num(p.WorldZ).Num(p.Scale).Num(p.Zclip)
		}
	}
}
