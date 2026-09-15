package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
)

type MovieDisplayFrame struct {
	*MovieImage
	Palette          []byte
	OriginX, OriginY int
}
type movieReturn struct {
	file  string
	frame int
}
type activeMovie struct {
	name      string
	mov       *df.Movie
	seg       *df.MovieSegment
	frames    *MovieFrames
	byName    map[string]int
	soundJump *struct {
		frame int
		sound PlayHandle
	}
	hasRegions                          bool
	palette                             []byte
	paletteGeneration                   uint64
	pos, action1, action2, segmentIndex int
	interval, lastTick, segmentStart    float64
	cuesFired                           map[int]bool
}
type movieSequence struct{ done, cutscene bool }
type MoviePlayer struct {
	Session            *Session
	Gamma              *ScreenGamma
	Log                func(string)
	OnFinished         func()
	OnStarted          func(string)
	OnCutsceneFinished func()
	EscapeSkipsSegment bool
	active             *activeMovie
	callStack          []movieReturn
	eventSounds        []PlayHandle
	sequence           *movieSequence
	played             orderedMap[bool]
	clickSound         string
}

func NewMoviePlayer(s *Session, gamma *ScreenGamma, onFinished func()) *MoviePlayer {
	if gamma == nil {
		gamma = NewScreenGamma()
	}
	if onFinished == nil {
		onFinished = func() {}
	}
	return &MoviePlayer{Session: s, Gamma: gamma, OnFinished: onFinished, Log: func(string) {}}
}
func (p *MoviePlayer) Playing() bool { return p.active != nil }
func (p *MoviePlayer) PlayingFile() string {
	if p.active == nil {
		return ""
	}
	return p.active.name
}
func (p *MoviePlayer) FramePos() int {
	if p.active == nil {
		return -1
	}
	return min(p.active.pos, p.active.frames.Len()-1)
}
func (p *MoviePlayer) WaitingRegions() []df.MovieRegion {
	m := p.active
	if m == nil || !m.hasRegions {
		return nil
	}
	return m.seg.Frames[min(m.pos, len(m.seg.Frames)-1)].Regions
}
func (p *MoviePlayer) Frame() (*MovieDisplayFrame, error) {
	m := p.active
	if m == nil {
		return nil, nil
	}
	f, err := m.frames.Get(min(m.pos, m.frames.Len()-1))
	if err != nil {
		return nil, err
	}
	if m.paletteGeneration != p.Gamma.Generation {
		m.palette = p.Gamma.DisplayPalette(df.PaletteRGBA(m.seg.PaletteRaw, 256, m.seg.File.Order))
		m.paletteGeneration = p.Gamma.Generation
	}
	return &MovieDisplayFrame{MovieImage: f, Palette: m.palette, OriginX: m.seg.OriginX, OriginY: m.seg.OriginY}, nil
}

// Play keeps a script parked for the entire chain, including nested movie calls.
// A nil task starts playback without waiting, for the host's initial menu.
func (p *MoviePlayer) Play(task *Task, file string, start int) error {
	chained := p.sequence != nil
	if !chained {
		clear(p.Session.MovieActions)
	}
	ok, err := p.load(file, start)
	if err != nil || !ok {
		if p.sequence != nil {
			p.sequence.cutscene = false
		}
		p.finish(false)
		return err
	}
	if chained {
		return nil
	}
	seq := &movieSequence{cutscene: movieIsCutscene(p.active.mov)}
	p.sequence = seq
	if task != nil {
		task.Wait(func() bool { return seq.done })
	}
	return nil
}

func movieIsCutscene(mov *df.Movie) bool {
	frames := 0
	for i := range mov.Segments {
		if df.MovieHasRegions(&mov.Segments[i]) {
			return false
		}
		frames += len(mov.Segments[i].Frames)
	}
	return frames > 1
}

func (p *MoviePlayer) load(name string, start int) (bool, error) {
	key := strings.ToLower(name)
	p.played.Set(key, true)
	data, err := p.Session.Read(key)
	if err != nil || data == nil {
		p.Log(fmt.Sprintf("playmovie: %q not available", name))
		return false, nil
	}
	mov, err := df.ReadMovie(data)
	if err != nil {
		p.Log(fmt.Sprintf("playmovie: %s: %v", name, err))
		return false, nil
	}
	if len(mov.Segments) == 0 || len(mov.Segments[0].Frames) == 0 {
		return false, nil
	}
	if p.OnStarted != nil {
		p.OnStarted(key)
	}
	if p.sequence != nil && movieIsCutscene(mov) {
		p.sequence.cutscene = true
	}
	return p.enterSegment(mov, key, 0, start)
}
func (p *MoviePlayer) enterSegment(mov *df.Movie, name string, index, start int) (bool, error) {
	seg := &mov.Segments[index]
	if len(seg.Frames) == 0 {
		return false, nil
	}
	frames := NewMovieFrames(seg)
	// Decode only the first requested frame. Later deltas stream during Tick.
	start = max(0, min(start, frames.Len()-1))
	if _, err := frames.Get(start); err != nil {
		return false, err
	}
	audio, err := df.DecodeMovieSegmentAudio(seg)
	if err != nil {
		return false, err
	}
	seconds := 0.0
	if audio != nil {
		seconds = audio.AudioSec
	}
	interval := df.MovieSegmentInterval(seg, frames.Len(), seconds, index)
	if audio != nil {
		p.Session.Audio.Halt(VoiceChannel)
		bed := df.MovieSoundtrackFor(seg, audio, interval, frames.Len(), df.MovieBedRuntimeMS(mov, index))
		p.Session.Audio.Play(VoiceChannel, &bed.Audio, PlayOptions{Loop: bed.Loop})
	}
	names := map[string]int{}
	for i, f := range seg.Frames {
		if f.Name != "" {
			names[strings.ToLower(f.Name)] = i
		}
	}
	action := func(name string) int {
		if name != "" {
			if i, ok := names[strings.ToLower(name)]; ok {
				return i
			}
		}
		return -1
	}
	p.Session.Events.Flush()
	p.active = &activeMovie{name: name, mov: mov, seg: seg, frames: frames, byName: names, hasRegions: df.MovieHasRegions(seg), palette: p.Gamma.DisplayPalette(df.PaletteRGBA(seg.PaletteRaw, 256, seg.File.Order)), paletteGeneration: p.Gamma.Generation, pos: start, interval: interval, action1: action(seg.ActionFrame1), action2: action(seg.ActionFrame2), segmentIndex: index, cuesFired: map[int]bool{}}
	p.recordAction(start)
	p.Log(fmt.Sprintf("movie: %s segment %d/%d (%d frames)", name, index+1, len(mov.Segments), frames.Len()))
	if sound := seg.Frames[start].Sound; sound != "" {
		return true, p.playSound(sound)
	}
	return true, nil
}
func (p *MoviePlayer) endSegment() error {
	m := p.active
	if m != nil && m.segmentIndex+1 < len(m.mov.Segments) {
		ok, err := p.enterSegment(m.mov, m.name, m.segmentIndex+1, 0)
		if !ok {
			p.finish(false)
		}
		return err
	}
	p.finish(false)
	return nil
}
func (p *MoviePlayer) recordAction(index int) {
	m := p.active
	if m == nil {
		return
	}
	if index == m.action1 {
		p.Session.MovieActions[1] = true
	}
	if index == m.action2 {
		p.Session.MovieActions[2] = true
	}
}
func (p *MoviePlayer) regionAt(x, y float64) *df.MovieRegion {
	m := p.active
	if m == nil || !m.hasRegions {
		return nil
	}
	x -= float64(m.seg.OriginX)
	y -= float64(m.seg.OriginY)
	for i := range m.seg.Frames[m.pos].Regions {
		r := &m.seg.Frames[m.pos].Regions[i]
		if x >= float64(min(r.X0, r.X1)) && x <= float64(max(r.X0, r.X1)) && y >= float64(min(r.Y0, r.Y1)) && y <= float64(max(r.Y0, r.Y1)) {
			return r
		}
	}
	return nil
}
func (p *MoviePlayer) ClickableAt(x, y float64) bool {
	m := p.active
	if m == nil {
		return false
	}
	if !m.hasRegions {
		return m.interval == 0
	}
	return p.regionAt(x, y) != nil
}
func (p *MoviePlayer) Click(x, y float64) error {
	m := p.active
	if m == nil {
		return nil
	}
	if !m.hasRegions {
		if m.interval == 0 {
			m.pos++
			if m.pos >= m.frames.Len() {
				p.finish(true)
			}
		}
		return nil
	}
	r := p.regionAt(x, y)
	if r == nil {
		return nil
	}
	if r.Sound != "" {
		if err := p.playSound(r.Sound); err != nil {
			return err
		}
	}
	m.lastTick = 0
	p.clickSound = r.Sound
	defer func() { p.clickSound = "" }()
	return p.action(r.Type, r.Target, r.Event)
}
func (p *MoviePlayer) Key(name string, special bool) (bool, error) {
	m := p.active
	if m == nil {
		return false, nil
	}
	if len(name) == 1 && name[0] >= '0' && name[0] <= '9' {
		p.Session.SetWaveVolume(float64(name[0] - '0'))
		return true, nil
	}
	if !special || (name != "." && name != "q") || !m.seg.KeySkips {
		return false, nil
	}
	if p.EscapeSkipsSegment && m.segmentIndex+1 < len(m.mov.Segments) {
		return true, p.endSegment()
	}
	p.finish(true)
	return true, nil
}
func (p *MoviePlayer) playSound(name string) error {
	m := p.active
	if m == nil {
		return nil
	}
	key := strings.ToLower(name)
	var snd *df.Audio
	if loc, ok := m.seg.Sounds[key]; ok {
		a, err := df.DecodeAudio(m.seg.File.Data(loc), m.seg.File.Order)
		if err != nil {
			return err
		}
		snd = &a
	} else {
		var err error
		snd, err = p.Session.AudioLib.Sound(name)
		if err != nil {
			return err
		}
	}
	if snd == nil {
		return nil
	}
	p.eventSounds = slices.DeleteFunc(p.eventSounds, func(h PlayHandle) bool { return h.Done() })
	handle := p.Session.Audio.Play(SoundChannel, snd, PlayOptions{})
	p.eventSounds = append(p.eventSounds, handle)
	m.soundJump = nil
	if follow, ok := m.seg.SoundFollows[key]; ok {
		if frame, ok := m.byName[strings.ToLower(follow)]; ok {
			m.soundJump = &struct {
				frame int
				sound PlayHandle
			}{frame, handle}
		}
	}
	return nil
}
func (p *MoviePlayer) enter(index int) error {
	m := p.active
	m.pos = index
	p.recordAction(index)
	sound := m.seg.Frames[index].Sound
	if sound != "" && sound != p.clickSound {
		return p.playSound(sound)
	}
	return nil
}
func (p *MoviePlayer) action(kind int, target, event string) error {
	m := p.active
	switch kind {
	case 2:
		index, ok := m.byName[strings.ToLower(target)]
		if !ok {
			p.Log(fmt.Sprintf("movie: no frame named %q", target))
			p.finish(false)
			return nil
		}
		return p.enter(index)
	case 3, 4:
		if kind == 4 {
			if index, ok := m.byName[strings.ToLower(target)]; ok && len(p.callStack) < 5 {
				p.callStack = append(p.callStack, movieReturn{m.name, index})
			}
		}
		p.chainTo(event, 0)
	case 5:
		if len(p.callStack) > 0 {
			ret := p.callStack[len(p.callStack)-1]
			p.callStack = p.callStack[:len(p.callStack)-1]
			p.chainTo(ret.file, ret.frame)
		} else {
			p.finish(false)
		}
	case 6:
		if m.pos+1 < m.frames.Len() {
			return p.enter(m.pos + 1)
		}
		return p.endSegment()
	case 7:
		if m.pos > 0 {
			return p.enter(m.pos - 1)
		}
	default:
		return p.endSegment()
	}
	return nil
}
func (p *MoviePlayer) chainTo(next string, start int) {
	p.active = nil
	if next == "" {
		p.finish(false)
		return
	}
	p.Session.Track("movie chain:"+next, true, func(*Task) error { return p.Session.OnPlayMovie(next, &start) })
}
func (p *MoviePlayer) Tick(now float64) (*MovieImage, error) {
	m := p.active
	if m == nil {
		return nil, nil
	}
	if m.segmentStart == 0 {
		m.segmentStart = now
	}
	for c, cue := range m.seg.Cues {
		if m.cuesFired[c] || now-m.segmentStart < float64(float64(cue.Tick)*df.MovieTickMS) {
			continue
		}
		m.cuesFired[c] = true
		if index, ok := m.byName[strings.ToLower(cue.Target)]; ok {
			m.lastTick = now
			if err := p.enter(index); err != nil {
				return nil, err
			}
		}
	}
	if m.soundJump != nil && m.soundJump.sound.Done() {
		frame := m.soundJump.frame
		m.soundJump = nil
		m.lastTick = now
		if err := p.enter(frame); err != nil {
			return nil, err
		}
	}
	if m.interval > 0 && !df.MovieFrameWaits(m.seg, m.pos) {
		if m.lastTick == 0 {
			m.lastTick = now
		}
		if now-m.lastTick >= df.MovieFrameHoldMS(m.seg, m.pos) {
			if m.seg.Frames[m.pos].WaitsForVoice {
				for _, h := range p.eventSounds {
					if !h.Done() {
						return m.frames.Get(m.pos)
					}
				}
			}
			m.lastTick = now
			f := m.seg.Frames[m.pos]
			if err := p.action(f.Type, f.Target, f.Event); err != nil {
				return nil, err
			}
		}
	}
	if p.active == nil {
		return nil, nil
	}
	return p.active.frames.Get(p.active.pos)
}
func (p *MoviePlayer) Abandon() {
	if p.active != nil || p.sequence != nil {
		if p.sequence != nil {
			p.sequence.cutscene = false
		}
		p.finish(true)
	}
}
func (p *MoviePlayer) finish(dismissed bool) {
	interactive := p.active != nil && p.active.hasRegions
	if p.active != nil && p.active.seg.Flags&df.MovieAnyInputAborts == 0 {
		p.Session.Events.Flush()
	}
	p.active = nil
	p.callStack = nil
	if p.played.Len() > 0 {
		names := []string{}
		for name := range p.played.All() {
			names = append(names, name)
		}
		p.played = orderedMap[bool]{}
		if p.Session.OnMoviesDone != nil {
			p.Session.OnMoviesDone(names)
		}
	}
	p.Session.Audio.Halt(VoiceChannel)
	if dismissed || interactive {
		for _, h := range p.eventSounds {
			h.Stop()
		}
	}
	p.eventSounds = nil
	p.Session.Fade.Snapshot = nil
	p.Session.Fade.PendingReveal = true
	p.OnFinished()
	seq := p.sequence
	p.sequence = nil
	if seq != nil {
		seq.done = true
		if seq.cutscene && p.OnCutsceneFinished != nil {
			p.OnCutsceneFinished()
		}
	}
}
