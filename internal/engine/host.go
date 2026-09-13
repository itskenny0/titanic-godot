package engine

import (
	"fmt"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/save"
	"github.com/itskenny0/titanic-godot/internal/script"
)

type HostFiles interface {
	Provide(string) ([]byte, error)
	SetDisc(int)
	ActiveDisc() int
	Has(string) bool
	Evict(string) int
}
type HostUI struct {
	Log, HUD                           func(string)
	ShowStage, MapChanged, SetsChanged func()
}
type ActivateOptions struct{ SkipOpen, Scripted bool }
type GameHost struct {
	Files      HostFiles
	UI         HostUI
	Session    *Session
	Director   *ScreenDirector
	LoadedSets map[string]*df.Set
	Viewer     *SetViewer
	ThemeMix   float64
	currentKey string
}

func NewGameHost(files HostFiles, audio AudioSink, ui HostUI) *GameHost {
	if ui.Log == nil {
		ui.Log = func(string) {}
	}
	if ui.HUD == nil {
		ui.HUD = func(string) {}
	}
	if ui.ShowStage == nil {
		ui.ShowStage = func() {}
	}
	if ui.MapChanged == nil {
		ui.MapChanged = func() {}
	}
	if ui.SetsChanged == nil {
		ui.SetsChanged = func() {}
	}
	s := NewSession(files.Provide, audio)
	h := &GameHost{Files: files, UI: ui, Session: s, Director: NewScreenDirector(s, ScreenWidth, ScreenHeight, nil), LoadedSets: map[string]*df.Set{}, ThemeMix: 128}
	h.Director.Log = ui.Log
	s.Log = ui.Log
	s.Photos.Log = ui.Log
	s.OnMoviesDone = func(names []string) {
		freed := 0
		for _, name := range names {
			freed += files.Evict(name)
		}
		if freed > 1<<20 {
			ui.Log(fmt.Sprintf("movies done: freed %.1f MB", float64(freed)/(1<<20)))
		}
	}
	s.OnAbandonMovie = h.Director.Movies.Abandon
	s.OnDiscChange = func(disc int) {
		if files.ActiveDisc() == disc {
			return
		}
		files.SetDisc(disc)
		ui.Log(fmt.Sprintf("disc %d (titanic%d) mounted", disc, disc))
	}
	s.OnSetChange = func(file, scene, view string) error {
		data, err := files.Provide(file)
		if err != nil {
			return err
		}
		if data == nil {
			ui.Log("cannot travel to " + file + ": file not available")
			return nil
		}
		if !h.parseInto(file, data, ui.Log) {
			return nil
		}
		base := strings.TrimSuffix(file, ".set")
		wanted := append(siblingFiles(base), h.BootPlan().Casts...)
		h.preloadFiles(wanted)
		ui.SetsChanged()
		return h.ActivateSet(file, scene, view, ActivateOptions{Scripted: true})
	}
	return h
}
func (h *GameHost) Close()                   { h.Session.Close() }
func (h *GameHost) Screen() *ScreenPresenter { return h.Director.Screen }
func (h *GameHost) BootPlan() *BootPlan      { return h.Session.BootPlan() }
func siblingFiles(base string) []string {
	return []string{base + ".shp", base + ".prp", base + ".trk", base + ".snd", base + ".sfx", base + ".11k"}
}
func (h *GameHost) preloadFiles(names []string) {
	for _, name := range names {
		_, _ = h.Files.Provide(name)
	}
}
func (h *GameHost) parseInto(name string, data []byte, fail func(string)) bool {
	set, err := df.ReadSet(data)
	if err != nil {
		fail(fmt.Sprintf("cannot parse %s: %v", name, err))
		return false
	}
	h.LoadedSets[name] = set
	return true
}
func (h *GameHost) releaseSet(name string) {
	base := strings.TrimSuffix(strings.ToLower(name), ".set")
	if h.Session.Props.Shops.Has(base + ".shp") {
		h.Session.CloseShop(base + ".shp")
	}
	for _, bank := range []string{base + ".sfx", base + ".11k", base + ".snd"} {
		h.Session.AudioLib.CloseBank(bank)
	}
	delete(h.LoadedSets, base+".set")
	freed := 0
	for _, file := range []string{base + ".set", base + ".shp", base + ".prp", base + ".sfx", base + ".11k", base + ".snd"} {
		freed += h.Files.Evict(file)
	}
	if freed > 0 {
		h.UI.Log(fmt.Sprintf("left %s: freed %.1f MB", base, float64(freed)/(1<<20)))
	}
}

// Lifecycle operations run inside a Session task, allowing authored handlers to
// wait for frames, speech and dialogs without blocking the Godot thread.
func (h *GameHost) ActivateSet(name, scene, view string, opts ActivateOptions) error {
	set := h.LoadedSets[name]
	if set == nil {
		return nil
	}
	s := h.Session
	if h.currentKey != "" && h.currentKey != name {
		h.releaseSet(h.currentKey)
	}
	h.currentKey = name
	if !opts.Scripted {
		s.Scheduler.Reset()
	}
	s.CurrentSetFile = strings.TrimSuffix(strings.ToLower(name), ".set")
	s.EnsureBooted()
	v, err := NewSetViewer(set, s, scene, view, h.Director.Gamma, h.Director)
	if err != nil {
		return err
	}
	h.Viewer = v
	h.Director.SetRoom(v)
	if s.NavGestureActive {
		v.ArmNavHooks()
	}
	v.BindJumpHooks()
	v.OnHUD = h.UI.HUD
	v.Log = h.UI.Log
	s.OnPlayMovie = func(name string, start *int) error {
		if h.Viewer == nil {
			return nil
		}
		if _, err := h.Files.Provide(name); err != nil {
			return err
		}
		at := 0
		if start != nil {
			at = *start
		}
		return h.Director.Movies.Play(s.Executor.Current(), name, at)
	}
	if err = v.ShowView(); err != nil {
		return err
	}
	h.UI.ShowStage()
	h.UI.MapChanged()
	if opts.SkipOpen {
		return nil
	}
	v.Start()
	return v.StartTheme()
}
func (h *GameHost) WarmBootResources() {
	h.preloadFiles(h.BootPlan().Resources)
	h.UI.Log("boot resources ready")
}
func (h *GameHost) Preload() {
	plan := h.BootPlan()
	wanted := append([]string{"bootfile"}, plan.Resources...)
	if plan.LandingSet != nil {
		wanted = append(wanted, *plan.LandingSet)
		wanted = append(wanted, siblingFiles(strings.TrimSuffix(*plan.LandingSet, ".set"))...)
	}
	seen := map[string]bool{}
	for _, name := range wanted {
		if !seen[name] && !h.Files.Has(name) {
			_, _ = h.Files.Provide(name)
		}
		seen[name] = true
	}
	h.UI.Log("boot resources ready")
}
func (h *GameHost) LoadSet(name string, opts ActivateOptions) error {
	h.UI.HUD("loading " + name)
	data, err := h.Files.Provide(name)
	if err != nil {
		return err
	}
	if data == nil {
		h.UI.HUD("could not load " + name)
		return nil
	}
	wanted := append(siblingFiles(strings.TrimSuffix(name, ".set")), h.BootPlan().Resources...)
	h.preloadFiles(wanted)
	if !h.parseInto(name, data, h.UI.HUD) {
		return nil
	}
	h.UI.SetsChanged()
	return h.ActivateSet(name, "", "", opts)
}
func (h *GameHost) LoadSavedGame(data []byte) error {
	game, err := save.Parse(data)
	if err != nil {
		h.UI.HUD("not a valid saved game: " + err.Error())
		return nil
	}
	h.Director.Movies.Abandon()
	if err := h.LoadSet(game.Set+".set", ActivateOptions{SkipOpen: true}); err != nil {
		return err
	}
	if h.Viewer == nil {
		h.UI.HUD("could not load the saved game's set (" + game.Set + ")")
		return nil
	}
	_, err = h.Session.LoadGame(data)
	if err != nil {
		return err
	}
	h.Session.Fade.Level = 0
	return nil
}
func (h *GameHost) Restart(task *Task) error {
	if err := h.Session.PrepareRestart(task); err != nil {
		return err
	}
	return h.ColdBoot()
}
func (h *GameHost) ColdBoot() error {
	s := h.Session
	s.Interp.Globals.Set("tour", script.Num(0))
	s.Fade.Queue = nil
	s.Fade.Snapshot = nil
	s.Fade.PendingReveal = false
	s.Fade.Blanked = false
	s.Fade.Level = 1
	plan := h.BootPlan()
	h.preloadFiles(plan.Resources)
	if plan.LandingSet == nil {
		h.UI.Log("this boot names no first room: booting its own way")
		if !s.BootedByGame() {
			h.UI.HUD("no bootfile in this edition: nothing to boot")
			return nil
		}
		s.Fade.Level = 1
		s.SetName = "none"
		s.CurrentSetFile = ""
		if _, err := s.RunGlobal("boot", nil); err != nil {
			return err
		}
		h.startAtHalfMix()
		return nil
	}
	if !s.BootedByGame() {
		h.UI.HUD("no bootfile in this edition: nothing to boot")
		return nil
	}
	if err := h.LoadSet(*plan.LandingSet, ActivateOptions{SkipOpen: true}); err != nil {
		return err
	}
	s.Fade.Level = 1
	s.SuppressStageBootFallback = true
	err := func() error {
		defer func() { s.SuppressStageBootFallback = false }()
		_, err := s.RunGlobal("boot", nil)
		return err
	}()
	if err != nil {
		return err
	}
	s.SetName = "none"
	s.CurrentSetFile = ""
	h.startAtHalfMix()
	tour, _ := s.Interp.Globals.Get("tour")
	handler := "advanceday"
	if propertyNumber(tour) != 0 {
		handler = "advancetour"
	}
	_, err = s.RunGlobal(handler, nil)
	return err
}
func (h *GameHost) startAtHalfMix() {
	s := h.Session
	s.Interp.Globals.Set("themevolume", script.Num(h.ThemeMix))
	s.WaveVolume = 5
	s.SetThemeVolume(h.ThemeMix, "")
	for _, ch := range []AudioChannel{SoundChannel, VoiceChannel} {
		s.Audio.SetChannelVolume(ch, .5)
	}
}
