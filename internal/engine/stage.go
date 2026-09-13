package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type FlatImage struct {
	Pixels, Palette []byte
	Width, Height   int
}
type StageController struct {
	Dispatch                           *ScriptDispatch
	Fade                               *FadeState
	Read                               func(string) ([]byte, error)
	ResetClut, ResetPlugins, ClearText func()
	SetPointer                         func(float64, float64)
	File                               *df.Stage
	Name, CurrentFlat                  string
	FlatNames                          []string
	images                             map[string]*FlatImage
	regions                            map[string][]df.StageRegion
	outgoing                           *FlatImage
}

func NewStageController(d *ScriptDispatch, fade *FadeState, read func(string) ([]byte, error)) *StageController {
	return &StageController{Dispatch: d, Fade: fade, Read: read, Name: "none", CurrentFlat: "none", images: map[string]*FlatImage{}, regions: map[string][]df.StageRegion{}, ResetClut: func() {}, ResetPlugins: func() {}, ClearText: func() {}, SetPointer: func(float64, float64) {}}
}
func (s *StageController) fire(inst *script.Instance, handler, me string) {
	if !hasHandler(inst, handler) {
		return
	}
	if _, err := s.Dispatch.Interp.Run(inst, handler, nil, script.CallContext{Me: me}, nil); err != nil {
		s.Dispatch.Log(fmt.Sprintf("%s.%s: %v", me, handler, err))
	}
}
func (s *StageController) OpenStageFile(name string) bool {
	key := strings.ToLower(name)
	if s.Name == key {
		return true
	}
	if s.File != nil {
		s.CloseStageFile()
	}
	s.ResetClut()
	data, err := s.Read(key)
	if err != nil || data == nil {
		s.Dispatch.Log(fmt.Sprintf("openstagefile: %q not available", name))
		return false
	}
	stg, err := df.ReadStage(data)
	if err != nil {
		s.Dispatch.Log(fmt.Sprintf("openstagefile: %s: %v", name, err))
		return false
	}
	s.File, s.Name = stg, key
	if s.Fade.Blanked && s.Fade.Level == 1 {
		s.Fade.PendingReveal = true
	} else {
		s.Fade.Queue = nil
		s.Fade.Snapshot = nil
		s.Fade.Level = 0
	}
	d := s.Dispatch
	d.StageScript = d.InstanceFrom(stg.File.Data(stg.MainScriptLocation), key)
	d.RefreshFallbacks()
	for _, flat := range stg.Flats {
		inst := d.InstanceFrom(stg.File.Data(flat.LocationScript), flat.Name)
		if inst != nil {
			d.FlatScripts.Set(strings.ToLower(flat.Name), inst)
		}
		s.FlatNames = append(s.FlatNames, flat.Name)
	}
	d.Log(fmt.Sprintf("stage loaded: %s (%d flat(s))", key, len(stg.Flats)))
	s.CurrentFlat = "none"
	s.fire(d.StageScript, "openstage", key)
	if s.CurrentFlat == "none" && len(stg.Flats) > 0 {
		s.GotoFlat(stg.Flats[0].Name)
	}
	return true
}
func (s *StageController) CloseStageFile() {
	d := s.Dispatch
	s.fire(d.StageScript, "closestage", s.Name)
	s.fireFlat(s.CurrentFlat, "closeflat")
	s.CurrentFlat = "none"
	s.File = nil
	s.Name = "none"
	d.StageScript = nil
	d.FlatScripts = orderedMap[*script.Instance]{}
	s.FlatNames = nil
	clear(s.images)
	clear(s.regions)
	s.ResetPlugins()
	d.RefreshFallbacks()
}
func (s *StageController) ResetOverlayStack() {
	for _, n := range []string{"1", "2", "3"} {
		s.Dispatch.Interp.Globals.Set("savestage"+n, script.Str(""))
		s.Dispatch.Interp.Globals.Set("saveflat"+n, script.Str(""))
	}
}
func (s *StageController) ResolveFlat(ref string) string {
	for _, name := range s.FlatNames {
		if strings.EqualFold(name, ref) {
			return name
		}
	}
	index := propertyNumber(script.Str(ref))
	if index == math.Trunc(index) && index >= 1 && index <= float64(len(s.FlatNames)) {
		return s.FlatNames[int(index)-1]
	}
	return ref
}
func (s *StageController) FlatToIndex(ref string) int {
	name := s.ResolveFlat(ref)
	for i, f := range s.FlatNames {
		if strings.EqualFold(f, name) {
			return i + 1
		}
	}
	return 0
}
func (s *StageController) GotoFlat(name string) {
	target := s.ResolveFlat(name)
	s.outgoing = s.FlatImage(s.CurrentFlat)
	s.fireFlat(s.CurrentFlat, "closeflat")
	s.CurrentFlat = target
	s.ClearText()
	s.fireFlat(target, "openflat")
}
func (s *StageController) StageRefName() string {
	if s.File == nil {
		return ""
	}
	return s.File.RefName
}
func (s *StageController) regionsFor(name string) []df.StageRegion {
	if s.File == nil || name == "none" {
		return nil
	}
	key := s.Name + ":" + name
	if regs, ok := s.regions[key]; ok {
		return regs
	}
	regs := []df.StageRegion{}
	for _, f := range s.File.Flats {
		if strings.EqualFold(f.Name, name) {
			regs = df.ReadStageRegions(s.File.File.Data(f.LocationClickLogic), s.File.Version)
			break
		}
	}
	s.regions[key] = regs
	return regs
}
func (s *StageController) CurrentFlatRegions() []df.StageRegion { return s.regionsFor(s.CurrentFlat) }
func (s *StageController) FlatButtonNames(name string) []string {
	out := []string{}
	for _, r := range s.regionsFor(name) {
		out = append(out, r.Name)
	}
	return out
}
func (s *StageController) FlatRegion(flat, name string) *df.StageRegion {
	for _, r := range s.regionsFor(flat) {
		if strings.EqualFold(r.Name, name) {
			return &r
		}
	}
	return nil
}
func (s *StageController) SendToButton(flat, regionName, handler string, args []script.Value, caller string, parent *script.Frame) (script.Value, error) {
	if s.File == nil {
		return script.Num(0), nil
	}
	region := s.FlatRegion(flat, regionName)
	if region == nil {
		s.Dispatch.Log(fmt.Sprintf("sendtobutton: no region %q in flat %s", regionName, flat))
		return script.Num(0), nil
	}
	name := region.Name
	if name == "" {
		name = "region"
	}
	d := s.Dispatch
	inst := d.InstanceFrom(s.File.File.Data(region.Script), name)
	current := d.FlatScripts.Get(strings.ToLower(s.CurrentFlat))
	ctx := script.CallContext{Me: region.Name, Target: region.Name}
	if hasHandler(inst, handler) {
		inst.Parent = current
		if current == nil {
			inst.Parent = d.StageScript
		}
		res, err := d.Interp.Run(inst, handler, args, ctx, parent)
		return res.Value, err
	}
	for _, lib := range append([]*script.Instance{current, d.StageScript}, d.BootScripts...) {
		if hasHandler(lib, handler) {
			res, err := d.Interp.Run(lib, handler, args, ctx, parent)
			return res.Value, err
		}
	}
	return script.Num(0), nil
}
func (s *StageController) ClickAt(x, y int) (bool, error) {
	return s.ClickAtPoint(float64(x), float64(y))
}
func (s *StageController) ClickAtPoint(x, y float64) (bool, error) {
	if s.File == nil {
		return false, nil
	}
	var hit *df.StageRegion
	for _, r := range s.CurrentFlatRegions() {
		if x >= float64(r.Left) && x <= float64(r.Right) && y >= float64(r.Top) && y <= float64(r.Bottom) {
			hit = &r
			break
		}
	}
	if hit == nil {
		return false, nil
	}
	d := s.Dispatch
	name := hit.Name
	if name == "" {
		name = "region"
	}
	inst := d.InstanceFrom(s.File.File.Data(hit.Script), name)
	flat := d.FlatScripts.Get(strings.ToLower(s.CurrentFlat))
	sPointer := func() { s.SetPointer(float64(x), float64(y)) }
	if inst == nil {
		sPointer()
		for _, link := range []*script.Instance{flat, d.StageScript} {
			if hasHandler(link, "mousedown") {
				if _, err := d.Interp.Run(link, "mousedown", []script.Value{script.Str(hit.Name)}, script.CallContext{Me: link.Name, Target: hit.Name}, nil); err != nil {
					d.Log(fmt.Sprintf("stage hotspot %s: %v", hit.Name, err))
				}
			}
		}
		return false, nil
	}
	over, err := d.Props.PropAtPoint(x, y, nil, false, nil)
	if err != nil {
		return false, err
	}
	if over != nil && hasHandler(d.PropScripts.Get(strings.ToLower(over.Group.Name)), "mousedown") {
		return false, nil
	}
	inst.Parent = flat
	if flat == nil {
		inst.Parent = d.StageScript
	}
	sPointer()
	chain := []*script.Instance{}
	for _, link := range []*script.Instance{inst, flat, d.StageScript} {
		if link != nil && !slices.Contains(chain, link) {
			chain = append(chain, link)
		}
	}
	if _, err := d.RunChain(chain, "mousedown", []script.Value{script.Str(hit.Name)}, func(link *script.Instance) script.CallContext {
		return script.CallContext{Me: link.Name, Target: hit.Name}
	}, true, nil); err != nil {
		d.Log(fmt.Sprintf("stage region %s: %v", hit.Name, err))
	}
	return true, nil
}
func (s *StageController) KeydownTarget() *script.Instance {
	flat := s.Dispatch.FlatScripts.Get(strings.ToLower(s.CurrentFlat))
	if hasHandler(flat, "keydown") {
		return flat
	}
	if hasHandler(s.Dispatch.StageScript, "keydown") {
		return s.Dispatch.StageScript
	}
	return nil
}
func (s *StageController) fireFlat(name, handler string) {
	if name == "" || name == "none" {
		return
	}
	if _, err := s.Dispatch.SendEvent("sendtoflat", name, handler, nil, name, nil); err != nil {
		s.Dispatch.Log(fmt.Sprintf("flat %s.%s: %v", name, handler, err))
	}
}

var deltaFlat = regexp.MustCompile(`^(.+\..+)\.(\d+)$`)

func (s *StageController) deltaBase(name string) *FlatImage {
	m := deltaFlat.FindStringSubmatch(name)
	if m == nil || s.File == nil {
		return nil
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return nil
	}
	previous := m[1]
	if n > 1 {
		previous += "." + strconv.Itoa(n-1)
	}
	for _, f := range s.File.Flats {
		if f.Name == previous {
			return s.FlatImage(previous)
		}
	}
	return nil
}
func (s *StageController) FlatImage(name string) *FlatImage {
	if s.File == nil || name == "none" {
		return nil
	}
	target := name
	if name != s.CurrentFlat {
		target = s.ResolveFlat(name)
	}
	key := s.Name + ":" + target
	if img := s.images[key]; img != nil {
		return img
	}
	var flat *df.StageFlat
	for _, f := range s.File.Flats {
		if f.Name == target {
			flat = &f
			break
		}
	}
	if flat == nil {
		return nil
	}
	fb := new(df.FrameBuffer)
	under := s.deltaBase(target)
	if under == nil {
		under = s.outgoing
	}
	if under != nil {
		if err := fb.Ensure(under.Width, under.Height); err != nil {
			return nil
		}
		copy(fb.Pixels, under.Pixels)
	}
	frame, err := df.DecodeFrame(s.File.File.Data(flat.LocationFrame), fb, nil)
	if err != nil {
		s.Dispatch.Log(fmt.Sprintf("flat image %s: %v", key, err))
		return nil
	}
	img := &FlatImage{Pixels: slices.Clone(fb.Pixels[:frame.Width*frame.Height]), Width: frame.Width, Height: frame.Height, Palette: df.PaletteRGBA(s.File.PaletteRaw, 256, nil)}
	s.images[key] = img
	return img
}
