package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"regexp"
	"slices"
	"strings"
)

const PuppetPressFloorMS = 10.0 / 60 * 1000

type Bevel struct {
	Text string
	ID   float64
}
type PuppetPress struct {
	Index int
	Until float64
}
type PuppetPlaque struct {
	Bevels []Bevel
	Chosen *int
}
type PuppetIdle struct {
	Line               df.PuppetDialogue
	MinTicks, MaxTicks int
	DueAt              float64
}
type PuppetAnimation struct {
	Frames []df.PuppetAnimFrame
	Start  float64
}
type PuppetState struct {
	Name              string
	Pup               *df.Puppet
	Scripts           orderedMap[*script.Instance]
	StanceIdx         int
	Visible           bool
	Subtitle          string
	Bevels            []Bevel
	Chosen            *int
	Press             *PuppetPress
	EventWaiter       func(float64)
	SpeakSkip         func()
	Interrupted       bool
	VoiceQueue        []df.PuppetDialogue
	LastPlaque        *PuppetPlaque
	Repeating         bool
	Idle              []*PuppetIdle
	Anim              *PuppetAnimation
	Pose, DefaultPose *df.PuppetAnimFrame
	DefaultStance     int
	byText            map[string]df.PuppetDialogue
}
type PuppetController struct {
	Dispatch              *ScriptDispatch
	Executor              *Executor
	Audio                 AudioSink
	Read                  func(string) ([]byte, error)
	Random, AmbientRandom func() float64
	Param                 func(int) float64
	SetWaveVolume         func(float64) float64
	Puppet                *PuppetState
}

func NewPuppetController(d *ScriptDispatch, e *Executor, audio AudioSink, read func(string) ([]byte, error)) *PuppetController {
	return &PuppetController{Dispatch: d, Executor: e, Audio: audio, Read: read, Random: SeededRandom(1), AmbientRandom: SeededRandom(0x9e3779b8), Param: func(int) float64 { return 0 }, SetWaveVolume: func(v float64) float64 { return v }}
}

var idleLine = regexp.MustCompile(`(?i)^idle [1-4]$`)

func Subtitled(line df.PuppetDialogue) bool {
	return line.Raw != "" && !strings.HasPrefix(line.Raw, "*") && !idleLine.MatchString(line.Ident) && strings.Trim(line.Raw, " ") != ""
}
func (c *PuppetController) OpenPuppetFile(name string) bool {
	key := strings.ToLower(name)
	data, err := c.Read(key)
	if err != nil || data == nil {
		c.Dispatch.Log(fmt.Sprintf("openpuppetfile: %q not available", name))
		return false
	}
	pup, err := df.ReadPuppet(data)
	if err != nil {
		c.Dispatch.Log(fmt.Sprintf("openpuppetfile: %s: %v", name, err))
		return false
	}
	c.open(key, pup)
	return true
}
func (c *PuppetController) open(key string, pup *df.Puppet) {
	p := &PuppetState{Name: key, Pup: pup, Visible: true, Bevels: []Bevel{}, VoiceQueue: []df.PuppetDialogue{}, Idle: []*PuppetIdle{}}
	var main *script.Instance
	for _, ref := range pup.Scripts {
		inst := c.Dispatch.InstanceFrom(pup.File.Data(ref.Location), ref.Name)
		if inst == nil {
			continue
		}
		p.Scripts.Set(ref.Name, inst)
		if ref.Name == "boot script" {
			main = inst
		}
	}
	for _, inst := range p.Scripts.All() {
		if inst != main {
			inst.Parent = main
		}
	}
	if len(pup.DialogueOrder) > 0 {
		line := pup.Dialogue[pup.DialogueOrder[0]]
		frames := pup.AnimLogic(line.AnimLogicLocation)
		if len(frames) > 0 {
			p.Pose = &frames[0]
		}
		p.StanceIdx = line.Stance
	}
	p.DefaultPose, p.DefaultStance = p.Pose, p.StanceIdx
	c.Puppet = p
	c.Dispatch.PuppetScripts = p.Scripts
	c.Dispatch.Log(fmt.Sprintf("puppet opened: %s (%d lines, %d scripts)", key, len(pup.Dialogue), len(pup.Scripts)))
}
func (c *PuppetController) ClosePuppetFile() {
	p := c.Puppet
	if p == nil {
		return
	}
	if p.EventWaiter != nil {
		p.EventWaiter(-1)
	}
	if p.SpeakSkip != nil {
		p.SpeakSkip()
	}
	c.Audio.Halt(VoiceChannel)
	c.Puppet = nil
	c.Dispatch.PuppetScripts = orderedMap[*script.Instance]{}
}
func (p *PuppetState) linesByText() map[string]df.PuppetDialogue {
	if p.byText == nil {
		p.byText = map[string]df.PuppetDialogue{}
		for _, name := range p.Pup.DialogueOrder {
			line := p.Pup.Dialogue[name]
			key := strings.TrimSpace(strings.ToLower(line.Text))
			if _, ok := p.byText[key]; key != "" && !ok {
				p.byText[key] = line
			}
		}
	}
	return p.byText
}
func (c *PuppetController) Speak(task *Task, ident string) {
	p := c.Puppet
	if p == nil {
		return
	}
	asked := strings.ToLower(ident)
	line, ok := p.Pup.Dialogue[asked]
	if !ok {
		line, ok = p.linesByText()[strings.TrimSpace(asked)]
	}
	if !ok {
		c.Dispatch.Log(fmt.Sprintf("puppetspeak: no line %q in %s", ident, p.Name))
		return
	}
	if len(p.VoiceQueue) < 3 {
		p.VoiceQueue = append(p.VoiceQueue, line)
	}
	if p.Interrupted {
		return
	}
	c.playLine(task, p, line)
}
func (c *PuppetController) playLine(task *Task, p *PuppetState, line df.PuppetDialogue) {
	p.Subtitle = ""
	if Subtitled(line) {
		p.Subtitle = line.Text
	}
	p.StanceIdx = line.Stance
	seconds := math.Max(1, float64(len([]rune(line.Raw)))/15)
	audio, err := df.DecodeAudio(p.Pup.File.Data(line.AudioLocation), nil)
	if err != nil {
		c.Dispatch.Log(fmt.Sprintf("puppetspeak %s: %v", line.Ident, err))
	} else {
		seconds = float64(len(audio.Samples)) / float64(audio.SampleRate)
		c.Audio.Play(VoiceChannel, &audio, PlayOptions{})
	}
	frames := p.Pup.AnimLogic(line.AnimLogicLocation)
	if len(frames) > 0 {
		p.Anim = &PuppetAnimation{Frames: frames, Start: c.Executor.Now()}
	}
	skipped := false
	p.SpeakSkip = func() { skipped = true }
	deadline := c.Executor.Now() + seconds*1000 + 150
	defer func() {
		p.SpeakSkip = nil
		c.Audio.Halt(VoiceChannel)
		if c.Puppet == p {
			p.Subtitle = ""
			if p.Anim != nil {
				p.Pose = &p.Anim.Frames[len(p.Anim.Frames)-1]
				p.Anim = nil
			}
		}
	}()
	// Dialogue has an authored duration plus a 150 ms grace period. Other sound
	// waits use PlayHandle.Done; replacing this timing changes conversation pacing.
	task.Wait(func() bool { return skipped || c.Executor.Now() >= deadline })
}
func (c *PuppetController) Frame() *df.PuppetAnimFrame {
	p := c.Puppet
	if p == nil {
		return nil
	}
	if p.Anim != nil {
		idx := int(math.Floor((c.Executor.Now() - p.Anim.Start) / 33.3))
		idx = max(0, min(idx, len(p.Anim.Frames)-1))
		return &p.Anim.Frames[idx]
	}
	return p.Pose
}
func (c *PuppetController) Clear() {
	if p := c.Puppet; p != nil {
		p.Bevels = []Bevel{}
		p.Chosen = nil
		p.Press = nil
		p.Subtitle = ""
	}
}
func (c *PuppetController) Base(ident string) {
	p := c.Puppet
	if p == nil {
		return
	}
	if ident == "" {
		p.Pose = p.DefaultPose
		p.StanceIdx = p.DefaultStance
		p.Anim = nil
		return
	}
	line, ok := p.Pup.Dialogue[strings.ToLower(ident)]
	if !ok {
		c.Dispatch.Log(fmt.Sprintf("puppetbase: no line %q in %s", ident, p.Name))
		return
	}
	p.StanceIdx = line.Stance
	frames := p.Pup.AnimLogic(line.AnimLogicLocation)
	if len(frames) > 0 {
		p.Pose = &frames[0]
		p.Anim = nil
	}
}
func (c *PuppetController) Bevel(text string, id float64) {
	p := c.Puppet
	if p == nil {
		return
	}
	if len(p.Bevels) >= 5 {
		c.Dispatch.Log(fmt.Sprintf("puppetbevel: %s offered a sixth choice (%q), dropped", p.Name, text))
		return
	}
	p.Chosen = nil
	p.Bevels = append(p.Bevels, Bevel{text, id})
}
func (c *PuppetController) Scramble() {
	p := c.Puppet
	if p == nil || len(p.Bevels) < 2 {
		return
	}
	n := len(p.Bevels)
	for i := 0; i < n*5; i++ {
		a, b := int(math.Floor(c.Random()*float64(n))), int(math.Floor(c.Random()*float64(n)))
		p.Bevels[a], p.Bevels[b] = p.Bevels[b], p.Bevels[a]
	}
}
func (c *PuppetController) Key(name string, special bool) bool {
	p := c.Puppet
	if p == nil || !p.Visible || p.SpeakSkip == nil && p.EventWaiter == nil {
		return false
	}
	if len(name) == 1 && name[0] >= '0' && name[0] <= '9' {
		c.SetWaveVolume(float64(name[0] - '0'))
		return true
	}
	if !special || name != "." {
		return false
	}
	if p.SpeakSkip != nil {
		c.SkipLine()
	} else if p.EventWaiter != nil {
		p.EventWaiter(-1)
	}
	return true
}
func (c *PuppetController) SkipLine() {
	if p := c.Puppet; p != nil {
		p.Interrupted = true
		if p.SpeakSkip != nil {
			p.SpeakSkip()
		}
	}
}
func (c *PuppetController) Event(task *Task) float64 {
	p := c.Puppet
	if p == nil {
		return -1
	}
	p.Interrupted = false
	if len(p.Bevels) == 0 {
		return -1
	}
	done := false
	result := -1.
	p.EventWaiter = func(id float64) {
		p.EventWaiter = nil
		p.Idle = nil
		chosen := p.Chosen
		if id == -1 {
			chosen = nil
		}
		p.LastPlaque = &PuppetPlaque{Bevels: slices.Clone(p.Bevels), Chosen: chosen}
		p.VoiceQueue = nil
		result = id
		done = true
	}
	c.armIdleSlots(p)
	task.Fork("puppet idle", func(child *Task) error { c.runIdleSlots(child, p); return nil })
	defer func() {
		if !done {
			p.EventWaiter = nil
			p.Idle = nil
		}
	}()
	task.Wait(func() bool { return done })
	return result
}
func (c *PuppetController) nextIdleDelay(min, max int) float64 {
	spread := max - min
	draw := 0.
	if spread > 0 {
		draw = math.Floor(c.AmbientRandom()*float64(spread)) + 1
	}
	return (float64(min) + draw) * (1000.0 / 60)
}
func (c *PuppetController) armIdleSlots(p *PuppetState) {
	p.Idle = nil
	if c.Param(8) == 0 {
		return
	}
	for i, timer := range p.Pup.IdleTimers {
		line, ok := p.Pup.Dialogue[fmt.Sprintf("idle %d", i+1)]
		if !ok || timer.MinTicks <= 0 || timer.MaxTicks < timer.MinTicks {
			continue
		}
		p.Idle = append(p.Idle, &PuppetIdle{Line: line, MinTicks: timer.MinTicks, MaxTicks: timer.MaxTicks, DueAt: c.Executor.Now() + c.nextIdleDelay(timer.MinTicks, timer.MaxTicks)})
	}
}
func (c *PuppetController) runIdleSlots(task *Task, p *PuppetState) {
	for c.Puppet == p && p.EventWaiter != nil {
		task.Sleep(EngineStepMS)
		if c.Puppet != p || p.EventWaiter == nil {
			return
		}
		if p.Repeating || p.SpeakSkip != nil {
			continue
		}
		now := c.Executor.Now()
		for _, slot := range p.Idle {
			if now < slot.DueAt {
				continue
			}
			slot.DueAt = now + c.nextIdleDelay(slot.MinTicks, slot.MaxTicks)
			c.playLine(task, p, slot.Line)
			if c.Puppet != p {
				return
			}
			if p.Interrupted {
				if p.EventWaiter != nil {
					p.EventWaiter(-1)
				}
				return
			}
			break
		}
	}
}
func (c *PuppetController) repeatLastExchange(task *Task) {
	p := c.Puppet
	if p == nil || p.Repeating {
		return
	}
	last := p.LastPlaque
	if last == nil && len(p.VoiceQueue) == 0 {
		return
	}
	p.Repeating = true
	shown := PuppetPlaque{p.Bevels, p.Chosen}
	p.Bevels = nil
	p.Chosen = nil
	if last != nil {
		p.Bevels, p.Chosen = last.Bevels, last.Chosen
	}
	defer func() {
		if c.Puppet == p {
			p.Interrupted = false
			p.Bevels, p.Chosen = shown.Bevels, shown.Chosen
			p.Repeating = false
		}
	}()
	if last != nil && last.Chosen != nil && *last.Chosen >= 0 && *last.Chosen < len(last.Bevels) {
		mine := last.Bevels[*last.Chosen]
		if line, ok := p.linesByText()[strings.TrimSpace(strings.ToLower(mine.Text))]; ok {
			c.playLine(task, p, line)
		}
	}
	for _, line := range slices.Clone(p.VoiceQueue) {
		if c.Puppet != p {
			return
		}
		c.playLine(task, p, line)
		if p.Interrupted {
			break
		}
	}
}
func (c *PuppetController) Press(task *Task, i int, inPicture bool) {
	p := c.Puppet
	if p == nil {
		return
	}
	if i >= 0 && i < len(p.Bevels) && p.EventWaiter != nil {
		p.Press = &PuppetPress{i, c.Executor.Now() + PuppetPressFloorMS}
		return
	}
	if inPicture && p.EventWaiter != nil {
		run := func(task *Task) error { c.repeatLastExchange(task); return nil }
		if task != nil {
			task.Fork("repeat conversation", run)
		} else {
			c.Executor.Start("repeat conversation", run)
		}
	}
}
func (c *PuppetController) Release(i int) {
	p := c.Puppet
	if p == nil {
		return
	}
	press := p.Press
	if press == nil || press.Index != i {
		p.Press = nil
		return
	}
	if p.EventWaiter == nil {
		return
	}
	p.Chosen = &i
	p.EventWaiter(p.Bevels[i].ID)
}
func (c *PuppetController) Choose(task *Task, i int, inPicture bool) {
	c.Press(task, i, inPicture)
	c.Release(i)
}
