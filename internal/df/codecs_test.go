package df

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

func TestContainerRoundTrip(t *testing.T) {
	for _, order := range []Order{binary.LittleEndian, binary.BigEndian} {
		for _, kind := range []int32{0, 1, 2} {
			f := File{Order: order, Header: FileHeader{FourCC: 1234, Type: kind, GapWhere: 2}, HeaderRaw: make([]byte, HeaderSize), Containers: []Container{{ID: 17, Data: []byte{1, 2}}, {ID: 19, Data: []byte{3, 4}}, {ID: 20, Data: make([]byte, 8), Gap: true}}}
			f.HeaderRaw[600] = 73
			data, err := f.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadFile(data)
			if err != nil {
				t.Fatal(err)
			}
			if got.Order != order || got.HeaderRaw[600] != 73 || got.Containers[0].ID != 17 || !got.Containers[2].Gap {
				t.Fatalf("header/container mismatch: %+v", got.Header)
			}
			before := append([]byte(nil), data...)
			if err := got.Patch(0, func(b []byte) error { b[0] = 99; return nil }); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, data) {
				t.Fatal("patch changed source bytes")
			}
			written, err := got.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			again, err := ReadFile(written)
			if err != nil {
				t.Fatal(err)
			}
			if again.Data(0)[0] != 99 {
				t.Fatal("patch lost")
			}
		}
	}
	for _, bad := range [][]byte{nil, make([]byte, 32), append(bytes.Repeat([]byte{255}, 24), make([]byte, 1024)...)} {
		if _, err := ReadFile(bad); err == nil {
			t.Fatal("accepted damaged container file")
		}
	}
}

func TestFrameDeltaModes(t *testing.T) {
	var fb FrameBuffer
	cases := []struct {
		data, want []byte
		z          int
	}{
		{[]byte{1, 0, 8, 0, 20, 21, 1, 2, 55, 2, 0}, []byte{1, 2, 1, 2, 1, 2, 1, 2}, -1},
		{[]byte{1, 0, 5, 0, 20, 40, 7, 240}, []byte{7, 7, 7, 7, 7}, -1},
		{[]byte{2, 0, 2, 0, 4, 1, 2, 4, 3, 4}, []byte{1, 2, 3, 4}, -1},
		{[]byte{2, 0, 2, 0, 60, 40}, []byte{3, 4, 3, 4}, -1},
		{[]byte{1, 0, 2, 0, 4, 3, 4, 2, 0, 1, 2, 7}, []byte{3, 4}, 7},
	}
	for i, c := range cases {
		f, err := DecodeFrame(c.data, &fb, binary.LittleEndian)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if !bytes.Equal(fb.Pixels[:f.Width*f.Height], c.want) || f.ZOffset != c.z {
			t.Fatalf("case %d: got %v %+v", i, fb.Pixels, f)
		}
	}
	if !bytes.Equal(fb.ZPixels[:2], []byte{7, 7}) {
		t.Fatal("depth run differs")
	}
	for _, bad := range [][]byte{nil, {1, 0, 2, 0, 56}, {1, 0, 2, 0, 4, 1}, {1, 0, 2, 0, 20, 254, 1}} {
		if _, err := DecodeFrame(bad, &FrameBuffer{}, nil); err == nil {
			t.Fatal("accepted damaged frame", bad)
		}
	}
}

func TestAudioModes(t *testing.T) {
	for _, order := range []Order{binary.LittleEndian, binary.BigEndian} {
		for _, c := range []struct {
			codec, size int
			stream      []byte
			want        []float32
		}{
			{1, 6, []byte{64, 0x80, 0x1f, 0xc1, 0}, []float32{0, 1.0 / 128, 0, 0, 0, -0.5}},
			{2, 6, []byte{0x81, 0x7f, 0xc0}, []float32{512.0 / 32768, 480.0 / 32768, -1}},
		} {
			data := make([]byte, 48+len(c.stream))
			order.PutUint32(data, 0x10000)
			order.PutUint16(data[26:], uint16(c.codec))
			order.PutUint32(data[28:], 22050)
			order.PutUint32(data[36:], uint32(c.size))
			order.PutUint32(data[44:], 48)
			copy(data[48:], c.stream)
			a, err := DecodeAudio(data, order)
			if err != nil {
				t.Fatal(err)
			}
			if a.SampleRate != 22050 || !reflect.DeepEqual(a.Samples, c.want) {
				t.Fatalf("audio mismatch: %+v want %v", a, c.want)
			}
			target := make([]float32, len(c.want))
			if _, err := DecodeAudio(data, order, target); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(target, c.want) {
				t.Fatal("borrowed output differs")
			}
			if _, err := DecodeAudio(data[:len(data)-1], order); err == nil {
				t.Fatal("accepted truncated audio")
			}
		}
	}
	a := Audio{Samples: []float32{-1, .5, 1}}
	pcm := a.StereoPCM(1, -1)
	for i, want := range []int16{-32767, 16384, 32767} {
		if int16(binary.LittleEndian.Uint16(pcm[i*4:])) != want || binary.LittleEndian.Uint16(pcm[i*4+2:]) != 0 {
			t.Fatal("stereo pan/rounding differs")
		}
	}
	resampled, err := Resample([]float32{0, 1}, 1, 2)
	if err != nil || !reflect.DeepEqual(resampled, []float32{0, .5, 1, 1}) {
		t.Fatal("upsampling end point", resampled, err)
	}
	if math.IsNaN(float64(resampled[len(resampled)-1])) {
		t.Fatal("invalid final sample")
	}
}
