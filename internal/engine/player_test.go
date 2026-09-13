package engine

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/itskenny0/titanic-godot/internal/script"
)

type testPlayerBridge struct {
	read    func(string) ([]byte, error)
	written map[string][]byte
	measure func(string, string) (float64, error)
}

func (b *testPlayerBridge) Read(path string) ([]byte, error) {
	if data := b.written[path]; data != nil {
		return data, nil
	}
	if b.read != nil {
		return b.read(path)
	}
	return nil, nil
}
func (b *testPlayerBridge) Write(path string, data []byte) error {
	if b.written == nil {
		b.written = map[string][]byte{}
	}
	b.written[path] = bytes.Clone(data)
	return nil
}
func (b *testPlayerBridge) Measure(text, font string) (float64, error) {
	if b.measure != nil {
		return b.measure(text, font)
	}
	return float64(len(script.UTF16Units(text)) * 7), nil
}
func playerEvent(events []any, kind string) map[string]any {
	for _, event := range events {
		if e, ok := event.(map[string]any); ok && e["type"] == kind {
			return e
		}
	}
	return nil
}
func TestPlayerDialogPauseOwnershipAndCancellation(t *testing.T) {
	p := NewPlayer(&testPlayerBridge{})
	defer p.Close()
	if err := p.Boot(PlayerConfig{}); err != nil {
		t.Fatal(err)
	}
	s := p.Host.Session
	p.Events()
	var result bool
	s.Track("question", false, func(*Task) error { var err error; result, err = s.OnQuestionDialog("Continue?"); return err })
	p.pump(false)
	event := playerEvent(p.Events(), "dialog")
	if event == nil || event["kind"] != "question" || !s.Clock.Frozen() {
		t.Fatal("question did not freeze game")
	}
	p.Command(PlayerCommand{Action: "pause", On: true})
	p.Command(PlayerCommand{Action: "reply", ID: event["id"].(uint64), Value: false})
	if result || len(p.dialogs) != 0 || s.Clock.Frozen() {
		t.Fatal("dialog did not resolve and release its own freeze")
	}
	before := p.now
	if err := p.Tick(100); err != nil {
		t.Fatal(err)
	}
	if p.now != before {
		t.Fatal("paused game clock advanced")
	}
	p.Command(PlayerCommand{Action: "pause", On: false})
	s.Clock.Freeze()
	s.Track("already frozen dialog", false, func(*Task) error { return s.OnNoteDialog("Note") })
	p.pump(false)
	event = playerEvent(p.Events(), "dialog")
	if event == nil {
		t.Fatal("note missing")
	}
	p.Command(PlayerCommand{Action: "reply", ID: event["id"].(uint64), Value: true})
	if !s.Clock.Frozen() {
		t.Fatal("dialog thawed a freeze it did not own")
	}
	p.Command(PlayerCommand{Action: "reply", ID: event["id"].(uint64), Value: true})
	if len(p.dialogs) != 0 {
		t.Fatal("duplicate reply recreated dialog")
	}
}
func TestPlayerCorruptLoadDoesNotRestartOrWrite(t *testing.T) {
	bridge := &testPlayerBridge{read: func(string) ([]byte, error) { return []byte("corrupt"), nil }}
	p := NewPlayer(bridge)
	defer p.Close()
	p.Boot(PlayerConfig{})
	p.Events()
	for _, action := range []string{"load", "import"} {
		if err := p.Command(PlayerCommand{Action: action, Path: "bad.ti"}); err != nil {
			t.Fatal(err)
		}
		events := p.Events()
		if playerEvent(events, "error") == nil || playerEvent(events, "restart") != nil || len(bridge.written) != 0 {
			t.Fatal("invalid save changed game or storage", events)
		}
		if p.Host.Session.Clock.Frozen() || p.busy {
			t.Fatal("failed load left game frozen")
		}
	}
}
func TestPlayerFrameDeliveryAndCommandGates(t *testing.T) {
	p := NewPlayer(&testPlayerBridge{})
	defer p.Close()
	p.Boot(PlayerConfig{})
	p.Events()
	if p.Frame() != nil {
		t.Fatal("blank frame emitted before rendering")
	}
	p.Host.Screen().FrameValid = true
	p.Host.Screen().Blit(p.Context)
	if len(p.Frame()) != 512*384*4 || p.Frame() != nil {
		t.Fatal("frame delivery duplicated unchanged pixels")
	}
	p.Command(PlayerCommand{Action: "pause", On: true})
	p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: 20, Y: 30})
	if p.Host.Session.PointerDown {
		t.Fatal("paused input pressed pointer")
	}
	p.Command(PlayerCommand{Action: "pause", On: false})
	p.Command(PlayerCommand{Action: "pointer", Kind: "move", X: 20.25, Y: 30.5, Shift: true})
	if p.Host.Session.PointerX != 20.25 || p.Host.Session.PointerY != 30.5 || !p.Host.Session.ShiftDown {
		t.Fatal("fractional mouse or modifier was lost")
	}
	p.Command(PlayerCommand{Action: "gamma", Key: float64(0)})
	if p.Host.Director.Gamma.Generation == 0 {
		t.Fatal("numeric gamma command did not reach renderer")
	}
	p.Command(PlayerCommand{Action: "gamma", Key: float64(9)})
	if p.Host.Director.Gamma.Channels != [3]float64{.65, .65, .65} {
		t.Fatal("gamma reset failed")
	}
	if err := p.Tick(-1); err == nil {
		t.Fatal("negative frame duration accepted")
	}
}
func TestNativePlayerIntegration(t *testing.T) {
	files := ownedFiles(t)
	bridge := &testPlayerBridge{read: os.ReadFile}
	p := NewPlayer(bridge)
	defer p.Close()
	p.Now = func() time.Time { return time.Date(2026, 9, 13, 12, 34, 56, 0, time.UTC) }
	if err := p.Boot(PlayerConfig{Index: files.Index}); err != nil {
		t.Fatal(err)
	}
	audioEnds := map[uint64]float64{}
	events := []any{}
	collect := func() {
		next := p.Events()
		events = append(events, next...)
		for _, e := range next {
			if event, ok := e.(AudioEvent); ok {
				if event.Type == "audio_play" {
					p.TakeAudio(event.ID)
					if !event.Loop {
						audioEnds[event.ID] = p.now + float64(event.Samples)*1000/float64(event.Rate)
					}
				}
				if event.Type == "audio_stop" {
					delete(audioEnds, event.ID)
				}
			}
			if event, ok := e.(map[string]any); ok && event["type"] == "error" {
				t.Fatal(event["text"])
			}
		}
	}
	step := func() {
		for id, end := range audioEnds {
			if p.now >= end {
				p.Command(PlayerCommand{Action: "audio_done", ID: id})
				delete(audioEnds, id)
			}
		}
		if err := p.Tick(50); err != nil {
			t.Fatal(err)
		}
		collect()
	}
	until := func(label string, done func() bool) {
		t.Helper()
		for i := 0; i < 1200; i++ {
			if done() {
				return
			}
			step()
		}
		t.Fatal(label, "did not settle", p.State(), p.Host.Session.Pending())
	}
	collect()
	until("intro", func() bool { return p.Host.Director.Movies.Playing() })
	p.Command(PlayerCommand{Action: "key", Key: ".", Special: true})
	until("menu", func() bool { return p.State()["movie"] == "playmode.mov" })
	p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: 266, Y: 254})
	p.Command(PlayerCommand{Action: "pointer", Kind: "release", X: 266, Y: 254})
	until("date", func() bool { return p.State()["movie"] == "datebed.mov" })
	p.Command(PlayerCommand{Action: "key", Key: ".", Special: true})
	until("room", func() bool { return p.State()["set"] == "bedsit1" && !p.Host.Director.InputLocked() })
	s := p.Host.Session
	s.Interp.Globals.Set("player_sentinel", script.Str("saved correctly"))
	p.Command(PlayerCommand{Action: "save"})
	collect()
	event := playerEvent(events, "dialog")
	if event == nil || event["kind"] != "save" || event["value"] != "bedsit1 2026-09-13-12-34-56" || !s.Clock.Frozen() {
		t.Fatal("save dialog differs", event)
	}
	p.Command(PlayerCommand{Action: "reply", ID: event["id"].(uint64), Value: "native player test"})
	collect()
	data := bridge.written["save:native player test"]
	if len(data) == 0 || playerEvent(events, "saved") == nil || s.Clock.Frozen() {
		t.Fatal("save did not write or resume game")
	}
	before := s.CurrentViewName()
	p.Command(PlayerCommand{Action: "key", Key: "rightarrow"})
	until("navigation", func() bool { return s.CurrentViewName() != before && !p.Host.Director.InputLocked() })
	events = nil
	importPath := `C:\Exports\Native.TI`
	bridge.read = func(path string) ([]byte, error) {
		if path == importPath {
			return data, nil
		}
		return os.ReadFile(path)
	}
	p.Command(PlayerCommand{Action: "import", Path: importPath})
	collect()
	if event := playerEvent(events, "restart"); event == nil || event["save"] != importPath {
		t.Fatal("validated import did not request restart", events)
	}
	if !bytes.Equal(bridge.written["save:Native.TI"], data) {
		t.Fatal("imported save bytes or Windows basename changed")
	}
	// Boot a fresh runtime from the saved bytes, as the Godot restart event does.
	other := NewPlayer(bridge)
	defer other.Close()
	if err := other.Boot(PlayerConfig{Index: files.Index, Save: "save:Native.TI"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300 && other.Host.Director.InputLocked(); i++ {
		if err := other.Tick(50); err != nil {
			t.Fatal(err)
		}
	}
	value, _ := other.Host.Session.Interp.Globals.Get("player_sentinel")
	if value.String() != "saved correctly" || other.State()["view"] != before {
		t.Fatal("fresh runtime did not restore save", other.State(), value)
	}
	if errEvent := playerEvent(other.Events(), "error"); errEvent != nil {
		t.Fatal(errEvent)
	}
	t.Log(fmt.Sprintf("Native player commands, save dialog, import and restart passed (%d save bytes)", len(data)))
}
