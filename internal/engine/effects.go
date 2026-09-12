package engine

import "math"

type RGBAFrame struct {
	RGBA          []byte
	Width, Height int
}
type FadeRamp struct{ To, Steps float64 }
type FadeState struct {
	Level, LastTick        float64
	Queue                  []FadeRamp
	Snapshot               *RGBAFrame
	PendingReveal, Blanked bool
}

func (f *FadeState) Active() bool { return len(f.Queue) > 0 }
func (f *FadeState) Tick(now float64, scriptBusy bool) {
	if len(f.Queue) == 0 {
		f.LastTick = 0
		if f.PendingReveal && f.Snapshot == nil && !scriptBusy {
			f.PendingReveal = false
			f.Blanked = false
			f.Level = 0
		}
		return
	}
	f.PendingReveal = false
	tick := TicksAt(now)
	if f.LastTick == 0 {
		f.LastTick = tick - 1
	}
	for len(f.Queue) > 0 && f.LastTick < tick {
		f.LastTick++
		ramp := f.Queue[0]
		if ramp.To == 0 {
			f.Snapshot = nil
		}
		delta := 1 / ramp.Steps
		if ramp.To > f.Level {
			f.Level = math.Min(ramp.To, f.Level+delta)
		} else {
			f.Level = math.Max(ramp.To, f.Level-delta)
		}
		if math.Abs(f.Level-ramp.To) < delta/2 {
			f.Level = ramp.To
			f.Queue = f.Queue[1:]
		}
	}
}

type WipeState struct {
	Dir                                 string
	Span, StepMS, Step, Steps, LastTick float64
	Settled                             bool
	From, To                            *RGBAFrame
}

func NewWipeState() WipeState          { return WipeState{Span: 1, StepMS: RampStepMS} }
func (w *WipeState) Active() bool      { return w.Dir != "" && w.From != nil && !w.Settled }
func (w *WipeState) Compositing() bool { return w.Dir != "" && w.From != nil }
func (w *WipeState) End() {
	w.Dir = ""
	w.From = nil
	w.Step = 0
	w.Steps = 0
	w.LastTick = 0
	w.Span = 1
	w.Settled = false
	w.To = nil
}
func (w *WipeState) Tick(now float64) {
	if !w.Active() {
		return
	}
	stepMS := w.StepMS
	if stepMS == 0 || math.IsNaN(stepMS) {
		stepMS = RampStepMS
	}
	if w.LastTick == 0 {
		w.LastTick = now - stepMS
	}
	for w.Active() && now-w.LastTick >= stepMS {
		w.LastTick += stepMS
		w.Step++
		if w.Step >= w.Steps {
			if (w.Dir == "turnleft" || w.Dir == "turnright") && w.Span < 1 {
				w.Settled = true
			} else {
				w.End()
			}
		}
	}
}

// GameClock excludes time spent frozen by menus, dialogs, or lifecycle pauses.
// The scheduler and frame counter both consume this same logical time.
type GameClock struct {
	rawNow, frozenTotal     float64
	frozenSince             *float64
	lastFrameTick           *float64
	FrameCounter, FrameRate float64
	Audio                   AudioSink
}

func NewGameClock(audio AudioSink) *GameClock { return &GameClock{Audio: audio, FrameRate: 3} }
func (c *GameClock) GameTime(raw float64) float64 {
	c.rawNow = raw
	now := raw
	if c.frozenSince != nil {
		now = *c.frozenSince
	}
	return now - c.frozenTotal
}
func (c *GameClock) Frozen() bool { return c.frozenSince != nil }
func (c *GameClock) Freeze() {
	if c.Frozen() {
		return
	}
	at := c.rawNow
	c.frozenSince = &at
	if c.Audio != nil {
		c.Audio.SetSuspended(true)
	}
}
func (c *GameClock) Thaw() {
	if !c.Frozen() {
		return
	}
	c.frozenTotal += c.rawNow - *c.frozenSince
	c.frozenSince = nil
	if c.Audio != nil {
		c.Audio.SetSuspended(false)
	}
}
func (c *GameClock) AdvanceFrames(now float64) {
	period := math.Max(1, jsRound(c.FrameRate))
	tick := TicksAt(now)
	if c.lastFrameTick == nil {
		c.lastFrameTick = &tick
		return
	}
	due := math.Floor((tick - *c.lastFrameTick) / period)
	if due <= 0 {
		return
	}
	c.FrameCounter += math.Min(due, 64)
	*c.lastFrameTick += due * period
}
