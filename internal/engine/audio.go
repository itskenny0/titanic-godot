package engine

import (
	"slices"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
)

type audioBankEntry struct {
	file *df.File
	bank *df.AudioBank
}
type AudioLibrary struct {
	banks map[string]audioBankEntry
	order []string
	cache map[string]*df.Audio
}

func NewAudioLibrary() *AudioLibrary {
	return &AudioLibrary{banks: map[string]audioBankEntry{}, order: []string{}, cache: map[string]*df.Audio{}}
}
func (l *AudioLibrary) OpenBank(name string, data []byte) error {
	key := strings.ToLower(name)
	if _, ok := l.banks[key]; ok {
		return nil
	}
	file, err := df.ReadFile(data)
	if err != nil {
		return err
	}
	bank, err := df.ReadAudioBankFrom(file)
	if err != nil {
		return err
	}
	l.banks[key] = audioBankEntry{file, bank}
	l.order = append(l.order, key)
	return nil
}
func (l *AudioLibrary) find(name string) (string, *audioBankEntry) {
	want := strings.ToLower(name)
	if b, ok := l.banks[want]; ok {
		return want, &b
	}
	for _, key := range l.order {
		b := l.banks[key]
		if strings.ToLower(b.bank.TrackName) == want {
			return key, &b
		}
	}
	return "", nil
}
func (l *AudioLibrary) TrackNameOf(name string) (string, bool) {
	_, b := l.find(name)
	if b == nil {
		return "", false
	}
	return strings.ToLower(b.bank.TrackName), true
}
func (l *AudioLibrary) CloseBank(name string) []string {
	want := strings.ToLower(name)
	_, direct := l.banks[want]
	dropped := []string{}
	for _, key := range slices.Clone(l.order) {
		b := l.banks[key]
		if direct && key != want || !direct && strings.ToLower(b.bank.TrackName) != want {
			continue
		}
		delete(l.banks, key)
		dropped = append(dropped, key)
		l.order = slices.DeleteFunc(l.order, func(s string) bool { return s == key })
		for ck := range l.cache {
			if strings.HasPrefix(ck, key+"|") {
				delete(l.cache, ck)
			}
		}
		delete(l.cache, "theme:"+b.bank.TrackName)
	}
	return dropped
}
func (l *AudioLibrary) BankNames() []string { return slices.Clone(l.order) }
func (l *AudioLibrary) SoundNames(name string) []string {
	_, b := l.find(name)
	if b == nil {
		return []string{}
	}
	return slices.Clone(b.bank.SingleOrder)
}
func (l *AudioLibrary) TrackNames() []string {
	out := []string{}
	for _, key := range l.order {
		if len(l.banks[key].bank.LoopChunks) > 0 {
			out = append(out, key)
		}
	}
	return out
}
func (l *AudioLibrary) LoopTable(name string) (*df.LoopTable, error) {
	_, b := l.find(name)
	if b == nil {
		return nil, nil
	}
	r := df.NewReader(b.file.Data(0), b.file.Order)
	r.Seek(28)
	loc := int(r.I32())
	if r.Err != nil {
		return nil, r.Err
	}
	if loc <= 0 || loc >= len(b.file.Containers) {
		return &df.LoopTable{Order: []int{}, Records: []df.BankChunk{}}, nil
	}
	table, err := df.ReadLoopTable(b.file.Data(loc), b.file.Order)
	return &table, err
}
func (l *AudioLibrary) Sound(name string) (*df.Audio, error) {
	key := strings.TrimSuffix(strings.ToLower(name), ".wav")
	for _, bankKey := range l.order {
		b := l.banks[bankKey]
		ref, ok := b.bank.Singles[key]
		if !ok {
			continue
		}
		cacheKey := bankKey + "|" + key
		if a := l.cache[cacheKey]; a != nil {
			return a, nil
		}
		audio, err := df.DecodeAudio(b.file.Data(ref.ContainerLoc), b.file.Order)
		if err != nil {
			return nil, err
		}
		l.cache[cacheKey] = &audio
		return &audio, nil
	}
	return nil, nil
}
func (l *AudioLibrary) Theme(name string) (*df.Audio, error) {
	candidates := l.order
	if name != "" {
		key, b := l.find(name)
		if b == nil {
			return nil, nil
		}
		candidates = []string{key}
	}
	for _, key := range candidates {
		b := l.banks[key]
		if len(b.bank.LoopChunks) == 0 {
			continue
		}
		cacheKey := "theme:" + b.bank.TrackName
		if a := l.cache[cacheKey]; a != nil {
			return a, nil
		}
		parts := make([]df.Audio, 0, len(b.bank.LoopChunks))
		rate := 0
		for _, loc := range b.bank.LoopChunks {
			part, err := df.DecodeAudio(b.file.Data(loc), b.file.Order)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
			rate = max(rate, part.SampleRate)
		}
		total := 0
		for i := range parts {
			samples, err := df.Resample(parts[i].Samples, parts[i].SampleRate, rate)
			if err != nil {
				return nil, err
			}
			parts[i].Samples = samples
			total += len(samples)
		}
		audio := &df.Audio{SampleRate: rate, Samples: make([]float32, total)}
		at := 0
		for _, part := range parts {
			at += copy(audio.Samples[at:], part.Samples)
		}
		l.cache[cacheKey] = audio
		return audio, nil
	}
	return nil, nil
}
