package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"math"
	"slices"
	"strings"
)

const RightTurns = 0
const LeftTurns = 1
const maxFadeWaitTicks = 240

// ViewerDirector owns modal input and the final composite. The viewer owns the
// current room, its authored navigation and the scripts attached to its views.
type ViewerDirector interface {
	Busy() bool
	MovingCamera() bool
	KeyDown(string, bool) (bool, error)
	Click(float64, float64) error
}
type NavHooks struct {
	OnNavigate, OnSceneJump, OnViewJump func(string)
	Active                              bool
}
type SetViewer struct {
	Set                                                *df.Set
	Session                                            *Session
	Scripts                                            *SetScripts
	Director                                           ViewerDirector
	Gamma                                              *ScreenGamma
	Rings                                              *RingCache
	SceneIdx, ViewIdx                                  int
	ShowMap, ShowHotspots                              bool
	OnHUD, Log                                         func(string)
	palette, propPalette, basePalette, basePropPalette []byte
	paletteID, propPaletteID, paletteGeneration        uint64
	setDim                                             *ClutDim
	worldPalette                                       struct {
		stage      []byte
		dim        *ClutDim
		generation uint64
		out        []byte
	}
	warmDone                bool
	pendingJumpScene        *string
	animation               []*CachedFrame
	animationPos            int
	animationPace, lastTick float64
	animationDone           func() error
	current                 *CachedFrame
}

func NewSetViewer(set *df.Set, s *Session, startScene, startView string, gamma *ScreenGamma, director ViewerDirector) (*SetViewer, error) {
	if len(set.Scenes) == 0 {
		return nil, fmt.Errorf("set %q has no scenes", set.SetName)
	}
	for _, scene := range set.Scenes {
		if len(scene.Views) == 0 {
			return nil, fmt.Errorf("set %q scene %q has no views", set.SetName, scene.SceneName)
		}
	}
	if gamma == nil {
		gamma = NewScreenGamma()
	}
	v := &SetViewer{Set: set, Session: s, Director: director, Gamma: gamma, Rings: NewRingCache(set), Log: func(string) {}, OnHUD: func(string) {}}
	v.basePalette = df.PaletteRGBA(set.PaletteRaw, set.ColorCount, set.File.Order)
	v.basePropPalette = df.PaletteRGBA(set.PaletteRaw, 256, set.File.Order)
	v.updatePalettes()
	v.paletteGeneration = gamma.Generation
	v.Scripts = NewSetScripts(set, s)
	v.Scripts.Log = func(line string) { v.Log(line) }
	s.CurrentSceneName = func() string { return strings.ToLower(v.Scene().SceneName) }
	s.CurrentViewName = func() string {
		if v.Animating() {
			return "moving"
		}
		return strings.ToLower(v.View().ViewName)
	}
	s.CurrentRotation = func() float64 { return v.View().Rotation }
	s.NavDriver = v.Navigate
	s.SceneJumpDriver = v.SceneJump
	s.ViewJumpDriver = v.ViewJump
	s.Props.CurrentSet = s.SetName
	s.Actors.CurrentSet = s.SetName
	stars := make([]StarPoint, len(set.Actors))
	for i, a := range set.Actors {
		s.StarRegistry.Set(strings.ToLower(a.Identifier), a)
		stars[i] = StarPoint{Identifier: a.Identifier, PositionX: float64(a.PositionX), PositionY: float64(a.PositionY), PositionZ: float64(a.PositionZ)}
	}
	s.Actors.SettleStars(stars, s.Scheduler.IsWalk)
	s.Props.SettleStars(stars, set.Version == 1)
	s.Listener = func() *Listener {
		sc, view := v.Scene(), v.View()
		return &Listener{X: float64(sc.XAxisMap), Y: float64(sc.ZAxisMap), Deg: float64(view.Rotation8)}
	}
	s.ActiveCamera = v.RoomCamera
	for i, scene := range set.Scenes {
		if scene.SceneName == set.DefaultSceneName {
			v.SceneIdx = i
			break
		}
	}
	for i, view := range v.Scene().Views {
		if view.ViewName == set.DefaultViewName {
			v.ViewIdx = i
			break
		}
	}
	if err := v.ShowView(); err != nil {
		return nil, err
	}
	if startScene != "" {
		if _, err := v.JumpTo(startScene, startView); err != nil {
			return nil, err
		}
	}
	base := strings.ToLower(set.SetName)
	for _, bank := range []string{base + ".trk", base + ".sfx", base + ".11k", "unilib.trk"} {
		data, err := s.Read(bank)
		if err != nil {
			return nil, err
		}
		if data != nil {
			if err = s.AudioLib.OpenBank(bank, data); err != nil {
				return nil, err
			}
		}
	}
	if banks := s.AudioLib.BankNames(); len(banks) > 0 {
		v.Log("audio banks: " + strings.Join(banks, ", "))
	} else {
		v.Log("no audio banks for " + base)
	}
	return v, nil
}
func (v *SetViewer) Scene() *df.Scene    { return &v.Set.Scenes[v.SceneIdx] }
func (v *SetViewer) View() *df.SceneView { return &v.Scene().Views[v.ViewIdx] }
func (v *SetViewer) Start() {
	v.Scripts.OpenSet()
	if v.Session.RestoringSave {
		v.Scripts.LastSceneIdx = v.SceneIdx
		return
	}
	v.Scripts.OpenScene(v.SceneIdx)
}
func (v *SetViewer) Animating() bool { return v.animation != nil }
func (v *SetViewer) Busy() bool {
	if v.Director != nil {
		return v.Director.Busy()
	}
	return v.Animating()
}
func (v *SetViewer) RoomFrame() *CachedFrame   { return v.current }
func (v *SetViewer) RoomPalette() []byte       { return v.palette }
func (v *SetViewer) RoomPropPalette() []byte   { return v.propPalette }
func (v *SetViewer) RoomVersion() int          { return v.Set.Version }
func (v *SetViewer) RoomAnimating() bool       { return v.Animating() }
func (v *SetViewer) AvailableRoads() []df.Road { return v.Set.RoadsAt(v.View().ViewID) }
func (v *SetViewer) PlayerPace() float64 {
	switch v.Session.MoveSpeed {
	case "slow":
		return 2 * EngineStepMS
	case "fast":
		return EngineStepMS / 2
	case "instant":
		return 0
	default:
		return EngineStepMS
	}
}
func (v *SetViewer) report(err error) {
	if err != nil {
		v.Log(err.Error())
	}
}
func angularDistance(a, b float64) float64 {
	d := math.Mod(a-b, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return math.Min(d, 2*math.Pi-d)
}
func nearestView(scene *df.Scene, rotation float64) int {
	best, dist := 0, math.Inf(1)
	for i, view := range scene.Views {
		d := angularDistance(view.Rotation, rotation)
		if d < dist {
			best, dist = i, d
		}
	}
	return best
}
func (v *SetViewer) JumpTo(sceneName, viewName string) (bool, error) {
	idx := -1
	for i, sc := range v.Set.Scenes {
		if strings.EqualFold(sc.SceneName, sceneName) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, nil
	}
	scene := &v.Set.Scenes[idx]
	view := -1
	if viewName != "" {
		for i, vw := range scene.Views {
			if strings.EqualFold(vw.ViewName, viewName) {
				view = i
				break
			}
		}
	}
	if view < 0 && viewName != "" {
		rotation := v.View().Rotation
		if v.Session.LastRotation != nil {
			rotation = *v.Session.LastRotation
		}
		v.Session.LastRotation = nil
		view = nearestView(scene, rotation)
	}
	v.SceneIdx = idx
	v.ViewIdx = max(0, view)
	return true, v.ShowView()
}
func (v *SetViewer) ShowView() error {
	v.warmDone = false
	frame, err := v.standFrame()
	if err != nil {
		return err
	}
	v.current = frame
	view := v.View()
	roads := v.AvailableRoads()
	line := v.Set.SetName + " — " + v.Scene().SceneName + " / " + view.ViewName
	if len(roads) > 0 {
		names := []string{}
		for _, r := range roads {
			names = append(names, r.Transition.TransitionName)
		}
		line += "  ·  ↑ " + strings.Join(names, ", ")
	}
	if len(view.Objects) > 0 {
		line += fmt.Sprintf("  ·  %d hotspot(s)", len(view.Objects))
	}
	v.OnHUD(line)
	return nil
}
func (v *SetViewer) updatePalettes() {
	base, props := v.basePalette, v.basePropPalette
	if v.setDim != nil {
		base = DimPalette(base, *v.setDim)
		props = DimPalette(props, *v.setDim)
	}
	v.palette = v.Gamma.DisplayPalette(base)
	v.propPalette = v.Gamma.DisplayPalette(props)
	v.paletteID = newDrawID()
	v.propPaletteID = newDrawID()
}
func (v *SetViewer) ApplyRoomClut(dim *ClutDim) { v.setDim = dim; v.updatePalettes() }
func (v *SetViewer) RefreshRoomGamma() {
	if v.paletteGeneration == v.Gamma.Generation {
		return
	}
	v.paletteGeneration = v.Gamma.Generation
	v.updatePalettes()
}
func (v *SetViewer) BandPropPalette(stage []byte) []byte {
	hit := &v.worldPalette
	same := len(stage) == len(hit.stage) && len(stage) > 0 && &stage[0] == &hit.stage[0]
	if same && hit.dim == v.setDim && hit.generation == v.Gamma.Generation {
		return hit.out
	}
	composed := slices.Clone(v.basePropPalette)
	at := v.Set.ColorCount * 4
	if at >= 0 && at < len(composed) && at < len(stage) {
		copy(composed[at:], stage[at:])
	}
	if v.setDim != nil {
		composed = DimPalette(composed, *v.setDim)
	}
	hit.stage = stage
	hit.dim = v.setDim
	hit.generation = v.Gamma.Generation
	hit.out = v.Gamma.DisplayPalette(composed)
	return hit.out
}
func (v *SetViewer) RoomSignature(sig *DrawSignature) {
	sig.ID(v.paletteID).ID(v.propPaletteID).Bool(v.Animating()).Bool(v.ShowHotspots).Num(float64(v.SceneIdx)).Num(float64(v.ViewIdx))
}
func (v *SetViewer) StartTheme() error {
	if v.Session.CurrentThemeName != "none" {
		return nil
	}
	key := strings.ToLower(v.Set.SetName) + ".trk"
	theme, err := v.Session.AudioLib.Theme(key)
	if err != nil {
		return err
	}
	if theme != nil {
		v.Session.Audio.Play(ThemeChannel, theme, PlayOptions{Loop: true})
		v.Session.CurrentThemeName = key
	}
	return nil
}
func (v *SetViewer) AddResource(name string, data []byte) bool {
	key := strings.ToLower(name)
	for _, ext := range []string{".trk", ".sfx", ".11k"} {
		if strings.HasSuffix(key, ext) {
			if err := v.Session.AudioLib.OpenBank(key, data); err != nil {
				return false
			}
			v.Log("audio bank opened: " + key)
			return true
		}
	}
	if key == strings.ToLower(v.Set.SetName)+".shp" {
		v.Session.Track("openShop "+key, false, func(*Task) error { v.Session.OpenShop(key); return nil })
		return true
	}
	return false
}
func (v *SetViewer) RoomOcclusion() *Occlusion {
	f := v.current
	if f == nil || f.Z == nil {
		return nil
	}
	levels := v.Set.ZLevelCount
	if levels == 0 {
		levels = 24
	}
	scale := float64(v.Set.ZFarMax) / float64(levels)
	if !(scale > 0) {
		return nil
	}
	return &Occlusion{Z: f.Z, W: f.Width, H: f.Height, Scale: scale, Levels: float64(levels), GroundBias: v.Set.SpriteZBias}
}
func (v *SetViewer) cameraFrom(p CameraPose) *WorldCamera {
	w, h := v.Set.ViewPortWidth, v.Set.ViewPortHeight
	if w == 0 {
		w = 512
	}
	if h == 0 {
		h = 264
	}
	th := 2 * math.Pi * float64(int32JS(p.Deg)&255) / 256
	sb := v.Set.CameraSetback
	f := float64(max(w, h)) / 2
	if v.Set.FocalLength != nil {
		f = *v.Set.FocalLength
	}
	return &WorldCamera{X: p.X - jsRound(sb*math.Cos(th)), Y: p.Y - jsRound(sb*math.Sin(th)), Z: p.Z + v.Session.CameraHiBias, Deg: p.Deg, F: f, CX: float64(w) / 2, CY: float64(h) / 2, ClipW: w, ClipH: h}
}
func (v *SetViewer) WorldCamera() *WorldCamera {
	sc, vw := v.Scene(), v.View()
	p := CameraPose{X: float64(sc.XAxisMap), Y: float64(sc.ZAxisMap), Z: jsRound(vw.CameraHeight * 512), Deg: float64(vw.Rotation8)}
	if fi := standFrameInfo(sc, v.ViewIdx); fi != nil {
		p.X = float64(fi.PosX16)
		p.Y = float64(fi.PosZ16)
		p.Z = float64(fi.PosY16)
	}
	return v.cameraFrom(p)
}
func (v *SetViewer) RoomCamera() *WorldCamera {
	if v.Animating() {
		if v.current == nil || v.current.Camera == nil {
			return nil
		}
		return v.cameraFrom(*v.current.Camera)
	}
	return v.WorldCamera()
}
