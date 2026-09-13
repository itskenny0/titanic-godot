package df

import (
	"math"
	"strings"
)

const MovieTickMS = 50.0 / 3
const NativeMovieFrameMS = 66.0
const MovieAnyInputAborts = 8

func MovieFrameWaits(s *MovieSegment, index int) bool {
	return index >= 0 && index < len(s.Frames) && len(s.Frames[index].Regions) > 0 && !s.Frames[index].PlaysThroughRegions
}
func MovieFrameHoldMS(s *MovieSegment, index int) float64 {
	if index < 0 || index >= len(s.Frames) {
		return NativeMovieFrameMS
	}
	return float64(float64(max(s.Frames[index].HoldTicks, s.MinHoldTicks)) * MovieTickMS)
}
func MovieHasRegions(s *MovieSegment) bool {
	for _, f := range s.Frames {
		if len(f.Regions) > 0 {
			return true
		}
	}
	return false
}
func MovieSegmentInterval(s *MovieSegment, frameCount int, audioSec float64, segmentIndex int) float64 {
	regions := MovieHasRegions(s)
	hasStep := false
	for _, f := range s.Frames {
		if f.Type == 6 || f.Type == 7 {
			hasStep = true
			break
		}
	}
	interval := 0.0
	switch {
	case !regions && s.AudioLoops && audioSec > 0:
		interval = math.Max(NativeMovieFrameMS, float64(audioSec*1000)/float64(frameCount))
	case regions:
		interval = NativeMovieFrameMS
		if len(s.AudioChunks) > 0 || len(s.Sounds) > 0 {
			interval = 145
		}
	case hasStep:
		interval = math.Max(NativeMovieFrameMS, math.Min(1200, 3000/float64(frameCount)))
	}
	if interval == 0 && segmentIndex > 0 && !regions {
		interval = NativeMovieFrameMS
	}
	return interval
}
func MovieSegmentOnScreenMS(s *MovieSegment) float64 {
	t, soundEnds := 0.0, 0.0
	for i, f := range s.Frames {
		if loc, ok := s.Sounds[strings.ToLower(f.Sound)]; f.Sound != "" && ok {
			if a, err := DecodeAudio(s.File.Data(loc), s.File.Order); err == nil {
				soundEnds = math.Max(soundEnds, t+float64(float64(len(a.Samples))/float64(a.SampleRate)*1000))
			}
		}
		t += MovieFrameHoldMS(s, i)
		if f.WaitsForVoice {
			t = math.Max(t, soundEnds)
		}
	}
	return t
}
func MovieBedRuntimeMS(m *Movie, index int) float64 {
	ms := 0.0
	for i := index; i < len(m.Segments); i++ {
		s := &m.Segments[i]
		if i > index && len(s.AudioChunks) > 0 {
			break
		}
		ms += MovieSegmentOnScreenMS(s)
		waits := false
		for j := range s.Frames {
			if MovieFrameWaits(s, j) {
				waits = true
				break
			}
		}
		if waits {
			break
		}
	}
	return ms
}

type MovieSegmentAudio struct {
	Rate              int
	Resampled, Unique [][]float32
	AudioSec          float64
}
type MovieSoundtrack struct {
	Audio
	Loop bool
}

func DecodeMovieSegmentAudio(s *MovieSegment) (*MovieSegmentAudio, error) {
	if len(s.AudioChunks) == 0 {
		return nil, nil
	}
	// Repeated loop-table entries share decoded samples. Their order is retained.
	decoded := map[int]Audio{}
	rate := 0
	for _, loc := range s.AudioChunks {
		if _, ok := decoded[loc]; ok {
			continue
		}
		a, err := DecodeAudio(s.File.Data(loc), s.File.Order)
		if err != nil {
			return nil, err
		}
		decoded[loc] = a
		rate = max(rate, a.SampleRate)
	}
	out := &MovieSegmentAudio{Rate: rate}
	resampled := map[int][]float32{}
	uniqueCount := 0
	for _, loc := range s.AudioChunks {
		samples, ok := resampled[loc]
		if !ok {
			a := decoded[loc]
			var err error
			samples, err = Resample(a.Samples, a.SampleRate, rate)
			if err != nil {
				return nil, err
			}
			resampled[loc] = samples
			out.Unique = append(out.Unique, samples)
			uniqueCount += len(samples)
		}
		out.Resampled = append(out.Resampled, samples)
	}
	out.AudioSec = float64(uniqueCount) / float64(rate)
	return out, nil
}
func concatMovieAudio(parts [][]float32, cap int) []float32 {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	total = min(cap, total)
	out := make([]float32, total)
	at := 0
	for _, p := range parts {
		if at >= total {
			break
		}
		at += copy(out[at:], p)
	}
	return out
}
func MovieSoundtrackFor(s *MovieSegment, a *MovieSegmentAudio, interval float64, frameCount int, onScreenMS float64) *MovieSoundtrack {
	if MovieHasRegions(s) && s.AudioLoops {
		return &MovieSoundtrack{Audio: Audio{SampleRate: a.Rate, Samples: concatMovieAudio(a.Unique, int(^uint(0)>>1))}, Loop: true}
	}
	predicted := 0.0
	if interval > 0 {
		predicted = float64(interval*float64(frameCount)) / 1000
	}
	runtime := math.Max(a.AudioSec, math.Max(predicted, onScreenMS/1000))
	cap := max(1, int(math.Ceil(float64(float64(runtime*1.1)*float64(a.Rate)))))
	samples := concatMovieAudio(a.Resampled, cap)
	return &MovieSoundtrack{Audio: Audio{SampleRate: a.Rate, Samples: samples}, Loop: s.AudioLoops && len(samples) < cap}
}
