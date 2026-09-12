package engine

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

const EngineStepMS = 50
const RampStepMS = EngineStepMS / 3.0

func TicksAt(ms float64) float64 { return math.Floor(ms * 3 / 50) }

type Listener struct{ X, Y, Deg float64 }

// SchedulerHost supplies session state and serializes callbacks through Executor.
// Track must register work before returning so busy-script checks see it.
type SchedulerHost interface {
	CurrentSet() string
	CurrentFlatName() string
	ScriptBusy() bool
	AmbientRandom() float64
	ListenerPosition() *Listener
	HasHandler(kind, name, handler string) bool
	HasGlobal(name string) bool
	RunGlobal(*Task, string) error
	SendEvent(task *Task, command, target, handler, caller string) error
	Track(name string, idle bool, run func(*Task) error)
	WithNavigation(run func() error) error
	AdvanceClock(now float64)
	Log(string)
}
type GameLoop struct {
	Kind, Name, Handler string
	Count, Period       float64
	Paused              bool
}
type Cricket struct {
	Name, SetName                     string
	X, Y, Radius, Base, Jitter, Count float64
	Paused                            bool
	Handle                            PlayHandle
}
type soundSlot struct {
	Name   string
	Handle PlayHandle
}
type Scheduler struct {
	Host                      SchedulerHost
	Actors                    *ActorRuntime
	AudioLib                  *AudioLibrary
	Audio                     AudioSink
	Loops                     []*GameLoop
	Crickets                  []*Cricket
	Walks                     orderedMap[*Walk]
	LoopFlags                 orderedMap[bool]
	soundLoops                orderedMap[PlayHandle]
	soundVol, soundPan        map[string]float64
	soundChannels             [2]*soundSlot
	timeLastTick, clockLastMS float64
	clockDispatching          bool
}

func NewScheduler(host SchedulerHost, actors *ActorRuntime, lib *AudioLibrary, audio AudioSink) *Scheduler {
	return &Scheduler{Host: host, Actors: actors, AudioLib: lib, Audio: audio, soundVol: map[string]float64{}, soundPan: map[string]float64{}}
}
func (s *Scheduler) SetSoundVol(name string, v float64) { s.soundVol[strings.ToLower(name)] = v }
func (s *Scheduler) GetSoundVol(name string) float64 {
	v, ok := s.soundVol[strings.ToLower(name)]
	if !ok {
		return 255
	}
	return v
}
func (s *Scheduler) SetSoundPan(name string, v float64) { s.soundPan[strings.ToLower(name)] = v }
func (s *Scheduler) GetSoundPan(name string) float64 {
	v, ok := s.soundPan[strings.ToLower(name)]
	if !ok {
		return 128
	}
	return v
}
func (s *Scheduler) CurrentSound(channel int) string {
	if channel < 1 || channel > 2 {
		return ""
	}
	slot := s.soundChannels[channel-1]
	if slot != nil && !slot.Handle.Done() {
		return slot.Name
	}
	return ""
}
func (s *Scheduler) rand(n float64) float64 {
	if n > 0 {
		return math.Floor(s.Host.AmbientRandom()*n) + 1
	}
	return 0
}

var emptyCall = regexp.MustCompile(`^(.*?)\s*\(\s*\)$`)

func loopHandlerName(handler string) string {
	m := emptyCall.FindStringSubmatch(strings.TrimSpace(handler))
	if m != nil {
		return m[1]
	}
	return handler
}
func (s *Scheduler) MakeLoop(kind, name, handler string, period float64) {
	s.StopLoop(kind, name)
	s.addLoop(kind, name, handler, period, "makeloop")
}
func (s *Scheduler) RestoreLoop(kind, name, handler string, remaining float64) {
	s.addLoop(kind, name, handler, remaining, "loadgame")
}
func (s *Scheduler) addLoop(kind, name, handler string, period float64, operation string) {
	if len(s.Loops) >= 32 {
		s.Host.Log(fmt.Sprintf("%s: loop table full (32), dropping %s/%s", operation, kind, name))
		return
	}
	s.Loops = append(s.Loops, &GameLoop{Kind: strings.ToLower(kind), Name: strings.ToLower(name), Handler: loopHandlerName(handler), Count: math.Max(1, period), Period: math.Max(1, period)})
}
func (s *Scheduler) StopLoop(kind, name string) {
	kind, name = strings.ToLower(kind), strings.ToLower(name)
	s.Loops = slices.DeleteFunc(s.Loops, func(l *GameLoop) bool { return l.Kind == kind && (name == "all" || l.Name == name) })
}
func (s *Scheduler) PauseLoop(kind, name string, on bool) {
	kind, name = strings.ToLower(kind), strings.ToLower(name)
	for _, l := range s.Loops {
		if l.Kind == kind && (name == "all" || l.Name == name) {
			l.Paused = on
		}
	}
}
func (s *Scheduler) IsLoop(kind, name string) bool {
	kind, name = strings.ToLower(kind), strings.ToLower(name)
	return slices.ContainsFunc(s.Loops, func(l *GameLoop) bool { return l.Kind == kind && l.Name == name })
}
func (s *Scheduler) MakeCricket(name string, x, y, radius, base, jitter float64) {
	s.StopCricket(name)
	if len(s.Crickets) >= 16 {
		s.Host.Log("makecricket: table full (16), dropping " + name)
		return
	}
	count := base
	if jitter >= 0 {
		count += s.rand(jitter)
	}
	s.Crickets = append(s.Crickets, &Cricket{Name: strings.ToLower(name), SetName: s.Host.CurrentSet(), X: x, Y: y, Radius: math.Max(1, radius), Base: base, Jitter: jitter, Count: count})
}
func (s *Scheduler) RestoreCricket(name, set string, x, y, radius, base, jitter, next float64) {
	if len(s.Crickets) >= 16 {
		s.Host.Log("loadgame: cricket table full (16), dropping " + name)
		return
	}
	s.Crickets = append(s.Crickets, &Cricket{Name: strings.ToLower(name), SetName: strings.ToLower(set), X: x, Y: y, Radius: math.Max(1, radius), Base: base, Jitter: jitter, Count: next})
}
func (s *Scheduler) StopCricket(name string) {
	name = strings.ToLower(name)
	s.Crickets = slices.DeleteFunc(s.Crickets, func(c *Cricket) bool {
		if name == "all" || c.Name == name {
			if c.Handle != nil {
				c.Handle.Stop()
			}
			return true
		}
		return false
	})
}
func (s *Scheduler) PauseCricket(name string, on bool) {
	name = strings.ToLower(name)
	for _, c := range s.Crickets {
		if name == "all" || c.Name == name {
			c.Paused = on
		}
	}
}
func (s *Scheduler) IsCricket(name string) bool {
	name = strings.ToLower(name)
	return slices.ContainsFunc(s.Crickets, func(c *Cricket) bool { return c.Name == name })
}
func (s *Scheduler) SoundLoop(name string, on bool) {
	key := strings.ToLower(name)
	if on {
		s.LoopFlags.Set(key, true)
		return
	}
	s.LoopFlags.Delete(key)
	if h := s.soundLoops.Get(key); h != nil {
		h.Stop()
	}
	s.soundLoops.Delete(key)
}
func (s *Scheduler) PlaySound(name string, overlap bool) error {
	key := strings.ToLower(name)
	audio, err := s.AudioLib.Sound(key)
	if err != nil {
		return err
	}
	if audio == nil {
		banks := strings.Join(s.AudioLib.BankNames(), ", ")
		if banks == "" {
			banks = "none"
		}
		s.Host.Log(fmt.Sprintf("sound not found: %s (banks: %s)", name, banks))
		return nil
	}
	volume := math.Max(0, math.Min(1, s.GetSoundVol(key)/255))
	pan := math.Max(-1, math.Min(1, (s.GetSoundPan(key)-128)/128))
	if s.LoopFlags.Has(key) {
		if h := s.soundLoops.Get(key); h != nil && !h.Done() {
			return nil
		}
		h := s.Audio.Play(SoundChannel, audio, PlayOptions{Loop: true, Overlap: true, Volume: &volume, Pan: pan})
		s.soundLoops.Set(key, h)
		s.soundChannels[1] = &soundSlot{key, h}
		return nil
	}
	h := s.Audio.Play(SoundChannel, audio, PlayOptions{Overlap: overlap, Volume: &volume, Pan: pan})
	channel := 0
	if overlap {
		channel = 1
	}
	s.soundChannels[channel] = &soundSlot{key, h}
	return nil
}
func (s *Scheduler) HaltSounds() {
	s.Audio.Halt(SoundChannel)
	for _, h := range s.soundLoops.All() {
		h.Stop()
	}
	s.soundLoops = orderedMap[PlayHandle]{}
}
func (s *Scheduler) Reset() {
	s.Loops = nil
	for _, c := range s.Crickets {
		if c.Handle != nil {
			c.Handle.Stop()
		}
	}
	s.Crickets = nil
	s.Walks = orderedMap[*Walk]{}
	s.LoopFlags = orderedMap[bool]{}
	s.HaltSounds()
}
func (s *Scheduler) TickTime(now float64) error {
	s.Host.AdvanceClock(now)
	if s.timeLastTick == 0 {
		s.timeLastTick = now
	}
	steps := math.Floor((now - s.timeLastTick) / EngineStepMS)
	if steps > 0 {
		s.timeLastTick += steps * EngineStepMS
		steps = math.Min(steps, 64)
		for i := 0; i < int(steps); i++ {
			if err := s.serviceStep(); err != nil {
				return err
			}
		}
	}
	s.serviceGameClock(now)
	return nil
}
func (s *Scheduler) serviceGameClock(now float64) {
	if s.Host.ScriptBusy() || s.clockDispatching || s.clockLastMS == 0 {
		s.clockLastMS = now
		return
	}
	if !s.Host.HasGlobal("calctime") {
		s.clockLastMS = now
		return
	}
	calls := math.Floor((now - s.clockLastMS) / 50)
	if calls <= 0 {
		return
	}
	s.clockLastMS += calls * 50
	calls = math.Min(calls, 20)
	s.clockDispatching = true
	s.Host.Track("game clock", true, func(task *Task) error {
		defer func() { s.clockDispatching = false }()
		for i := 0; i < int(calls); i++ {
			if err := s.Host.RunGlobal(task, "calctime"); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Scheduler) serviceStep() error {
	s.serviceWalks()
	s.SilenceAbsentCrickets()
	for i := len(s.Crickets) - 1; i >= 0; i-- {
		c := s.Crickets[i]
		if c.Paused {
			continue
		}
		if c.Count > 0 {
			c.Count--
		}
		if c.Count > 0 || c.SetName != "" && c.SetName != s.Host.CurrentSet() || c.Handle != nil && !c.Handle.Done() {
			continue
		}
		if err := s.fireCricket(c); err != nil {
			return err
		}
		if c.Jitter < 0 {
			s.Crickets = slices.Delete(s.Crickets, i, i+1)
		} else {
			c.Count = c.Base + s.rand(c.Jitter)
		}
	}
	s.fireDueLoops(func(l *GameLoop) bool { return l.Period > 1 })
	s.Actors.AdvanceAnimation()
	return nil
}
func (s *Scheduler) ServiceFrameLoops() {
	s.fireDueLoops(func(l *GameLoop) bool { return l.Period <= 1 })
}
func (s *Scheduler) PumpFrameLoops(exceptName string) {
	ex := strings.ToLower(exceptName)
	due := []*GameLoop{}
	for _, l := range s.Loops {
		if !l.Paused && l.Period <= 1 && l.Name != ex {
			l.Count--
			if l.Count <= 0 {
				due = append(due, l)
			}
		}
	}
	s.fireNow(due)
	due = nil
	for _, l := range s.Loops {
		if !l.Paused && l.Period > 1 && l.Count <= 0 && l.Name != ex {
			due = append(due, l)
		}
	}
	s.fireNow(due)
}
func (s *Scheduler) fireDueLoops(selectLoop func(*GameLoop) bool) {
	due := []*GameLoop{}
	for _, l := range s.Loops {
		if !l.Paused && selectLoop(l) {
			if l.Count > 0 {
				l.Count--
			}
			if l.Count <= 0 {
				due = append(due, l)
			}
		}
	}
	if !s.Host.ScriptBusy() {
		s.fireNow(due)
	}
}
func (s *Scheduler) fireNow(due []*GameLoop) {
	if len(due) == 0 {
		return
	}
	s.Host.Track("script loops", false, func(task *Task) error {
		for _, l := range due {
			i := slices.Index(s.Loops, l)
			if i < 0 {
				continue
			}
			s.Loops = slices.Delete(s.Loops, i, i+1)
			s.fireLoop(task, l)
		}
		return nil
	})
}
func (s *Scheduler) fireLoop(task *Task, l *GameLoop) {
	command := map[string]string{"actor": "sendtoactor", "prop": "sendtoprop", "scene": "sendtoscene", "flat": "sendtoflat"}[l.Kind]
	if command == "" {
		command = "sendtoprop"
	}
	target := l.Name
	if l.Kind == "flat" && !s.Host.HasHandler("flat", l.Name, l.Handler) {
		target = s.Host.CurrentFlatName()
	}
	dispatch := func() error {
		if err := s.Host.SendEvent(task, command, target, l.Handler, target); err != nil {
			s.Host.Log(fmt.Sprintf("loop %s/%s.%s: %v", l.Kind, l.Name, l.Handler, err))
		}
		return nil
	}
	if l.Kind == "scene" {
		_ = s.Host.WithNavigation(dispatch)
	} else {
		_ = dispatch()
	}
}
func (s *Scheduler) SilenceAbsentCrickets() {
	for _, c := range s.Crickets {
		if c.Handle == nil || c.SetName == "" || c.SetName == s.Host.CurrentSet() {
			continue
		}
		c.Handle.Stop()
		c.Handle = nil
	}
}
func (s *Scheduler) fireCricket(c *Cricket) error {
	audio, err := s.AudioLib.Sound(c.Name)
	if err != nil {
		return err
	}
	if audio == nil {
		return nil
	}
	volume, pan := 1., 0.
	if lis := s.Host.ListenerPosition(); lis != nil {
		dx, dy := c.X-lis.X, c.Y-lis.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist >= c.Radius {
			return nil
		}
		volume = 1 - dist/c.Radius
		if dist > 1 {
			th := float64(int32JS(lis.Deg)&255) / 256 * 2 * math.Pi
			lateral := dy*math.Cos(th) - dx*math.Sin(th)
			pan = math.Max(-1, math.Min(1, lateral/dist))
		}
	}
	c.Handle = s.Audio.Play(SoundChannel, audio, PlayOptions{Overlap: true, Volume: &volume, Pan: pan, Loop: s.LoopFlags.Has(c.Name)})
	s.soundChannels[1] = &soundSlot{c.Name, c.Handle}
	return nil
}
