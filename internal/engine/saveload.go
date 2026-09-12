package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/save"
	"github.com/itskenny0/titanic-godot/internal/script"
	"slices"
	"strings"
)

// LoadGame validates the complete envelope and restoration trailer before
// abandoning scripts or changing state. The host opens core resources first.
func (s *Session) LoadGame(data []byte) (bool, error) {
	game, err := save.Parse(data)
	if err != nil {
		s.Log("opengame: not a valid saved game file (" + err.Error() + ")")
		return false, nil
	}
	if game.Title != "Titanic 1.0" {
		s.Log(fmt.Sprintf("opengame: saved game is from a different version (%q)", game.Title))
		return false, nil
	}
	s.Interp.Abandon()
	s.CursorDepth = 0
	s.mountSavedDisc(game.Disk)
	for _, name := range s.Interp.Globals.Keys() {
		if strings.HasPrefix(name, "__") {
			continue
		}
		_, number := game.NumGlobals[name]
		_, text := game.StrGlobals[name]
		if !number && !text {
			s.Interp.Globals.Delete(name)
		}
	}
	for _, name := range game.NumGlobalOrder {
		s.Interp.Globals.Set(name, script.Num(game.NumGlobals[name]))
	}
	for _, name := range game.StrGlobalOrder {
		s.Interp.Globals.Set(name, script.Str(game.StrGlobals[name]))
	}
	if game.Hallside != "" {
		s.Interp.Globals.Set("hallside", script.Str(game.Hallside))
	}
	if game.Savedeck != "" {
		s.Interp.Globals.Set("savedeck", script.Str(game.Savedeck))
	}
	s.Clock.FrameCounter = game.Frame
	s.Interp.Globals.Set("lockevents", script.Num(0))
	s.Scheduler.Reset()
	s.Audio.Halt(VoiceChannel)
	s.PuppetCtrl.ClosePuppetFile()
	s.Wipe.End()
	if s.OnAbandonMovie != nil {
		s.OnAbandonMovie()
	}
	s.StageCtrl.ResetOverlayStack()
	s.StageCtrl.CloseStageFile()
	s.StageCtrl.OpenStageFile("main.stg")
	s.SetVisible = true
	s.RestoringSave = true
	defer func() { s.RestoringSave = false }()
	s.SetCurrentSetName("none")
	s.CurrentSetFile = ""
	for _, file := range game.CastFiles {
		s.OpenCastFile(file)
	}
	s.ResetCast()
	s.RestoreActors(game.Actors)
	for _, name := range slices.Clone(s.Props.Shops.keys) {
		if shop := s.Props.Shops.Get(name); shop != nil && !shop.Persistent {
			s.CloseShop(name)
		}
	}
	s.RestoreProps(game.Inventory)
	for _, l := range game.Loops {
		s.Scheduler.RestoreLoop(l.Kind, l.Name, l.Handler, l.Period)
	}
	for _, c := range game.Crickets {
		s.Scheduler.RestoreCricket(c.Name, c.Set, c.X, c.Y, c.Radius, c.Base, c.Jitter, c.Next)
	}
	for _, w := range game.Walks {
		a := s.Actors.Get(w.Actor)
		usable := w.Type == 0 || w.Type == 1 || w.Type == 3 && w.Path != nil
		resumed := false
		if a != nil && usable {
			walk := Walk{TurnOnly: w.Type == 0, Paused: w.Paused, SX: w.StartX, SY: w.StartY, SZ: w.StartZ, DX: w.DestX - w.StartX, DY: w.DestY - w.StartY, DZ: w.DestZ - w.StartZ, Dist: w.Dist, Progress: w.Progress}
			if w.TurnTo >= 0 {
				v := w.TurnTo
				walk.TurnTo = &v
			}
			if w.Star != "" {
				star := w.Star
				walk.ArriveStar = &star
			}
			if w.Path != nil {
				walk.Path = make([]WalkPoint, len(w.Path))
				for i, p := range w.Path {
					walk.Path[i] = WalkPoint{X: p.X, Y: p.Y, Z: p.Z, Cum: p.Cum}
				}
			}
			resumed = s.Scheduler.RestoreWalk(w.Actor, walk)
		}
		if resumed {
			if w.Type == 0 {
				s.Log("loadgame: " + w.Actor + " was saved mid-turn; resuming it")
			} else {
				s.Log(fmt.Sprintf("loadgame: %s was saved walking to %q; resuming it", w.Actor, w.Star))
			}
			continue
		}
		if a != nil && strings.HasPrefix(a.PoseName, "walk") {
			pose := "stand" + strings.TrimPrefix(a.PoseName, "walk")
			a.PoseName = "stand"
			for _, p := range a.Member.Poses {
				if p.Name == pose {
					a.PoseName = pose
					break
				}
			}
			a.Step = 0
		}
		s.Log("loadgame: " + w.Actor + " was saved mid-walk; standing them at their saved position")
	}
	if err = s.restoreTheme(game); err != nil {
		return false, err
	}
	if err = s.OpenSetFile(game.Set+".set", game.Scene, game.View); err != nil {
		return false, err
	}
	return true, nil
}
func (s *Session) mountSavedDisc(disk string) {
	if disk == "" {
		return
	}
	volumes := s.BootPlan().Volumes
	want := strings.ToLower(strings.TrimSpace(disk))
	idx := slices.Index(volumes, want)
	if idx == 0 || idx == 1 {
		if s.OnDiscChange != nil {
			s.OnDiscChange(idx + 1)
		}
		s.MountedCD = disk
		return
	}
	if len(volumes) > 0 {
		s.Log(fmt.Sprintf("loadgame: saved on %q, which is not a disc this game mounts", disk))
	}
}
func (s *Session) ResetCast() {
	for _, key := range slices.Clone(s.Actors.Actors.keys) {
		a := s.Actors.Actors.Get(key)
		if !strings.EqualFold(a.Member.Name, key) {
			s.Actors.Remove(key)
			s.DropInstancedScript(key)
			continue
		}
		a.Visible = false
		a.Owner = script.Str("none")
		a.Value = script.Num(0)
		a.SetName = ""
		a.StarName = ""
		a.PoseName = "stand"
		a.Scale = 0
	}
}
func (s *Session) RestoreActors(actors []save.SavedActor) {
	for _, sa := range actors {
		a := s.Actors.Get(sa.Name)
		if a == nil {
			src := s.instanceSource(sa.Name)
			if src == "" {
				s.Log(fmt.Sprintf("loadgame: no cast member to re-instance %q from; dropped", sa.Name))
				continue
			}
			s.Actors.Instance(src, sa.Name)
			s.InstanceCastScript(src, sa.Name)
			a = s.Actors.Get(sa.Name)
			if a == nil {
				continue
			}
		}
		a.Owner = script.Str(sa.Owner)
		a.Value = script.Num(sa.Value)
		p := sa.Placement
		if p.Set == "" {
			continue
		}
		a.SetName = p.Set
		a.StarName = p.Star
		a.PoseName = p.Pose
		if a.PoseName == "" {
			a.PoseName = "stand"
		}
		a.WorldX = p.X
		a.WorldY = p.Y
		a.WorldZ = p.Z
		a.Deg = float64(int32JS(p.Deg) & 255)
		if p.Speed != 0 {
			a.Speed = p.Speed
		}
		if p.Turn != 0 {
			a.Turn = p.Turn
		}
		a.Zclip = p.Zclip
		a.Visible = p.Visible
		if p.Scale > 0 {
			a.Scale = p.Scale
		}
	}
}
func (s *Session) instanceSource(name string) string {
	units := script.UTF16Units(name)
	for n := len(units) - 1; n >= 2; n-- {
		prefix := script.StringFromUTF16(units[:n])
		for _, src := range []string{prefix, prefix + "1"} {
			if src != name && s.Actors.Get(src) != nil {
				return src
			}
		}
	}
	return ""
}
func (s *Session) RestoreProps(inventory []save.SavedProp) {
	for _, sp := range inventory {
		p := s.Props.Get(sp.Name)
		if p == nil {
			continue
		}
		p.Owner = script.Str(sp.Owner)
		p.Value = script.Num(sp.Value)
		p.Visible = sp.Visible
		p.WorldSpace = sp.Is3d
		if !sp.Is3d {
			p.AnchorX = sp.X
			p.AnchorY = sp.Y
		}
		p.Deg = script.Num(sp.Deg)
		p.Dist = sp.Dist
		if sp.Scale != 0 {
			p.Scale = sp.Scale
		}
		p.Zclip = sp.Zclip
		p.StateName = sp.View
		p.LastTick = 0
		p.FrameLocked = false
		st := p.State()
		twoFrameSelector := st != nil && len(st.Frames) == 2 && !st.Animated
		if p.DegVariants || twoFrameSelector {
			p.FrameOrder = nil
			p.FrameIdx = 0
			if st != nil {
				p.FrameIdx = FrameIndexForDegree(st, propertyNumberOrZero(p.Deg))
			}
			p.FrameLocked = true
			p.Animating = false
		} else {
			p.FrameIdx = 0
			p.FrameOrder = nil
			if st != nil {
				p.FrameOrder = PlaySequence(st, DegreeVariantFrames(st, propertyNumberOrZero(p.Deg)))
			}
			p.Animating = st != nil && p.FrameCount(st) > 1
		}
	}
}
func (s *Session) restoreTheme(game *save.Game) error {
	s.Audio.Halt(ThemeChannel)
	s.CurrentThemeName = "none"
	for _, bank := range game.TrackFiles {
		s.OpenTrackFile(bank)
	}
	t := game.Theme
	if t == nil {
		return nil
	}
	s.OpenTrackFile(t.Track)
	theme, err := s.AudioLib.Theme(t.Track)
	if err != nil {
		return err
	}
	if theme == nil {
		s.Log(fmt.Sprintf("loadgame: saved theme %q is not available; the room loads silent", t.Track))
		return nil
	}
	s.Audio.Play(ThemeChannel, theme, PlayOptions{Loop: true})
	s.CurrentThemeName = t.Track
	volume := 255.
	if v, ok := s.Interp.Globals.Get("themevolume"); ok {
		volume = v.Num()
	}
	s.SetThemeVolume(volume, "")
	if t.Extras > 0 {
		s.Log(fmt.Sprintf("loadgame: %g additional saved sound loop(s) not restored (re-armed by the room)", t.Extras))
	}
	return nil
}
