package engine

import (
	"encoding/binary"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"reflect"
	"testing"
)

func puppetFixture(t *testing.T) (*PuppetController, *HostAudio) {
	t.Helper()
	e := NewExecutor()
	t.Cleanup(e.Close)
	audio := new(HostAudio)
	d := NewScriptDispatch(script.NewInterpreter(), new(PropRuntime), new(ActorRuntime))
	c := NewPuppetController(d, e, audio, nil)
	sound := make([]byte, 50)
	binary.LittleEndian.PutUint32(sound, 0x10000)
	binary.LittleEndian.PutUint16(sound[26:], 1)
	binary.LittleEndian.PutUint32(sound[28:], 10)
	binary.LittleEndian.PutUint32(sound[36:], 2)
	binary.LittleEndian.PutUint32(sound[44:], 48)
	anim := make([]byte, 82*3)
	for i := 0; i < 3; i++ {
		for layer := 0; layer < 11; layer++ {
			at := i*82 + 16 + layer*6
			binary.LittleEndian.PutUint16(anim[at:], uint16(i))
			binary.LittleEndian.PutUint16(anim[at+2:], uint16(i+layer))
			binary.LittleEndian.PutUint16(anim[at+4:], uint16(i+layer*2))
		}
	}
	pup := &df.Puppet{File: &df.File{Containers: []df.Container{{}, {Data: sound}, {Data: anim}}}, Dialogue: map[string]df.PuppetDialogue{}, DialogueOrder: []string{"hello", "reply", "idle 1"}, IdleTimers: []df.IdleTimer{{MinTicks: 3, MaxTicks: 3}}}
	for _, line := range []df.PuppetDialogue{{Ident: "hello", Text: "Hello there", Raw: "Hello there", Stance: 1, AudioLocation: 1, AnimLogicLocation: 2}, {Ident: "reply", Text: "Goodbye", Raw: "Goodbye", Stance: 2, AudioLocation: 1, AnimLogicLocation: 2}, {Ident: "idle 1", Text: "Idle instruction", Raw: "Idle instruction", Stance: 0, AudioLocation: 1, AnimLogicLocation: 2}} {
		pup.Dialogue[line.Ident] = line
	}
	c.open("test.pup", pup)
	return c, audio
}
func pumpPuppet(t *testing.T, c *PuppetController, now float64) {
	t.Helper()
	for _, done := range c.Executor.Pump(now, true, 1000) {
		if done.Err != nil {
			t.Fatal(done.Err)
		}
	}
}
func TestPuppetSpeechTimingAndSkipping(t *testing.T) {
	c, audio := puppetFixture(t)
	p := c.Puppet
	done := false
	c.Executor.Start("speech", func(task *Task) error { c.Speak(task, " hello THERE "); done = true; return nil })
	pumpPuppet(t, c, 0)
	if done || p.Subtitle != "Hello there" || p.StanceIdx != 1 || p.Anim == nil || len(p.VoiceQueue) != 1 {
		t.Fatal("speech did not start by text lookup")
	}
	if c.Frame().Layers[0].Frame != 0 {
		t.Fatal("initial speech pose differs")
	}
	pumpPuppet(t, c, 50)
	if c.Frame().Layers[0].Frame != 1 {
		t.Fatal("animation did not follow the 33.3 ms line clock")
	}
	pumpPuppet(t, c, 349)
	if done {
		t.Fatal("speech ended before its grace period")
	}
	pumpPuppet(t, c, 350)
	if !done || p.Subtitle != "" || p.Anim != nil || p.Pose.Layers[0].Frame != 2 {
		t.Fatal("speech did not hold its final pose")
	}
	events := audio.DrainEvents()
	if len(events) != 2 || events[0].Type != "audio_play" || events[1].Type != "audio_stop" {
		t.Fatal("voice playback lifecycle differs", events)
	}
	c.Executor.Start("skip", func(task *Task) error {
		c.Speak(task, "hello")
		c.Speak(task, "reply")
		c.Speak(task, "hello")
		return nil
	})
	pumpPuppet(t, c, 400)
	volume := -1.
	c.SetWaveVolume = func(n float64) float64 { volume = n; return n }
	if !c.Key("7", false) || volume != 7 || c.Key(".", false) {
		t.Fatal("conversation volume or special-key handling differs")
	}
	if !c.Key(".", true) {
		t.Fatal("skip key was not accepted")
	}
	pumpPuppet(t, c, 401)
	if !p.Interrupted || p.SpeakSkip != nil || p.Subtitle != "" || len(p.VoiceQueue) != 3 {
		t.Fatal("skip did not suppress subsequent lines or bound repeat history")
	}
	events = audio.DrainEvents()
	if len(events) != 2 || events[0].Type != "audio_play" {
		t.Fatal("skipped speech started another voice", events)
	}
	c.Base("")
	if p.Pose != p.DefaultPose || p.StanceIdx != p.DefaultStance {
		t.Fatal("default puppet base was lost")
	}
	c.Base("REPLY")
	if p.StanceIdx != 2 || p.Pose.Layers[0].Frame != 0 {
		t.Fatal("base line did not change stance and pose")
	}
}
func TestPuppetChoicesAndClosing(t *testing.T) {
	c, _ := puppetFixture(t)
	for i := 0; i < 6; i++ {
		c.Bevel("choice", float64(40+i))
	}
	if len(c.Puppet.Bevels) != 5 {
		t.Fatal("choice capacity differs")
	}
	answer := -99.
	c.Executor.Start("choice", func(task *Task) error { answer = c.Event(task); return nil })
	pumpPuppet(t, c, 100)
	c.Press(nil, 1, false)
	if c.Puppet.Press == nil || c.Puppet.Press.Until != 266.66666666666663 {
		t.Fatal("press highlight duration differs")
	}
	c.Release(2)
	pumpPuppet(t, c, 101)
	if answer != -99 || c.Puppet.Press != nil {
		t.Fatal("release on another row answered the question")
	}
	c.Choose(nil, 2, false)
	pumpPuppet(t, c, 102)
	p := c.Puppet
	if answer != 42 || p.Chosen == nil || *p.Chosen != 2 || p.LastPlaque == nil || p.LastPlaque.Chosen == nil || len(p.LastPlaque.Bevels) != 5 {
		t.Fatal("choice or repeat plaque was not retained")
	}
	c.Clear()
	c.Bevel("continue", 1)
	c.Executor.Start("closed choice", func(task *Task) error { answer = c.Event(task); return nil })
	pumpPuppet(t, c, 200)
	c.ClosePuppetFile()
	pumpPuppet(t, c, 250)
	if answer != -1 || c.Puppet != nil || c.Executor.Pending() != 0 {
		t.Fatal("closing conversation stranded an event waiter")
	}
}
func TestPuppetIdleSkipEndsConversation(t *testing.T) {
	c, _ := puppetFixture(t)
	c.Param = func(i int) float64 {
		if i == 8 {
			return 1
		}
		return 0
	}
	c.Bevel("continue", 1)
	answer := -99.
	c.Executor.Start("idle choice", func(task *Task) error { answer = c.Event(task); return nil })
	pumpPuppet(t, c, 1000)
	pumpPuppet(t, c, 1050)
	if c.Puppet.SpeakSkip == nil || c.Puppet.Anim == nil || c.Puppet.Subtitle != "" {
		t.Fatal("idle speech did not start or displayed an instruction subtitle")
	}
	c.SkipLine()
	pumpPuppet(t, c, 1051)
	if answer != -1 || c.Puppet.EventWaiter != nil || c.Puppet.SpeakSkip != nil || c.Executor.Pending() != 0 {
		t.Fatal("skipping idle speech did not exit the choice wait")
	}
}
func TestPuppetRepeatRestoresCurrentChoices(t *testing.T) {
	c, _ := puppetFixture(t)
	c.Bevel("Hello there", 42)
	c.Executor.Start("first choice", func(task *Task) error { c.Event(task); return nil })
	pumpPuppet(t, c, 0)
	c.Choose(nil, 0, false)
	pumpPuppet(t, c, 1)
	c.Executor.Start("reply", func(task *Task) error { c.Speak(task, "reply"); return nil })
	pumpPuppet(t, c, 50)
	pumpPuppet(t, c, 400)
	c.Clear()
	c.Bevel("Current question", 99)
	c.Executor.Start("next choice", func(task *Task) error { c.Event(task); return nil })
	pumpPuppet(t, c, 450)
	c.Press(nil, -1, true)
	pumpPuppet(t, c, 451)
	if !c.Puppet.Repeating || c.Puppet.Subtitle != "Hello there" || c.Puppet.Bevels[0].ID != 42 {
		t.Fatal("repeat did not replay the selected question")
	}
	pumpPuppet(t, c, 801)
	if c.Puppet.Subtitle != "Goodbye" {
		t.Fatal("repeat omitted the queued reply")
	}
	pumpPuppet(t, c, 1151)
	if c.Puppet.Repeating || c.Puppet.Subtitle != "" || c.Puppet.Bevels[0].ID != 99 || c.Puppet.Chosen != nil {
		t.Fatal("repeat did not restore current choices")
	}
	c.ClosePuppetFile()
	pumpPuppet(t, c, 1201)
}
func TestSubtitleRules(t *testing.T) {
	for _, v := range []struct {
		raw, ident string
		want       bool
	}{{"", "hello", false}, {"   ", "hello", false}, {"*command", "hello", false}, {"stage direction", "IDLE 4", false}, {"Hello", "line", true}, {"\t", "line", true}} {
		if Subtitled(df.PuppetDialogue{Raw: v.raw, Ident: v.ident}) != v.want {
			t.Fatal(v)
		}
	}
}
func TestPuppetCancellation(t *testing.T) {
	c, audio := puppetFixture(t)
	c.Executor.Start("speech", func(task *Task) error { c.Speak(task, "hello"); return nil })
	pumpPuppet(t, c, 0)
	c.Executor.CancelAll()
	if c.Puppet.SpeakSkip != nil || c.Puppet.Anim != nil || c.Puppet.Subtitle != "" || !audio.IsDone(VoiceChannel) {
		t.Fatal("cancelled speech retained a waiter or voice")
	}
}
func TestPuppetScrambleDeterminism(t *testing.T) {
	c, _ := puppetFixture(t)
	for i := 0; i < 5; i++ {
		c.Bevel("choice", float64(i))
	}
	c.Random = SeededRandom(12345)
	c.Scramble()
	ids := []float64{}
	for _, b := range c.Puppet.Bevels {
		ids = append(ids, b.ID)
	}
	if !reflect.DeepEqual(ids, []float64{1, 2, 4, 3, 0}) {
		t.Fatal("scramble differs", ids)
	}
}
