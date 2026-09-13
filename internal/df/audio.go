package df

import (
	"fmt"
	"math"
)

type Audio struct {
	SampleRate int
	Samples    []float32
}
type AudioHeader struct{ Codec, SampleRate, ByteSize, DataStart int }

func ReadAudioHeader(data []byte, order Order) (AudioHeader, error) {
	r := NewReader(data, order)
	if len(data) < 48 || r.I32() != 0x10000 {
		return AudioHeader{}, fmt.Errorf("audio container: bad header")
	}
	r.Seek(26)
	codec := int(r.I16())
	rate := int(r.I32())
	r.Seek(36)
	size := int(r.I32())
	r.Seek(44)
	start := int(r.I32())
	if rate <= 0 || size < 0 || size > 128<<20 || start < 48 || start > len(data) {
		return AudioHeader{}, fmt.Errorf("audio container: invalid dimensions")
	}
	return AudioHeader{codec, rate, size, start}, nil
}

func DecodeAudio(data []byte, order Order, target ...[]float32) (Audio, error) {
	h, err := ReadAudioHeader(data, order)
	if err != nil {
		return Audio{}, err
	}
	r := NewReader(data, order)
	r.Seek(h.DataStart)
	a := Audio{SampleRate: h.SampleRate}
	n := h.ByteSize
	if h.Codec != 1 {
		n /= 2
	}
	if len(target) > 0 {
		if len(target[0]) != n {
			return Audio{}, fmt.Errorf("audio output size mismatch")
		}
		a.Samples = target[0]
	} else {
		a.Samples = make([]float32, n)
	}
	if h.Codec == 1 {
		if h.ByteSize == 0 {
			return a, nil
		}
		pos := 0
		put := func(v int8) {
			if pos < len(a.Samples) {
				a.Samples[pos] = float32(int(uint8(v))-128) / 128
			}
			pos++
		}
		prev := int8(r.U8())
		put(prev + 0x40)
		for pos < h.ByteSize && r.Err == nil {
			n := r.U8()
			switch {
			case n&0x80 == 0:
				prev = int8(n)
				put(prev + 0x40)
			case n&0x40 == 0:
				for i := 0; i < int(n&63)+1; i++ {
					b := r.U8()
					hi := int8(b) >> 4
					lo := int8(b<<4) >> 4
					step := prev + hi
					prev = step + lo
					put(step + 0x40)
					put(prev + 0x40)
				}
			default:
				for i := 0; i < int(n&63)+1; i++ {
					put(prev + 0x40)
				}
			}
		}
	} else {
		if h.ByteSize%2 != 0 {
			return Audio{}, fmt.Errorf("audio container: odd 16-bit PCM size")
		}
		var current int16
		for i := range a.Samples {
			b := r.U8()
			n := int16(int8(b<<1) >> 1)
			if b&0x80 == 0 {
				current += n * 32
			} else {
				current = n * 512
			}
			a.Samples[i] = float32(current) / 32768
		}
	}
	if r.Err != nil {
		return Audio{}, r.Err
	}
	return a, nil
}

func Resample(samples []float32, from, to int) ([]float32, error) {
	if from <= 0 || to <= 0 {
		return nil, fmt.Errorf("invalid sample rate")
	}
	if from == to || len(samples) == 0 {
		return samples, nil
	}
	ratio := float64(to) / float64(from)
	size := math.Max(1, math.Floor(float64(float64(len(samples))*ratio)+0.5))
	if size > 32<<20 {
		return nil, fmt.Errorf("resampled audio too large")
	}
	out := make([]float32, int(size))
	for i := range out {
		src := float64(i) / ratio
		p := int(src)
		fraction := src - float64(p)
		// Clamp the final interpolation to the final sample when upsampling.
		p = min(p, len(samples)-1)
		q := min(p+1, len(samples)-1)
		out[i] = float32(float64(float64(samples[p])*(1-fraction)) + float64(float64(samples[q])*fraction))
	}
	return out, nil
}

func (a Audio) StereoPCM(volume, pan float64) []byte {
	out := make([]byte, len(a.Samples)*4)
	_ = a.WriteStereoPCM(out, volume, pan)
	return out
}

func (a Audio) WriteStereoPCM(out []byte, volume, pan float64) error {
	if len(out) != len(a.Samples)*4 {
		return fmt.Errorf("stereo output size mismatch")
	}
	volume = math.Max(0, math.Min(1, volume))
	pan = math.Max(-1, math.Min(1, pan))
	l := float64(volume * math.Cos(float64((pan+1)*math.Pi)/4))
	r := float64(volume * math.Sin(float64((pan+1)*math.Pi)/4))
	for i, sample := range a.Samples {
		s := float64(math.Max(-1, math.Min(1, float64(sample))) * 32767)
		lv, rv := uint16(int16(math.Floor(float64(s*l)+0.5))), uint16(int16(math.Floor(float64(s*r)+0.5)))
		out[i*4], out[i*4+1], out[i*4+2], out[i*4+3] = byte(lv), byte(lv>>8), byte(rv), byte(rv>>8)
	}
	return nil
}
