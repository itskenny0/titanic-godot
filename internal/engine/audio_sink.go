package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"math"
	"strconv"
)

type AudioChannel string

const (
	SoundChannel AudioChannel = "sound"
	VoiceChannel AudioChannel = "voice"
	ThemeChannel AudioChannel = "theme"
)

type PlayOptions struct {
	Loop, Overlap bool
	Volume        *float64
	Pan           float64
}
type PlayHandle interface {
	Done() bool
	Stop()
}
type AudioSink interface {
	Play(AudioChannel, *df.Audio, PlayOptions) PlayHandle
	Halt(AudioChannel)
	IsDone(AudioChannel) bool
	SetChannelVolume(AudioChannel, float64)
	SetSuspended(bool)
}

func playVolume(opts PlayOptions) float64 {
	if opts.Volume == nil {
		return 1
	}
	return *opts.Volume
}

type AudioEvent struct {
	Type    string       `json:"type"`
	ID      uint64       `json:"id"`
	Channel AudioChannel `json:"channel"`
	Rate    int          `json:"rate"`
	Samples int          `json:"samples"`
	Loop    bool         `json:"loop"`
	Volume  float64      `json:"volume"`
	On      bool         `json:"on"`
}
type hostPlay struct {
	id            uint64
	channel       AudioChannel
	overlap, done bool
	host          *HostAudio
}

func (h *hostPlay) Done() bool { return h.done }
func (h *hostPlay) Stop() {
	h.done = true
	h.host.entries.Delete(strconv.FormatUint(h.id, 10))
	h.host.events = append(h.host.events, AudioEvent{Type: "audio_stop", ID: h.id})
}

// HostAudio transfers PCM and playback commands to Godot. Only audio_done from
// the actual audio player completes a handle; wall-clock guesses cannot unblock
// speech while a device is paused or its audio thread is still playing.
type HostAudio struct {
	nextID  uint64
	entries orderedMap[*hostPlay]
	buffers map[uint64][]byte
	events  []AudioEvent
}

func (a *HostAudio) Play(channel AudioChannel, audio *df.Audio, opts PlayOptions) PlayHandle {
	if !opts.Overlap {
		a.Halt(channel)
	}
	a.nextID++
	h := &hostPlay{id: a.nextID, channel: channel, overlap: opts.Overlap, host: a}
	a.entries.Set(strconv.FormatUint(h.id, 10), h)
	if a.buffers == nil {
		a.buffers = map[uint64][]byte{}
	}
	pan := opts.Pan
	if math.IsNaN(pan) {
		pan = 0
	}
	a.buffers[h.id] = audio.StereoPCM(playVolume(opts), pan)
	a.events = append(a.events, AudioEvent{Type: "audio_play", ID: h.id, Channel: channel, Rate: audio.SampleRate, Samples: len(audio.Samples), Loop: opts.Loop})
	return h
}
func (a *HostAudio) Halt(channel AudioChannel) {
	list := []*hostPlay{}
	for _, h := range a.entries.All() {
		if h.channel == channel && !h.overlap {
			list = append(list, h)
		}
	}
	for _, h := range list {
		h.Stop()
	}
}
func (a *HostAudio) IsDone(channel AudioChannel) bool {
	for _, h := range a.entries.All() {
		if h.channel == channel && !h.overlap && !h.done {
			return false
		}
	}
	return true
}
func (a *HostAudio) SetChannelVolume(channel AudioChannel, v float64) {
	a.events = append(a.events, AudioEvent{Type: "audio_volume", Channel: channel, Volume: v})
}
func (a *HostAudio) SetSuspended(on bool) {
	a.events = append(a.events, AudioEvent{Type: "audio_pause", On: on})
}
func (a *HostAudio) Finish(id uint64) {
	key := strconv.FormatUint(id, 10)
	if h := a.entries.Get(key); h != nil {
		h.done = true
		a.entries.Delete(key)
	}
}
func (a *HostAudio) TakePCM(id uint64) []byte  { b := a.buffers[id]; delete(a.buffers, id); return b }
func (a *HostAudio) DrainEvents() []AudioEvent { events := a.events; a.events = nil; return events }

type deferredPlay struct {
	audio   *df.Audio
	opts    PlayOptions
	real    PlayHandle
	stopped bool
}

func (h *deferredPlay) Done() bool { return h.stopped || h.real != nil && h.real.Done() }
func (h *deferredPlay) Stop() {
	h.stopped = true
	if h.real != nil {
		h.real.Stop()
	}
}

type completedPlay struct{}

func (completedPlay) Done() bool { return true }
func (completedPlay) Stop()      {}

type DeferredAudioSink struct {
	real      AudioSink
	volumes   orderedMap[float64]
	held      orderedMap[*deferredPlay]
	suspended bool
}

func (a *DeferredAudioSink) Attached() bool { return a.real != nil }
func (a *DeferredAudioSink) Attach(sink AudioSink) {
	a.real = sink
	if a.suspended {
		sink.SetSuspended(true)
	}
	for c, v := range a.volumes.All() {
		sink.SetChannelVolume(AudioChannel(c), v)
	}
	a.volumes = orderedMap[float64]{}
	for c, h := range a.held.All() {
		if !h.stopped {
			h.real = sink.Play(AudioChannel(c), h.audio, h.opts)
		}
	}
	a.held = orderedMap[*deferredPlay]{}
}
func (a *DeferredAudioSink) Play(channel AudioChannel, audio *df.Audio, opts PlayOptions) PlayHandle {
	if a.real != nil {
		return a.real.Play(channel, audio, opts)
	}
	if !opts.Loop {
		return completedPlay{}
	}
	h := &deferredPlay{audio: audio, opts: opts}
	a.held.Set(string(channel), h)
	return h
}
func (a *DeferredAudioSink) Halt(channel AudioChannel) {
	key := string(channel)
	if h := a.held.Get(key); h != nil {
		h.stopped = true
	}
	a.held.Delete(key)
	if a.real != nil {
		a.real.Halt(channel)
	}
}
func (a *DeferredAudioSink) IsDone(channel AudioChannel) bool {
	if a.real != nil {
		return a.real.IsDone(channel)
	}
	return !a.held.Has(string(channel))
}
func (a *DeferredAudioSink) SetChannelVolume(c AudioChannel, v float64) {
	if a.real != nil {
		a.real.SetChannelVolume(c, v)
	} else {
		a.volumes.Set(string(c), v)
	}
}
func (a *DeferredAudioSink) SetSuspended(on bool) {
	a.suspended = on
	if a.real != nil {
		a.real.SetSuspended(on)
	}
}
