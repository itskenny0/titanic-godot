package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/save"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

type TextOverlay struct {
	Text              string
	X, Y, Color, Size float64
}
type HitTarget struct{ Name, Type string }
type ClutDim struct{ Lo, Hi, Amt float64 }
type XRayReveal struct {
	Hidden, Base, Mask, Light string
	X, Y                      float64
	Aimed                     bool
}
type Session struct {
	*ScriptDispatch
	Executor                                                                        *Executor
	Read                                                                            func(string) ([]byte, error)
	Audio                                                                           AudioSink
	AudioLib                                                                        *AudioLibrary
	Clock                                                                           *GameClock
	Scheduler                                                                       *Scheduler
	StageCtrl                                                                       *StageController
	PuppetCtrl                                                                      *PuppetController
	Fade                                                                            FadeState
	Wipe                                                                            WipeState
	Events                                                                          EventQueue
	Random, AmbientRandom                                                           func() float64
	SetName, CurrentSetFile, MountedCD, CurrentThemeName                            string
	CurrentBinding                                                                  *SetScripts
	RestoringSave, SetVisible                                                       bool
	HasRealFrames, ModalMovies                                                      bool
	RealYieldSeq                                                                    uint64
	PointerX, PointerY                                                              float64
	PointerDown, ShiftDown                                                          bool
	lastInputPoll                                                                   float64
	LastResult, CursorName                                                          string
	CursorDepth                                                                     int
	CameraHiBias                                                                    float64
	LastRotation                                                                    *float64
	CurrentRotation                                                                 func() float64
	WaveVolume, ThemeVolume                                                         float64
	trackVolume                                                                     map[string]float64
	PuppetParams                                                                    map[int]float64
	TextOverlay                                                                     []TextOverlay
	MeasureText                                                                     func(string, float64) float64
	PictureMode, MoveSpeed                                                          string
	LowMemory, NavHappened, NavGestureActive, NavFromScript                         bool
	OnNavigate, OnSceneJump, OnViewJump, NavDriver, SceneJumpDriver, ViewJumpDriver func(string)
	CurrentSceneName, CurrentViewName                                               func() string
	OnSetChange                                                                     func(string, string, string) error
	Listener                                                                        func() *Listener
	ActiveCamera                                                                    func() *WorldCamera
	HitTestAt                                                                       func(float64, float64) HitTarget
	PointInSet, PointInStage                                                        func(float64, float64) bool
	OnClut                                                                          func(string, *ClutDim)
	RepaintNow                                                                      func()
	CaptureFrame                                                                    func() *RGBAFrame
	OnDiscChange                                                                    func(int)
	OnMoviesDone                                                                    func([]string)
	OnAbandonMovie                                                                  func()
	OnPlayMovie                                                                     func(string, *int) error
	OnNoteDialog                                                                    func(string) error
	OnQuestionDialog                                                                func(string) (bool, error)
	OnTextDialog                                                                    func(string, string) (string, error)
	OnQuit                                                                          func() error
	OnSaveGame                                                                      func([]byte, string) error
	OnLoadGame                                                                      func(string) ([]byte, error)
	MovieActions                                                                    map[int]bool
	XRay                                                                            *XRayReveal
	StarRegistry                                                                    orderedMap[df.Actor]
	instancedActors                                                                 map[string]bool
	inflight, idleInflight                                                          orderedMap[*Task]
	coreLoaded                                                                      bool
	plan                                                                            *BootPlan
}

func NewSession(read func(string) ([]byte, error), audio AudioSink) *Session {
	interp := script.NewInterpreter()
	d := NewScriptDispatch(interp, new(PropRuntime), new(ActorRuntime))
	e := NewExecutor()
	s := &Session{ScriptDispatch: d, Executor: e, Read: read, Audio: audio, AudioLib: NewAudioLibrary(), Clock: NewGameClock(audio), Wipe: NewWipeState(), SetName: "none", CurrentThemeName: "none", SetVisible: true, Random: rand.Float64, AmbientRandom: rand.Float64, lastInputPoll: math.Inf(-1), WaveVolume: 9, ThemeVolume: 255, trackVolume: map[string]float64{}, PuppetParams: map[int]float64{1: 0, 2: 128, 3: 250, 4: 251, 5: 888, 6: 12, 7: 1, 8: 0, 9: 2, 10: 8}, PictureMode: "original", MoveSpeed: "original", MovieActions: map[int]bool{}, instancedActors: map[string]bool{}}
	noop := func(string) {}
	s.OnNavigate, s.OnSceneJump, s.OnViewJump, s.NavDriver, s.SceneJumpDriver, s.ViewJumpDriver = noop, noop, noop, noop, noop, noop
	s.CurrentSceneName = func() string { return "" }
	s.CurrentViewName = func() string { return "" }
	s.OnSetChange = func(string, string, string) error { return nil }
	s.Listener = func() *Listener { return nil }
	s.ActiveCamera = func() *WorldCamera { return nil }
	s.HitTestAt = func(float64, float64) HitTarget { return HitTarget{} }
	s.PointInSet = func(float64, float64) bool { return false }
	s.PointInStage = s.PointInSet
	s.OnClut = func(string, *ClutDim) {}
	s.OnPlayMovie = func(string, *int) error { return nil }
	s.OnNoteDialog = func(message string) error { s.Log("note: " + message); return nil }
	s.OnQuestionDialog = func(string) (bool, error) { return false, nil }
	s.OnTextDialog = func(_ string, initial string) (string, error) { return initial, nil }
	s.OnQuit = func() error { s.Log("quit()"); return nil }
	s.OnSaveGame = func([]byte, string) error { return nil }
	s.OnLoadGame = func(string) ([]byte, error) { return nil, nil }
	s.StageCtrl = NewStageController(d, &s.Fade, read)
	s.StageCtrl.ResetClut = func() { s.OnClut("stage", nil) }
	s.StageCtrl.ResetPlugins = func() { s.XRay = nil }
	s.StageCtrl.ClearText = s.ClearTextOverlay
	s.StageCtrl.SetPointer = s.SetPointer
	s.PuppetCtrl = NewPuppetController(d, e, audio, read)
	s.PuppetCtrl.Random = func() float64 { return s.Random() }
	s.PuppetCtrl.AmbientRandom = func() float64 { return s.AmbientRandom() }
	s.PuppetCtrl.Param = func(slot int) float64 { return s.PuppetParams[slot] }
	s.PuppetCtrl.SetWaveVolume = s.SetWaveVolume
	s.Scheduler = NewScheduler(sessionScheduler{s}, d.Actors, s.AudioLib, audio)
	interp.RealYieldSeq = func() uint64 { return s.RealYieldSeq }
	interp.OnGlobalChange = func(name string, from, to script.Value) {
		s.Log(fmt.Sprintf("glob: %s = %s (was %s)", name, to.String(), from.String()))
	}
	registerSessionBuiltins(s)
	return s
}
func (s *Session) Close() {
	s.Executor.Close()
	s.PuppetCtrl.ClosePuppetFile()
	s.Scheduler.Reset()
	for _, channel := range []AudioChannel{SoundChannel, VoiceChannel, ThemeChannel} {
		s.Audio.Halt(channel)
	}
}
func (s *Session) SeedRandom(seed uint32) {
	s.Random = SeededRandom(seed)
	s.AmbientRandom = SeededRandom(seed ^ 0x9e3779b9)
}
func (s *Session) Track(name string, idle bool, run func(*Task) error) *Task {
	var task *Task
	if current := s.Executor.Current(); current != nil {
		task = current.Fork(name, run)
	} else {
		task = s.Executor.Start(name, run)
	}
	key := strconv.FormatUint(task.ID, 10)
	if idle {
		s.idleInflight.Set(key, task)
	} else {
		s.inflight.Set(key, task)
	}
	return task
}
func (s *Session) ScriptBusy() bool {
	for _, task := range s.inflight.All() {
		if !task.Done() {
			return true
		}
	}
	return false
}
func (s *Session) Pending() []string {
	out := []string{}
	for _, task := range s.inflight.All() {
		if !task.Done() {
			name := task.Name
			if name == "" {
				name = "?"
			}
			out = append(out, name)
		}
	}
	return out
}
func (s *Session) Pump(now float64, frame bool, budget int) []Completion {
	done := s.Executor.Pump(now, frame, budget)
	for _, result := range done {
		key := strconv.FormatUint(result.ID, 10)
		tracked := s.inflight.Has(key) || s.idleInflight.Has(key)
		s.inflight.Delete(key)
		s.idleInflight.Delete(key)
		if tracked && result.Err != nil {
			s.Log("script error: " + result.Err.Error())
		}
	}
	return done
}
func (s *Session) Settle(task *Task, maxRounds int) error {
	for i := 0; i < maxRounds && (s.inflight.Len() > 0 || s.idleInflight.Len() > 0); i++ {
		tasks := []*Task{}
		for _, t := range s.inflight.All() {
			if t != task && !t.Done() {
				tasks = append(tasks, t)
			}
		}
		for _, t := range s.idleInflight.All() {
			if t != task && !t.Done() {
				tasks = append(tasks, t)
			}
		}
		if len(tasks) == 0 {
			return nil
		}
		for _, t := range tasks {
			_ = task.Join(t)
		}
	}
	return nil
}
func (s *Session) NextFrame(task *Task) {
	if s.HasRealFrames {
		task.NextFrame()
	} else {
		task.Wait(func() bool { return true })
	}
}
func (s *Session) TickTime(now float64) error {
	s.Clock.AdvanceFrames(now)
	return s.Scheduler.TickTime(now)
}
func (s *Session) SetCurrentSetName(name string) {
	if s.SetName == name {
		return
	}
	s.SetName = name
	s.Scheduler.SilenceAbsentCrickets()
}
func (s *Session) ViewShowing() bool  { return s.SetVisible && s.SetName != "none" }
func (s *Session) StageOpen() bool    { return s.StageCtrl.Name != "none" }
func (s *Session) SubtitlesOn() bool  { v, ok := s.PuppetParams[7]; return !ok || v != 0 }
func (s *Session) CursorHidden() bool { return s.CursorDepth < 0 }
func (s *Session) SetWaveVolume(n float64) float64 {
	s.WaveVolume = math.Max(0, math.Min(9, jsRound(n)))
	gain := s.WaveVolume / 9
	s.Audio.SetChannelVolume(SoundChannel, gain)
	s.Audio.SetChannelVolume(VoiceChannel, gain)
	return s.WaveVolume
}
func (s *Session) SetThemeVolume(v float64, track string) {
	s.ThemeVolume = math.Max(0, math.Min(255, jsRound(v)))
	if track != "" {
		s.trackVolume[strings.ToLower(track)] = s.ThemeVolume
	}
	s.Audio.SetChannelVolume(ThemeChannel, s.ThemeVolume/255)
}
func (s *Session) VolumeForTrack(track string) (float64, bool) {
	v, ok := s.trackVolume[strings.ToLower(track)]
	return v, ok
}
func (s *Session) ClearTextOverlay() { s.TextOverlay = nil }
func (s *Session) TextWidth(text string, size float64) float64 {
	if s.MeasureText != nil {
		return jsRound(s.MeasureText(text, size))
	}
	n := 0
	for _, r := range text {
		n++
		if r > 0xffff {
			n++
		}
	}
	return math.Ceil(float64(n) * size * .6)
}
func (s *Session) EraseTextUnderProp(p *PropInstance) error {
	r, err := p.ScreenRect()
	if err != nil || r == nil {
		return err
	}
	s.TextOverlay = slices.DeleteFunc(s.TextOverlay, func(e TextOverlay) bool {
		x, y, w, h := e.X, e.Y-e.Size, s.TextWidth(e.Text, e.Size), e.Size+math.Ceil(e.Size/4)
		return x < r.X+float64(r.W) && r.X < x+w && y < r.Y+float64(r.H) && r.Y < y+h
	})
	return nil
}
func (s *Session) SetPointer(x, y float64) { s.PointerX, s.PointerY = x, y }
func (s *Session) PointerPoint() int32     { return PackPoint(s.PointerX, s.PointerY) }
func (s *Session) InputPolled()            { s.lastInputPoll = s.Executor.Now() }
func (s *Session) PollingInput() bool      { return s.Executor.Now()-s.lastInputPoll <= 4*EngineStepMS }
func (s *Session) FireHandler(inst *script.Instance, handler, me, label string) {
	if !hasHandler(inst, handler) {
		return
	}
	if _, err := s.Interp.Run(inst, handler, nil, script.CallContext{Me: me}, nil); err != nil {
		if label == "" {
			label = me + "." + handler
		}
		s.Log(label + ": " + err.Error())
	}
}
func (s *Session) OpenSetFile(file, scene, view string) error {
	key := strings.ToLower(file)
	s.Log(fmt.Sprintf("opensetfile(%q, %q, %q)", key, scene, view))
	s.LastRotation = nil
	if s.CurrentRotation != nil {
		v := s.CurrentRotation()
		s.LastRotation = &v
	}
	s.CurrentSetFile = strings.TrimSuffix(key, ".set")
	return s.OnSetChange(key, strings.ToLower(scene), strings.ToLower(view))
}
func (s *Session) LoadSet(file string) *df.Set {
	data, err := s.Read(strings.ToLower(file))
	if err != nil || data == nil {
		return nil
	}
	set, err := df.ReadSet(data)
	if err != nil {
		s.Log(file + ": " + err.Error())
		return nil
	}
	return set
}
func (s *Session) SnapshotSave() ([]byte, error) {
	m := save.Metadata{Version: 1, Globals: []save.Global{}, State: save.State{Disk: s.MountedCD, Set: s.CurrentSetFile, Scene: s.CurrentSceneName(), View: s.CurrentViewName(), Frame: s.Clock.FrameCounter, Inventory: []save.SavedProp{}, Actors: []save.SavedActor{}, Loops: []save.SavedLoop{}, Crickets: []save.SavedCricket{}, Walks: []save.SavedWalk{}, CastFiles: append([]string{}, s.Actors.Casts.keys...), TrackFiles: s.AudioLib.BankNames()}}
	for _, name := range s.Interp.Globals.Keys() {
		if strings.HasPrefix(name, "__") {
			continue
		}
		v, _ := s.Interp.Globals.Get(name)
		var value any = v.Number
		if v.IsString {
			value = v.Text
		}
		m.Globals = append(m.Globals, save.Global{Name: name, Value: value})
	}
	for name, p := range s.Props.Props.All() {
		owner := p.Owner.String()
		if owner == "" {
			owner = "none"
		}
		m.Inventory = append(m.Inventory, save.SavedProp{Name: name, Owner: owner, View: p.StateName, Visible: p.Visible, Is3d: p.WorldSpace, X: p.AnchorX, Y: p.AnchorY, Deg: propertyNumber(p.Deg), Dist: p.Dist, Scale: p.Scale, Value: propertyNumber(p.Value), Zclip: p.Zclip})
	}
	for name, a := range s.Actors.Actors.All() {
		owner := a.Owner.String()
		if owner == "" {
			owner = "none"
		}
		m.Actors = append(m.Actors, save.SavedActor{Name: name, Owner: owner, Value: propertyNumber(a.Value), Placement: save.Placement{Visible: a.Visible, Set: a.SetName, Star: a.StarName, Pose: a.PoseName, X: a.WorldX, Y: a.WorldY, Z: a.WorldZ, Deg: a.Deg, Speed: a.Speed, Turn: a.Turn, Scale: a.Scale, Zclip: a.Zclip}})
	}
	for _, l := range s.Scheduler.Loops {
		m.Loops = append(m.Loops, save.SavedLoop{Kind: l.Kind, Name: l.Name, Handler: l.Handler, Period: l.Count})
	}
	for _, c := range s.Scheduler.Crickets {
		m.Crickets = append(m.Crickets, save.SavedCricket{Name: c.Name, Set: c.SetName, X: c.X, Y: c.Y, Radius: c.Radius, Base: c.Base, Jitter: c.Jitter, Next: c.Count})
	}
	for name, w := range s.Scheduler.Walks.All() {
		a := s.Actors.Get(name)
		kind := 1.
		if w.TurnOnly {
			kind = 0
		} else if len(w.Path) > 1 {
			kind = 3
		}
		turn, deg, star := -1., 0., ""
		if w.TurnTo != nil {
			turn = *w.TurnTo
		}
		if a != nil {
			deg = a.Deg
			if w.TurnOnly {
				star = a.StarName
			}
		}
		if w.ArriveStar != nil {
			star = *w.ArriveStar
		}
		walk := save.SavedWalk{Actor: name, Type: kind, HasPayload: kind == 3, Paused: w.Paused, TurnTo: turn, Deg: deg, StartX: w.SX, StartY: w.SY, StartZ: w.SZ, DestX: w.SX + w.DX, DestY: w.SY + w.DY, DestZ: w.SZ + w.DZ, Progress: w.Progress, Dist: w.Dist, Star: star}
		if w.Path != nil {
			walk.Path = []save.Waypoint{}
			for _, p := range w.Path {
				walk.Path = append(walk.Path, save.Waypoint{X: p.X, Y: p.Y, Z: p.Z, Cum: p.Cum})
			}
		}
		m.Walks = append(m.Walks, walk)
	}
	if s.CurrentThemeName != "none" {
		volume := 255.
		if v, ok := s.Interp.Globals.Get("themevolume"); ok {
			volume = propertyNumber(v)
		}
		m.Theme = &save.SavedTheme{Track: s.CurrentThemeName, Volume: volume}
	}
	return save.AppendMetadata(save.NeutralTemplate(), m)
}
