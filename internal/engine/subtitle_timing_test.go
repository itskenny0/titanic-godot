package engine

import (
	"bytes"
	"strings"
	"testing"
)

func TestTimedSubtitlesFollowVoiceAndSkip(t *testing.T) {
	c, _ := puppetFixture(t)
	c.TimedSubtitles = true
	c.Executor.Start("speech", func(task *Task) error { c.Speak(task, "hello"); return nil })
	pumpPuppet(t, c, 0)
	from, to, alpha := c.subtitleReveal()
	if from != 0 || c.Puppet.Subtitle[:to] != "Hello" || alpha >= 1 {
		t.Fatal("first word should fade in", from, to, alpha)
	}
	pumpPuppet(t, c, 60)
	_, _, next := c.subtitleReveal()
	if next <= alpha {
		t.Fatal("fade did not advance")
	}
	pumpPuppet(t, c, 150)
	from, to, alpha = c.subtitleReveal()
	if c.Puppet.Subtitle[:from] != "Hello" || c.Puppet.Subtitle[:to] != "Hello there" || alpha >= 1 {
		t.Fatal("second word should fade over voice duration")
	}
	c.TimedSubtitles = false
	from, to, alpha = c.subtitleReveal()
	if from != len(c.Puppet.Subtitle) || from != to || alpha != 1 {
		t.Fatal("disabling timing must reveal full line immediately")
	}
	c.TimedSubtitles = true
	c.Key(".", true)
	pumpPuppet(t, c, 151)
	if c.Puppet.Subtitle != "" || c.Puppet.subtitleTiming != nil {
		t.Fatal("skipping retained subtitle animation")
	}
}

func TestSubtitleTimingMissingAudioUnicodeAndTextChanges(t *testing.T) {
	c, _ := puppetFixture(t)
	c.TimedSubtitles = true
	p := c.Puppet
	p.Pup.File.Containers[1].Data = nil
	c.Executor.Start("speech", func(task *Task) error { c.Speak(task, "hello"); return nil })
	pumpPuppet(t, c, 0)
	from, _, alpha := c.subtitleReveal()
	if from != len(p.Subtitle) || alpha != 1 {
		t.Fatal("missing audio should show complete subtitle")
	}
	p.Subtitle = "Ça va,\u2003capitaine?"
	p.subtitleTiming = newSubtitleTiming(p.Subtitle, 0, 2000)
	_, end, _ := c.subtitleReveal()
	if p.Subtitle[:end] != "Ça" {
		t.Fatal("Unicode word split", p.Subtitle[:end])
	}
	c.Executor.AdvanceClock(2000)
	_, end, alpha = c.subtitleReveal()
	if end != len(p.Subtitle) || alpha != 1 {
		t.Fatal("last word not fully visible by voice end")
	}
	p.Subtitle = "Replacement text"
	from, _, _ = c.subtitleReveal()
	if from != len(p.Subtitle) {
		t.Fatal("stale timing applied to new text")
	}
}

func TestSubtitleRevealInvalidatesOverlayAndPreservesWrapping(t *testing.T) {
	v, p := puppetViewFixture(t)
	c := v.Session.PuppetCtrl
	c.TimedSubtitles = true
	p.Subtitle = strings.Repeat("Welcome aboard ", 6)
	p.subtitleTiming = newSubtitleTiming(p.Subtitle, 0, 5000)
	ctx := NewDrawContext(512, 384)
	ctx.Measure = func(text, font string) float64 { return float64(len(text) * 7) }
	var first, second DrawSignature
	v.DrawSignature(&first)
	c.Executor.AdvanceClock(60)
	v.DrawSignature(&second)
	if first == second {
		t.Fatal("word animation did not invalidate static pose")
	}
	v.DrawOverlay(ctx)
	// The final layout, including line breaks, is computed from complete text.
	lines := puppetSubtitleLines(p.Subtitle, ctx.MeasureText)
	if len(lines) != 2 {
		t.Fatal("test subtitle did not wrap")
	}
	for _, command := range ctx.Commands {
		if command.Op == "text" && (command.Y != 240 || command.X != 8) {
			t.Fatal("early word moved from its final line position", command)
		}
	}
	c.Executor.AdvanceClock(5000)
	ctx.Commands = nil
	v.DrawOverlay(ctx)
	visible := map[float64]string{}
	for _, command := range ctx.Commands {
		if command.Op == "text" {
			visible[command.Y] += command.Text
		}
	}
	if visible[240] != lines[0] || visible[256] != lines[1] {
		t.Fatal("revealing text changed line wrapping", visible)
	}
}

func TestWideDialogueRetainsCharacterBehindSubtitles(t *testing.T) {
	v, p := puppetViewFixture(t)
	p.Subtitle = "Hello"
	classic := make([]byte, 512*384*4)
	wide := make([]byte, len(classic))
	if err := v.Composite(classic, nil); err != nil {
		t.Fatal(err)
	}
	v.Wide = true
	if err := v.Composite(wide, nil); err != nil {
		t.Fatal(err)
	}
	at := (224*512 + 1) * 4
	if bytes.Equal(wide[at:at+4], classic[at:at+4]) || wide[at] != v.Gamma.DisplayChannel(100, 0) {
		t.Fatal("wide subtitle matte still erases character")
	}
	if v.BevelAt(256, 276) != -1 {
		t.Fatal("invented an answer where none exists")
	}
	p.Bevels = []Bevel{{Text: "Answer", ID: 1}}
	if v.BevelAt(256, 276) != 0 {
		t.Fatal("wide mode changed authored answer coordinates")
	}
}

func TestDialoguePreferencesPauseAndFallback(t *testing.T) {
	p := adaptiveTestPlayer(t)
	s, d := p.Host.Session, p.Host.Director
	s.PuppetCtrl.Puppet = &PuppetState{Visible: true, Subtitle: "Hello there", Bevels: make([]Bevel, 5)}
	s.PuppetCtrl.Puppet.subtitleTiming = newSubtitleTiming("Hello there", s.Executor.Now(), 1000)
	if p.DialogueLayout().Eligible {
		t.Fatal("widescreen must be opt-in")
	}
	p.Command(PlayerCommand{Action: "pause", On: true})
	p.Command(PlayerCommand{Action: "wide_dialogue", On: true})
	p.Command(PlayerCommand{Action: "timed_subtitles", On: true})
	before, _, alpha := s.PuppetCtrl.subtitleReveal()
	if err := p.Tick(5000); err != nil {
		t.Fatal(err)
	}
	after, _, nextAlpha := s.PuppetCtrl.subtitleReveal()
	if before != after || alpha != nextAlpha {
		t.Fatal("pause advanced word animation")
	}
	if !d.PuppetView.Wide || !s.PuppetCtrl.TimedSubtitles {
		t.Fatal("preferences must work while menu pauses gameplay")
	}
	if got := p.DialogueLayout(); !got.Eligible || got.Choices != 5 {
		t.Fatal("dialogue metadata", got)
	}
	s.Fade.Level = .5
	if !p.DialogueLayout().Eligible {
		t.Fatal("in-place fade should not abruptly resize conversation")
	}
	s.Fade.PendingReveal = true
	if p.DialogueLayout().Eligible {
		t.Fatal("held frame must keep its original composition")
	}
	s.Fade.PendingReveal = false
	s.PuppetCtrl.Puppet.Visible = false
	if p.DialogueLayout().Eligible {
		t.Fatal("hidden puppet must not enable dialogue layout")
	}
}
