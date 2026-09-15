package engine

import (
	"path"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/save"
)

// Both manual saves and checkpoints use the same gameplay permission gate.
func (p *Player) canSaveGame() bool {
	if p.Host == nil || !p.Ready || p.Paused {
		return false
	}
	s := p.Host.Session
	return s.CurrentSetFile != "" && !p.Host.Director.InputLocked() && s.ViewShowing()
}

func (p *Player) checkpointMovieStarted(name string) {
	switch path.Base(strings.ToLower(strings.ReplaceAll(name, "\\", "/"))) {
	case "menu.mov", "playmode.mov", "playmore.mov", "credits.mov":
		// A new voyage must not inherit a checkpoint from an ending or intro.
		p.checkpointArmed, p.checkpointPending, p.checkpointQuiet = false, false, 0
	}
}

func (p *Player) checkpointMovieFinished() {
	if p.checkpointArmed && !p.autosaveDisabled {
		p.checkpointPending, p.checkpointQuiet = true, 0
	}
}

func (p *Player) serviceCheckpoint(dt float64) {
	if !p.canSaveGame() || p.busy || p.Host.Session.Clock.Frozen() {
		p.checkpointQuiet = 0
		return
	}
	p.checkpointArmed = true
	if !p.checkpointPending || p.autosaveDisabled {
		return
	}
	// Chained movies and their following scripts get time to settle. If control
	// is taken away again, start this quiet interval over, without losing the request.
	p.checkpointQuiet += dt
	if p.checkpointQuiet < 500 {
		return
	}
	p.checkpointPending, p.checkpointQuiet = false, 0
	p.emit("autosaving", nil)
	err := p.frozen(func() error {
		data, err := p.Host.Session.SnapshotSave()
		if err != nil {
			return err
		}
		if _, err = save.Parse(data); err != nil {
			return err
		}
		label := p.Host.Session.CurrentSetFile + " " + p.Now().UTC().Format("2006-01-02-15-04-05")
		return p.write("autosave:"+label, data)
	})
	if err != nil {
		// A full disk must not stop gameplay or retry a failing write every frame.
		p.emit("autosave_failed", map[string]any{"text": err.Error()})
		return
	}
	p.emit("autosaved", nil)
}
