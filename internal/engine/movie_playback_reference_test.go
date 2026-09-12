package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func TestMoviePlaybackReference(t *testing.T) {
	path := os.Getenv("TAOOT_MOVIE_PLAYBACK_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_MOVIE_PLAYBACK_REFERENCE to local movie streaming and soundtrack reference")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path     string
		Segments []struct {
			Interval, BedMS, OnScreenMS float64
			Holds                       []float64
			Waits                       []bool
			Audio                       *struct {
				Rate              int
				Seconds           float64
				Unique, Resampled []string
			}
			Bed *struct {
				Rate, Samples int
				Pixels        string
				Loop          bool
			}
			Visits []struct {
				Index, Width, Height, Retained, Decoded int
				Pixels                                  string
			}
		}
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	hash := func(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
	hashPCM := func(data []float32) string {
		b := make([]byte, len(data)*4)
		for i, x := range data {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
		}
		return hash(b)
	}
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := df.ReadMovie(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Segments) != len(entry.Segments) {
				t.Fatal("segment count differs")
			}
			for i, want := range entry.Segments {
				seg := &m.Segments[i]
				a, err := df.DecodeMovieSegmentAudio(seg)
				if err != nil {
					t.Fatal(err)
				}
				seconds := 0.0
				if a != nil {
					seconds = a.AudioSec
				}
				if (a == nil) != (want.Audio == nil) {
					t.Fatal("audio presence differs", i)
				}
				if a != nil {
					if a.Rate != want.Audio.Rate || seconds != want.Audio.Seconds {
						t.Fatal("soundtrack rate or unique duration differs", i)
					}
					unique, resampled := []string{}, []string{}
					for _, p := range a.Unique {
						unique = append(unique, hashPCM(p))
					}
					for _, p := range a.Resampled {
						resampled = append(resampled, hashPCM(p))
					}
					if !reflect.DeepEqual(unique, want.Audio.Unique) || !reflect.DeepEqual(resampled, want.Audio.Resampled) {
						t.Fatal("resampled soundtrack differs", i)
					}
				}
				interval, bedMS := df.MovieSegmentInterval(seg, len(seg.Frames), seconds, i), df.MovieBedRuntimeMS(m, i)
				if interval != want.Interval || bedMS != want.BedMS || df.MovieSegmentOnScreenMS(seg) != want.OnScreenMS {
					t.Fatal("movie timing differs", i, interval, want.Interval, bedMS, want.BedMS)
				}
				for j := range seg.Frames {
					if df.MovieFrameHoldMS(seg, j) != want.Holds[j] || df.MovieFrameWaits(seg, j) != want.Waits[j] {
						t.Fatal("frame hold or input wait differs", i, j)
					}
				}
				if a != nil {
					bed := df.MovieSoundtrackFor(seg, a, interval, len(seg.Frames), bedMS)
					if bed.SampleRate != want.Bed.Rate || len(bed.Samples) != want.Bed.Samples || bed.Loop != want.Bed.Loop || hashPCM(bed.Samples) != want.Bed.Pixels {
						t.Fatal("soundtrack output differs", i)
					}
				}
				stream := NewMovieFrames(seg)
				for _, visit := range want.Visits {
					f, err := stream.Get(visit.Index)
					if err != nil {
						t.Fatal(err)
					}
					if f.Width != visit.Width || f.Height != visit.Height || hash(f.Pixels) != visit.Pixels {
						t.Fatal("streamed frame pixels differ", i, visit.Index)
					}
					if stream.RetainedBytes() != visit.Retained || stream.DecodedFrames != visit.Decoded {
						t.Fatal("cache memory or decode count differs", i, visit.Index, stream.RetainedBytes(), visit.Retained, stream.DecodedFrames, visit.Decoded)
					}
				}
			}
		})
	}
}
