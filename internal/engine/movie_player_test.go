package engine

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func movieFixture() *df.Movie {
	sound := make([]byte, 50)
	binary.LittleEndian.PutUint32(sound, 0x10000)
	binary.LittleEndian.PutUint16(sound[26:], 1)
	binary.LittleEndian.PutUint32(sound[28:], 10)
	binary.LittleEndian.PutUint32(sound[36:], 2)
	binary.LittleEndian.PutUint32(sound[44:], 48)
	file := &df.File{Order: binary.LittleEndian, Containers: []df.Container{{}, {Data: []byte{1, 0, 2, 0, 4, 1, 2}}, {Data: []byte{1, 0, 2, 0, 4, 3, 4}}, {Data: []byte{1, 0, 2, 0, 4, 5, 6}}, {Data: sound}}}
	return &df.Movie{Segments: []df.MovieSegment{{File: file, PaletteRaw: make([]byte, 2048), MinHoldTicks: 3, KeySkips: true, OriginX: 10, OriginY: 20, ActionFrame1: "third", Sounds: map[string]int{"voice": 4}, SoundFollows: map[string]string{}, Frames: []df.MovieFrame{{Name: "first", LocationFrame: 1, Type: 6}, {Name: "second", LocationFrame: 2, Type: 6}, {Name: "third", LocationFrame: 3, Type: 1}}}}}
}
func newMovieTest(t *testing.T) (*MoviePlayer, *HostAudio) {
	t.Helper()
	audio := new(HostAudio)
	s := NewSession(func(string) ([]byte, error) { return nil, nil }, audio)
	t.Cleanup(s.Close)
	return NewMoviePlayer(s, nil, nil), audio
}
func startMovieTest(t *testing.T, p *MoviePlayer, m *df.Movie) {
	t.Helper()
	if ok, err := p.enterSegment(m, "test.mov", 0, 0); err != nil || !ok {
		t.Fatal("movie failed to start", err)
	}
}
func tickMovieTest(t *testing.T, p *MoviePlayer, now float64) {
	t.Helper()
	if _, err := p.Tick(now); err != nil {
		t.Fatal(err)
	}
}
func TestMovieVoiceWaitAndRegionCoordinates(t *testing.T) {
	p, audio := newMovieTest(t)
	m := movieFixture()
	seg := &m.Segments[0]
	seg.Frames[0].Sound = "voice"
	seg.Frames[1].WaitsForVoice = true
	seg.Frames[2].Regions = []df.MovieRegion{{Type: 2, X0: 8, Y0: 6, X1: 0, Y1: 0, Target: "FIRST", Sound: "voice"}}
	startMovieTest(t, p, m)
	tickMovieTest(t, p, 100)
	tickMovieTest(t, p, 150)
	if p.FramePos() != 1 {
		t.Fatal("movie did not advance after its authored hold")
	}
	tickMovieTest(t, p, 10000)
	if p.FramePos() != 1 {
		t.Fatal("timer completed speech without audio acknowledgement")
	}
	for _, e := range audio.DrainEvents() {
		if e.Type == "audio_play" {
			audio.Finish(e.ID)
		}
	}
	tickMovieTest(t, p, 10001)
	if p.FramePos() != 2 || !p.Session.MovieActions[1] {
		t.Fatal("voice completion did not enter action frame")
	}
	tickMovieTest(t, p, 20000)
	if p.FramePos() != 2 || p.ClickableAt(9.99, 20) || !p.ClickableAt(18, 26) {
		t.Fatal("interactive wait or inclusive origin-relative bounds differ")
	}
	if err := p.Click(14, 23); err != nil {
		t.Fatal(err)
	}
	events := audio.DrainEvents()
	plays := 0
	for _, e := range events {
		if e.Type == "audio_play" {
			plays++
		}
	}
	if p.FramePos() != 0 || plays != 1 {
		t.Fatal("click and destination played the same sound twice", plays)
	}
	if ok, err := p.Key("7", false); err != nil || !ok || p.Session.WaveVolume != 7 {
		t.Fatal("movie volume key failed")
	}
	if ok, _ := p.Key(".", false); ok {
		t.Fatal("ordinary period skipped movie")
	}
	if ok, err := p.Key(".", true); err != nil || !ok || p.Playing() || !p.Session.Fade.PendingReveal {
		t.Fatal("special skip failed to reveal room")
	}
}
func TestMovieCuesAndSoundFollow(t *testing.T) {
	p, audio := newMovieTest(t)
	m := movieFixture()
	seg := &m.Segments[0]
	seg.Frames[0].Regions = []df.MovieRegion{{Type: 1, X1: 2, Y1: 2}}
	seg.Cues = []df.MovieCue{{Tick: 6, Target: "second"}}
	seg.Frames[1].Regions = seg.Frames[0].Regions
	seg.Frames[1].Sound = "voice"
	seg.SoundFollows["voice"] = "third"
	startMovieTest(t, p, m)
	tickMovieTest(t, p, 100)
	tickMovieTest(t, p, 199)
	if p.FramePos() != 0 {
		t.Fatal("cue fired early")
	}
	tickMovieTest(t, p, 200)
	if p.FramePos() != 1 {
		t.Fatal("cue did not release held frame")
	}
	tickMovieTest(t, p, 1000)
	if p.FramePos() != 1 {
		t.Fatal("sound-follow jumped before host completion")
	}
	for _, e := range audio.DrainEvents() {
		if e.Type == "audio_play" {
			audio.Finish(e.ID)
		}
	}
	tickMovieTest(t, p, 1001)
	if p.FramePos() != 2 {
		t.Fatal("sound-follow target not entered")
	}
	tickMovieTest(t, p, 1051)
	if p.Playing() {
		t.Fatal("movie did not finish after final hold")
	}
}
func TestMovieChainReturnsAndSequenceCompletion(t *testing.T) {
	p, _ := newMovieTest(t)
	outer, inner := movieFixture(), movieFixture()
	outer.Segments[0].Frames[0].Type = 4
	outer.Segments[0].Frames[0].Event = "inner.mov"
	outer.Segments[0].Frames[0].Target = "third"
	inner.Segments[0].Frames[0].Type = 5
	p.Session.OnPlayMovie = func(name string, start *int) error {
		movie := inner
		if name == "test.mov" {
			movie = outer
		}
		_, err := p.enterSegment(movie, name, 0, *start)
		return err
	}
	startMovieTest(t, p, outer)
	seq := &movieSequence{}
	p.sequence = seq
	done := false
	p.Session.Track("waiting script", false, func(task *Task) error { task.Wait(func() bool { return seq.done }); done = true; return nil })
	p.Session.Pump(0, false, 1000)
	tickMovieTest(t, p, 100)
	tickMovieTest(t, p, 150)
	p.Session.Pump(150, true, 1000)
	if p.PlayingFile() != "inner.mov" || done || len(p.callStack) != 1 {
		t.Fatal("nested movie lost parent wait or return frame")
	}
	tickMovieTest(t, p, 200)
	tickMovieTest(t, p, 250)
	p.Session.Pump(250, true, 1000)
	if p.PlayingFile() != "test.mov" || p.FramePos() != 2 || done || len(p.callStack) != 0 {
		t.Fatal("nested movie did not return to named frame")
	}
	tickMovieTest(t, p, 300)
	tickMovieTest(t, p, 350)
	p.Session.Pump(350, true, 1000)
	if p.Playing() || !done {
		t.Fatal("movie sequence did not release its script")
	}
}
func TestMovieFinishPreservesAbortInputAndReleasesFiles(t *testing.T) {
	for _, flags := range []int{0, df.MovieAnyInputAborts} {
		p, _ := newMovieTest(t)
		m := movieFixture()
		m.Segments[0].Flags = flags
		startMovieTest(t, p, m)
		p.played.Set("test.mov", true)
		p.played.Set("inner.mov", true)
		p.played.Set("test.mov", true)
		var released []string
		p.Session.OnMoviesDone = func(names []string) { released = names }
		p.Session.Events.Post(QueuedEvent{Kind: "keydown", Key: "x"}, false)
		p.Abandon()
		if (p.Session.Events.Len() > 0) != (flags != 0) {
			t.Fatal("abort event policy changed", flags)
		}
		if !reflect.DeepEqual(released, []string{"test.mov", "inner.mov"}) {
			t.Fatal("movie resource release order differs", released)
		}
	}
}
