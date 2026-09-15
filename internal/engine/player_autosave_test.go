package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/save"
)

func checkpointPlayer(t *testing.T, bridge PlayerBridge) *Player {
	t.Helper()
	p := NewPlayer(bridge)
	t.Cleanup(p.Close)
	if err := p.Boot(PlayerConfig{}); err != nil {
		t.Fatal(err)
	}
	s := p.Host.Session
	p.Ready = true
	s.CurrentSetFile, s.SetName, s.SetVisible = "room", "room", true
	s.Fade = FadeState{}
	p.Events()
	return p
}

func TestCheckpointDefersUntilManualSaveIsAllowed(t *testing.T) {
	b := &testPlayerBridge{}
	p := checkpointPlayer(t, b)
	p.checkpointMovieFinished() // Initial intro never arms a checkpoint.
	p.serviceCheckpoint(500)
	if len(b.written) != 0 {
		t.Fatal("intro created checkpoint")
	}
	p.checkpointMovieFinished()
	p.Host.Session.SetVisible = false
	if p.canSaveGame() {
		t.Fatal("manual save permitted while view is hidden")
	}
	p.serviceCheckpoint(1000)
	if len(b.written) != 0 || !p.checkpointPending {
		t.Fatal("hidden view lost or wrote pending save")
	}
	p.Host.Session.SetVisible = true
	p.serviceCheckpoint(250)
	p.Host.Session.Fade.Queue = []FadeRamp{{To: 1, Steps: 2}}
	p.serviceCheckpoint(1000)
	if p.canSaveGame() || len(b.written) != 0 {
		t.Fatal("fade allowed save")
	}
	p.Host.Session.Fade.Queue = nil
	p.serviceCheckpoint(250)
	if len(b.written) != 0 {
		t.Fatal("quiet interval did not reset")
	}
	p.serviceCheckpoint(250)
	if len(b.written) != 1 {
		t.Fatal("checkpoint not written once safe")
	}
	for name, data := range b.written {
		if !strings.HasPrefix(name, "autosave:") {
			t.Fatal("checkpoint touched manual saves")
		}
		if _, err := save.Parse(data); err != nil {
			t.Fatal(err)
		}
	}
	events := p.Events()
	if playerEvent(events, "autosaving") == nil || playerEvent(events, "autosaved") == nil || playerEvent(events, "dialog") != nil {
		t.Fatal("wrong checkpoint events")
	}
	p.serviceCheckpoint(1000)
	if playerEvent(p.Events(), "autosaving") != nil {
		t.Fatal("same checkpoint repeated")
	}
}

func TestCheckpointDefersForScriptsAndFreezeAndRespectsSetting(t *testing.T) {
	b := &testPlayerBridge{}
	p := checkpointPlayer(t, b)
	p.serviceCheckpoint(50)
	p.checkpointMovieFinished()
	s := p.Host.Session
	done := false
	s.Track("cutscene follow-up", false, func(task *Task) error { task.Wait(func() bool { return done }); return nil })
	p.pump(false)
	p.serviceCheckpoint(1000)
	if p.canSaveGame() || len(b.written) != 0 {
		t.Fatal("active script allowed checkpoint")
	}
	done = true
	p.pump(false)
	s.Clock.Freeze()
	p.serviceCheckpoint(1000)
	if len(b.written) != 0 {
		t.Fatal("frozen game saved")
	}
	s.Clock.Thaw()
	p.Paused = true
	p.serviceCheckpoint(1000)
	p.Command(PlayerCommand{Action: "autosave_enabled", On: false})
	p.Paused = false
	p.serviceCheckpoint(1000)
	if len(b.written) != 0 || p.checkpointPending {
		t.Fatal("disabled autosave remained queued")
	}
	p.Command(PlayerCommand{Action: "autosave_enabled", On: true})
	p.checkpointMovieFinished()
	p.checkpointMovieStarted("MOVIES/PLAYMODE.MOV")
	p.serviceCheckpoint(1000)
	if len(b.written) != 0 {
		t.Fatal("new voyage inherited an old checkpoint")
	}
}

type failedCheckpointBridge struct{ testPlayerBridge }

func (b *failedCheckpointBridge) Write(string, []byte) error { return fmt.Errorf("disk full") }

func TestCheckpointWriteFailureDoesNotFreezeOrRetryContinuously(t *testing.T) {
	p := checkpointPlayer(t, &failedCheckpointBridge{})
	p.serviceCheckpoint(50)
	p.checkpointMovieFinished()
	p.serviceCheckpoint(500)
	if p.Host.Session.Clock.Frozen() || p.checkpointPending {
		t.Fatal("failure froze or retained checkpoint")
	}
	events := p.Events()
	if playerEvent(events, "autosave_failed") == nil || playerEvent(events, "error") != nil {
		t.Fatal("write failure must be nonmodal")
	}
	p.serviceCheckpoint(1000)
	if playerEvent(p.Events(), "autosaving") != nil {
		t.Fatal("failed write retried every frame")
	}
}

func TestCutsceneCompletionSignalOnceAndNotOnAbandon(t *testing.T) {
	for _, abandon := range []bool{false, true} {
		p, _ := newMovieTest(t)
		count := 0
		p.OnCutsceneFinished = func() { count++ }
		mov := movieFixture()
		startMovieTest(t, p, mov)
		p.sequence = &movieSequence{cutscene: movieIsCutscene(mov)}
		if abandon {
			p.Abandon()
		} else {
			p.finish(false)
			p.finish(false)
		}
		want := 1
		if abandon {
			want = 0
		}
		if count != want {
			t.Fatalf("completion count %d, want %d", count, want)
		}
	}
	mov := movieFixture()
	mov.Segments[0].Frames[0].Regions = []df.MovieRegion{{Type: 2}}
	if movieIsCutscene(mov) {
		t.Fatal("interactive menu classified as cutscene")
	}
}
