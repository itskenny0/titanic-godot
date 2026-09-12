package engine

import (
	"bytes"
	"errors"
	"github.com/itskenny0/titanic-godot/internal/save"
	"github.com/itskenny0/titanic-godot/internal/script"
	"reflect"
	"testing"
)

func TestBadLoadLeavesSessionIntact(t *testing.T) {
	s := builtinTestSession(t)
	s.Interp.Globals.Set("mission", script.Str("current"))
	s.CursorDepth = -3
	s.SetName = "room"
	s.PuppetCtrl.Puppet = &PuppetState{Visible: true}
	s.Scheduler.MakeLoop("prop", "door", "animate", 7)
	before := snapshotMetadata(t, s)
	epoch := s.Interp.CurrentEvent
	for _, data := range [][]byte{{1, 2, 3}, []byte("not a saved game")} {
		ok, err := s.LoadGame(data)
		if err != nil || ok {
			t.Fatal("bad save accepted", err)
		}
	}
	if !reflect.DeepEqual(before, snapshotMetadata(t, s)) || s.CursorDepth != -3 || s.Interp.CurrentEvent != epoch || s.PuppetCtrl.Puppet == nil || s.SetName != "room" {
		t.Fatal("rejected save changed the live session")
	}
	raw, err := save.ReadRaw(save.NeutralTemplate())
	if err != nil {
		t.Fatal(err)
	}
	raw.Containers[0].Data[1] = 'X'
	wrong, err := save.WriteRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.LoadGame(wrong); ok || err != nil || !reflect.DeepEqual(before, snapshotMetadata(t, s)) {
		t.Fatal("wrong game version replaced the session")
	}
}
func TestSaveDialogsThawAndRestoreCancelledScreen(t *testing.T) {
	s := builtinTestSession(t)
	image := &RGBAFrame{Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}}
	s.Fade = FadeState{Level: .4, Queue: []FadeRamp{{To: 1, Steps: 8}}, Snapshot: image, PendingReveal: true, Blanked: true}
	before := s.Fade
	s.OnLoadGame = func(string) ([]byte, error) {
		if !s.Clock.Frozen() || s.Fade.Level != 1 || s.Fade.Snapshot != nil {
			t.Error("load dialog was not frozen over black")
		}
		return nil, nil
	}
	callBuiltin(t, s, "opengame", script.Str("Titanic 1.0"))
	if s.Clock.Frozen() || !reflect.DeepEqual(before, s.Fade) {
		t.Fatal("cancelled load did not return to the previous screen")
	}
	s.OnLoadGame = func(string) ([]byte, error) { return []byte("damaged"), nil }
	callBuiltin(t, s, "opengame")
	if !reflect.DeepEqual(before, s.Fade) {
		t.Fatal("rejected load did not restore the screen")
	}
	wantErr := errors.New("disk full")
	var written []byte
	s.OnSaveGame = func(data []byte, version string) error {
		if !s.Clock.Frozen() || version != "Titanic 1.0" {
			t.Error("save callback outside frozen session")
		}
		written = bytes.Clone(data)
		return wantErr
	}
	task := s.Track("save", false, func(*Task) error {
		_, err := s.Interp.Builtins["savegame"](s.Interp, []script.Value{script.Str("Titanic 1.0")}, nil, nil)
		return err
	})
	s.Pump(0, false, 1000)
	if !task.Done() || !errors.Is(task.Err(), wantErr) || s.Clock.Frozen() {
		t.Fatal("failed save left time frozen", task.Err())
	}
	if _, err := save.Parse(written); err != nil {
		t.Fatal("save callback did not receive a complete Mac-format save", err)
	}
}
