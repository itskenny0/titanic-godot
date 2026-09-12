package engine

import (
	"encoding/binary"
	"github.com/itskenny0/titanic-godot/internal/df"
	"testing"
)

func TestHostAudioCompletionAndOverlap(t *testing.T) {
	audio := new(HostAudio)
	sound := &df.Audio{SampleRate: 22050, Samples: []float32{1, -1, .5}}
	volume := .5
	first := audio.Play(VoiceChannel, sound, PlayOptions{Pan: 1, Volume: &volume})
	second := audio.Play(VoiceChannel, sound, PlayOptions{})
	overlap := audio.Play(VoiceChannel, sound, PlayOptions{Overlap: true})
	events := audio.DrainEvents()
	if len(events) != 4 || events[0].Type != "audio_play" || events[1].Type != "audio_stop" || events[2].Type != "audio_play" || events[3].Type != "audio_play" {
		t.Fatal("playback replacement order differs", events)
	}
	if !first.Done() || second.Done() || overlap.Done() || audio.IsDone(VoiceChannel) {
		t.Fatal("playback handles have incorrect state")
	}
	pcm := audio.TakePCM(events[0].ID)
	if len(pcm) != 12 || binary.LittleEndian.Uint16(pcm) != 0 || int16(binary.LittleEndian.Uint16(pcm[2:])) != 16384 || int16(binary.LittleEndian.Uint16(pcm[6:])) != -16383 {
		t.Fatal("stereo PCM gain, pan, or negative rounding differs")
	}
	if audio.TakePCM(events[0].ID) != nil {
		t.Fatal("PCM transfer retained an extra copy")
	}
	audio.Finish(events[0].ID)
	if second.Done() {
		t.Fatal("stale completion ended the new voice")
	}
	audio.SetSuspended(true)
	audio.SetSuspended(false)
	if second.Done() {
		t.Fatal("pause completed a pending voice")
	}
	audio.Halt(VoiceChannel)
	if !second.Done() || overlap.Done() || !audio.IsDone(VoiceChannel) {
		t.Fatal("halt must leave overlapping audio independent")
	}
	overlap.Stop()
	if !overlap.Done() {
		t.Fatal("overlap stop did not complete handle")
	}
}
func TestAudioWaitResumesOnPlayerAcknowledgement(t *testing.T) {
	executor := NewExecutor()
	defer executor.Close()
	audio := new(HostAudio)
	sound := &df.Audio{SampleRate: 10, Samples: make([]float32, 1)}
	handle := audio.Play(VoiceChannel, sound, PlayOptions{})
	id := audio.DrainEvents()[0].ID
	resumed := false
	executor.Start("speech", func(task *Task) error { task.Wait(handle.Done); resumed = true; return nil })
	executor.Pump(0, true, 10)
	executor.Pump(10000, true, 10)
	if resumed {
		t.Fatal("voice wait guessed completion from elapsed time")
	}
	audio.Finish(id)
	executor.Pump(10001, true, 10)
	if !resumed {
		t.Fatal("player acknowledgement did not release speech wait")
	}
}
func TestDeferredAudioAttach(t *testing.T) {
	deferred := new(DeferredAudioSink)
	audio := new(HostAudio)
	sound := &df.Audio{SampleRate: 10, Samples: []float32{1}}
	dropped := deferred.Play(SoundChannel, sound, PlayOptions{})
	held := deferred.Play(ThemeChannel, sound, PlayOptions{Loop: true})
	cancelled := deferred.Play(VoiceChannel, sound, PlayOptions{Loop: true})
	deferred.Halt(VoiceChannel)
	deferred.SetChannelVolume(ThemeChannel, .6)
	deferred.SetSuspended(true)
	if !dropped.Done() || held.Done() || !cancelled.Done() || deferred.Attached() {
		t.Fatal("deferred playback state differs")
	}
	deferred.Attach(audio)
	events := audio.DrainEvents()
	if len(events) != 3 || events[0].Type != "audio_pause" || events[1].Type != "audio_volume" || events[2].Type != "audio_play" || events[2].Channel != ThemeChannel || !events[2].Loop {
		t.Fatal("attach did not restore pause, volume, then held theme", events)
	}
	held.Stop()
	if !held.Done() || !audio.IsDone(ThemeChannel) {
		t.Fatal("deferred handle lost control of attached playback")
	}
}
