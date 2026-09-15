package engine

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/itskenny0/titanic-godot/internal/hdpack"
	"github.com/itskenny0/titanic-godot/internal/save"
)

// PlayerBridge is the complete platform boundary. Godot owns file dialogs,
// storage and font metrics. Calls from scripts are marshalled back to Pump's
// thread; the exported C entry points must lock that thread for their duration.
type PlayerBridge interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
	Measure(string, string) (float64, error)
}
type PlayerConfig struct {
	DisableAutosave bool              `json:"disable_autosave"`
	HDPack          string            `json:"hd_pack"`
	Index           map[string]string `json:"index"`
	Save            string            `json:"save"`
	Testing         bool              `json:"testing"`
}
type PlayerCommand struct {
	Action             string `json:"action"`
	Kind               string `json:"kind"`
	Key                any    `json:"key"`
	X, Y               float64
	Radius             float64 `json:"radius"`
	Shift, Special, On bool
	ID                 uint64 `json:"id"`
	Value              any    `json:"value"`
	Path               string `json:"path"`
}
type playerDialog struct {
	ready bool
	value any
}
type Player struct {
	Bridge                                               PlayerBridge
	Host                                                 *GameHost
	Audio                                                *HostAudio
	Context                                              *DrawContext
	Ready, Paused                                        bool
	Testing                                              bool
	Now                                                  func() time.Time
	pauseOwned, busy                                     bool
	pointerOffsetX, pointerOffsetY                       float64
	now                                                  float64
	renderVersion                                        uint64
	events                                               []any
	nextDialog                                           uint64
	dialogs                                              map[uint64]*playerDialog
	testFailTick                                         bool
	profiling                                            bool
	profileTick, profileRender                           time.Duration
	autosaveDisabled, checkpointArmed, checkpointPending bool
	checkpointQuiet                                      float64
}

func NewPlayer(bridge PlayerBridge) *Player {
	p := &Player{Bridge: bridge, Audio: new(HostAudio), Context: NewDrawContext(ScreenWidth, ScreenHeight), Now: time.Now, dialogs: map[uint64]*playerDialog{}}
	p.Context.Measure = func(text, font string) float64 {
		v, err := p.effect(func() (any, error) { return p.Bridge.Measure(text, font) })
		if err != nil {
			p.fail(err)
			return 0
		}
		return v.(float64)
	}
	return p
}
func (p *Player) effect(run func() (any, error)) (any, error) {
	if p.Host != nil {
		if task := p.Host.Session.Executor.Current(); task != nil {
			return task.Effect(run)
		}
	}
	return run()
}
func (p *Player) read(path string) ([]byte, error) {
	v, err := p.effect(func() (any, error) { return p.Bridge.Read(path) })
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.([]byte), nil
}
func (p *Player) write(path string, data []byte) error {
	_, err := p.effect(func() (any, error) { return nil, p.Bridge.Write(path, data) })
	return err
}
func (p *Player) flushAudio() {
	for _, event := range p.Audio.DrainEvents() {
		p.events = append(p.events, event)
	}
}
func (p *Player) emit(kind string, fields map[string]any) {
	p.flushAudio()
	if fields == nil {
		fields = map[string]any{}
	}
	fields["type"] = kind
	p.events = append(p.events, fields)
}
func (p *Player) fail(err error) {
	if err != nil {
		p.emit("log", map[string]any{"text": err.Error()})
		p.emit("error", map[string]any{"text": err.Error()})
	}
}
func (p *Player) Boot(config PlayerConfig) error {
	if p.Host != nil {
		return fmt.Errorf("player already booted")
	}
	p.Testing = config.Testing
	p.autosaveDisabled = config.DisableAutosave
	files := NewFiles(config.Index, p.read)
	p.Host = NewGameHost(files, p.Audio, HostUI{Log: func(text string) { p.emit("log", map[string]any{"text": text}) }, HUD: func(text string) { p.emit("status", map[string]any{"text": text}) }, ShowStage: func() { p.emit("stage", nil) }})
	if config.HDPack != "" {
		pack, err := hdpack.Open(config.HDPack, p.read, func(text string) { p.emit("log", map[string]any{"text": text}) })
		if err != nil {
			p.emit("log", map[string]any{"text": fmt.Sprintf("HD pack unavailable: %v; using original artwork", err)})
		} else {
			p.Host.Screen().HD = NewHDSurface(pack, ScreenWidth, ScreenHeight)
			p.emit("log", map[string]any{"text": fmt.Sprintf("HD pack: %d images, 2x display, 24 MiB image cache", len(pack.Manifest.Images))})
		}
	}
	s := p.Host.Session
	s.PictureMode = "sharp"
	s.HasRealFrames = true
	s.OnNoteDialog = func(text string) error { return p.frozen(func() error { p.dialog("note", text, ""); return nil }) }
	s.OnQuestionDialog = func(text string) (bool, error) {
		var yes bool
		err := p.frozen(func() error { yes = playerTruthy(p.dialog("question", text, "")); return nil })
		return yes, err
	}
	s.OnTextDialog = func(text, initial string) (string, error) {
		var value string
		err := p.frozen(func() error { value = playerString(p.dialog("text", text, initial)); return nil })
		return value, err
	}
	s.OnQuit = func() error { p.emit("quit", nil); return nil }
	s.OnSaveGame = func(data []byte, _ string) error { p.fail(p.save(data)); return nil }
	s.OnLoadGame = func(string) ([]byte, error) { data, err := p.chooseSave(); p.fail(err); return data, nil }
	p.Host.Director.OnCursor = func(name string) { p.emit("cursor", map[string]any{"name": name}) }
	p.Host.Director.Movies.OnStarted = p.checkpointMovieStarted
	p.Host.Director.Movies.OnCutsceneFinished = p.checkpointMovieFinished
	s.Track("coldBoot", false, func(*Task) error {
		p.Host.Preload()
		p.Ready = true
		p.emit("ready", nil)
		if config.Save != "" {
			data, err := p.read(config.Save)
			if err != nil {
				return err
			}
			if data == nil {
				return fmt.Errorf("cannot read save")
			}
			if _, err := save.Parse(data); err != nil {
				return err
			}
			return p.Host.LoadSavedGame(data)
		}
		return p.Host.ColdBoot()
	})
	p.pump(false)
	return nil
}
func (p *Player) Close() {
	if p.Host != nil {
		p.Host.Close()
		p.Host = nil
	}
	clear(p.dialogs)
}
func (p *Player) pump(frame bool) {
	if p.Host == nil {
		return
	}
	s := p.Host.Session
	for _, done := range s.Pump(s.Executor.Now(), frame, 10000) {
		p.fail(done.Err)
	}
}
func (p *Player) Tick(dt float64) error {
	if p.testFailTick {
		p.testFailTick = false
		return fmt.Errorf("expected tick failure test")
	}
	if p.Host == nil || p.Paused {
		return nil
	}
	if math.IsNaN(dt) || math.IsInf(dt, 0) || dt < 0 {
		return fmt.Errorf("invalid frame duration")
	}
	began := time.Now()
	p.now += dt
	if _, err := p.Host.Director.Tick(p.now); err != nil {
		return err
	}
	p.pump(true)
	renderStart := time.Now()
	if err := p.Host.Director.Render(p.Context); err != nil {
		return err
	}
	p.serviceCheckpoint(dt)
	if p.profiling {
		p.profileTick += renderStart.Sub(began)
		p.profileRender += time.Since(renderStart)
	}
	return nil
}
func (p *Player) dialog(kind, text, value string) any {
	task := p.Host.Session.Executor.Current()
	if task == nil {
		panic("dialog outside game task")
	}
	p.nextDialog++
	id := p.nextDialog
	d := &playerDialog{}
	p.dialogs[id] = d
	defer delete(p.dialogs, id)
	p.emit("dialog", map[string]any{"id": id, "kind": kind, "text": text, "value": value})
	task.Wait(func() bool { return d.ready })
	return d.value
}
func (p *Player) frozen(run func() error) error {
	clock := p.Host.Session.Clock
	owns := !clock.Frozen()
	if owns {
		clock.Freeze()
		defer clock.Thaw()
	}
	return run()
}
func (p *Player) save(data []byte) error {
	label := p.Host.Session.CurrentSetFile
	if label == "" {
		label = "Voyage"
	}
	name := playerString(p.dialog("save", "Name this saved game.", label+" "+p.Now().UTC().Format("2006-01-02-15-04-05")))
	if name == "" {
		return nil
	}
	if _, err := save.Parse(data); err != nil {
		return err
	}
	if err := p.write("save:"+name, data); err != nil {
		return err
	}
	p.emit("saved", map[string]any{"name": name})
	return nil
}
func (p *Player) chooseSave() ([]byte, error) {
	path := playerString(p.dialog("load", "Choose a saved game.", ""))
	if path == "" {
		return nil, nil
	}
	data, err := p.read(path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("unable to read this save")
	}
	if _, err := save.Parse(data); err != nil {
		return nil, err
	}
	return data, nil
}
func playerString(value any) string {
	if !playerTruthy(value) {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}
func playerTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0 && !math.IsNaN(v)
	case int:
		return v != 0
	}
	return true
}
func (p *Player) Command(c PlayerCommand) error {
	if c.Action == "reply" {
		if d := p.dialogs[c.ID]; d != nil {
			d.value, d.ready = c.Value, true
		}
		if !p.Paused {
			p.pump(false)
		}
		return nil
	}
	if c.Action == "audio_done" {
		p.Audio.Finish(c.ID)
		if !p.Paused {
			p.pump(false)
		}
		return nil
	}
	if p.Host == nil {
		return nil
	}
	s, d := p.Host.Session, p.Host.Director
	if c.Action == "autosave_enabled" {
		p.autosaveDisabled = !c.On
		p.checkpointPending, p.checkpointQuiet = false, 0
		return nil
	}
	if c.Action == "pause" {
		if c.On == p.Paused {
			return nil
		}
		p.Paused = c.On
		if p.Paused {
			p.pauseOwned = !s.Clock.Frozen()
			if p.pauseOwned {
				s.Clock.Freeze()
			}
			s.PointerDown = false
			p.pointerOffsetX, p.pointerOffsetY = 0, 0
			d.Release(-1, -1)
		} else {
			if p.pauseOwned {
				s.Clock.Thaw()
			}
			p.pauseOwned = false
		}
		return nil
	}
	if !p.Ready || p.Paused {
		return nil
	}
	switch c.Action {
	case "pointer":
		if c.Kind == "press" {
			x, y := p.assistedPoint(c.X, c.Y, c.Radius)
			p.pointerOffsetX, p.pointerOffsetY = x-c.X, y-c.Y
		}
		if c.Kind == "press" || s.PointerDown {
			c.X += p.pointerOffsetX
			c.Y += p.pointerOffsetY
		}
		if c.Kind == "release" {
			p.pointerOffsetX, p.pointerOffsetY = 0, 0
		}
		s.ShiftDown = c.Shift
		s.SetPointer(c.X, c.Y)
		if c.Kind == "press" {
			s.PointerDown = true
			if err := d.Press(c.X, c.Y); err != nil {
				return err
			}
		} else if c.Kind == "release" {
			s.PointerDown = false
			p.pointerOffsetX, p.pointerOffsetY = 0, 0
			d.Release(c.X, c.Y)
		} else if !s.PointerDown {
			s.Track("hover", true, func(*Task) error { _, err := d.Hover(c.X, c.Y); return err })
		}
	case "key":
		key := playerString(c.Key)
		s.Track("key:"+key, false, func(*Task) error {
			if (key == "uparrow" || key == "leftarrow" || key == "rightarrow") && p.Host.Viewer != nil && (s.ViewShowing() || s.StageCtrl.KeydownTarget() == nil) {
				return p.Host.Viewer.PressNav(key)
			}
			_, err := d.KeyDown(key, c.Special)
			return err
		})
	case "gamma":
		key := 0
		switch n := c.Key.(type) {
		case float64:
			key = int(n)
		case int:
			key = n
		}
		if key == 9 {
			d.Gamma.Reset()
		} else {
			channels := AllGammaChannels
			if key >= 3 {
				channels = [3]bool{key < 5, key >= 5 && key < 7, key >= 7}
			}
			d.Gamma.Step(key%2 == 0, channels)
		}
	default:
		if p.busy {
			return nil
		}
		p.busy = true
		// Menu commands are outside the game's busy-script set, like the original
		// command layer. Saving must not reject its own request as an active script.
		s.Track("menu:"+c.Action, true, func(*Task) error { defer func() { p.busy = false }(); return p.menuCommand(c) })
	}
	p.pump(false)
	return nil
}
func (p *Player) menuCommand(c PlayerCommand) error {
	s := p.Host.Session
	switch c.Action {
	case "save":
		if !p.canSaveGame() {
			return fmt.Errorf("finish the conversation or animation, then save while exploring")
		}
		return p.frozen(func() error {
			data, err := s.SnapshotSave()
			if err != nil {
				return err
			}
			return p.save(data)
		})
	case "load", "import":
		return p.frozen(func() error {
			path := c.Path
			if path == "" {
				path = playerString(p.dialog(c.Action, "Choose a saved game.", ""))
			}
			if path == "" {
				return nil
			}
			data, err := p.read(path)
			if err != nil {
				return err
			}
			if data == nil {
				return fmt.Errorf("cannot read saved game")
			}
			if _, err := save.Parse(data); err != nil {
				return err
			}
			if c.Action == "import" {
				name := strings.ReplaceAll(path, "\\", "/")
				name = name[strings.LastIndex(name, "/")+1:]
				if err := p.write("save:"+name, data); err != nil {
					return err
				}
			}
			p.emit("restart", map[string]any{"save": path})
			return nil
		})
	case "new":
		return p.frozen(func() error {
			if playerTruthy(p.dialog("question", "Return to the main menu? Unsaved progress will be lost.", "")) {
				p.emit("restart", nil)
			}
			return nil
		})
	}
	return nil
}
func (p *Player) Events() []any {
	p.flushAudio()
	events := p.events
	p.events = nil
	if events == nil {
		return []any{}
	}
	return events
}
func (p *Player) Frame() []byte {
	if p.Host == nil || p.Context.Version == p.renderVersion {
		return nil
	}
	p.renderVersion = p.Context.Version
	screen := p.Host.Screen()
	if screen.HD != nil && screen.HD.Valid && screen.HD.Hits > screen.HD.FrameStartHits {
		return screen.HD.Pixels
	}
	return screen.Frame
}
func (p *Player) Overlay() []DrawCommand {
	if p.Context.Commands == nil {
		return []DrawCommand{}
	}
	return p.Context.Commands
}
func (p *Player) TakeAudio(id uint64) []byte { return p.Audio.TakePCM(id) }
func (p *Player) Profile(on bool)            { p.profiling = on }
func (p *Player) Timings() map[string]float64 {
	out := map[string]float64{"tick_ms": float64(p.profileTick) / float64(time.Millisecond), "render_ms": float64(p.profileRender) / float64(time.Millisecond)}
	p.profileTick, p.profileRender = 0, 0
	return out
}
func (p *Player) State() map[string]any {
	if p.Host == nil {
		return map[string]any{}
	}
	s, d := p.Host.Session, p.Host.Director
	var movie any
	if d.Movies.Playing() {
		movie = d.Movies.PlayingFile()
	}
	regions := []map[string]any{}
	for _, r := range d.Movies.WaitingRegions() {
		regions = append(regions, map[string]any{"type": r.Type, "target": r.Target, "event": r.Event, "sound": r.Sound, "x0": r.X0, "y0": r.Y0, "x1": r.X1, "y1": r.Y1, "record": r.Record})
	}
	choices := []map[string]any{}
	for _, c := range d.Choices() {
		choices = append(choices, map[string]any{"text": c.Text, "id": c.ID})
	}
	hdHits := uint64(0)
	if d.Screen.HD != nil {
		hdHits = d.Screen.HD.Hits
	}
	return map[string]any{"hd_hits": hdHits, "ready": p.Ready, "set": s.CurrentSetFile, "scene": s.CurrentSceneName(), "view": s.CurrentViewName(), "disc": p.Host.Files.ActiveDisc(), "movie": movie, "choices": choices, "regions": regions, "frame": s.Clock.FrameCounter, "paused": p.Paused, "inputLocked": d.InputLocked(), "viewShowing": s.ViewShowing(), "frozen": s.Clock.Frozen()}
}
