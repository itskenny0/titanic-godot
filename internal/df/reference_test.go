package df

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func TestReferenceCorpus(t *testing.T) {
	path := os.Getenv("TAOOT_CODEC_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_CODEC_REFERENCE to hashes from tests/go-codec-reference.ts")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path, Roundtrip string
		Rings           [][]struct {
			Loc, Width, Height, ZOffset int
			Pixels, Z                   string
		}
		Audio []struct {
			Loc, Rate, Samples int
			PCM                string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	frames, chunks := 0, 0
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := ReadFile(data)
			if err != nil {
				t.Fatal(err)
			}
			written, err := file.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if digest(written) != entry.Roundtrip {
				t.Fatal("container serialization differs")
			}
			for ring, refs := range entry.Rings {
				var fb FrameBuffer
				for index, want := range refs {
					got, err := DecodeFrame(file.Data(want.Loc), &fb, file.Order)
					if err != nil {
						t.Fatalf("ring %d frame %d container %d: %v", ring, index, want.Loc, err)
					}
					if got.Width != want.Width || got.Height != want.Height || got.ZOffset != want.ZOffset {
						t.Fatalf("container %d dimensions/depth offset differ: %+v", want.Loc, got)
					}
					n := got.Width * got.Height
					if digest(fb.Pixels[:n]) != want.Pixels {
						t.Fatalf("ring %d frame %d container %d pixels differ", ring, index, want.Loc)
					}
					if got.HasZ && digest(fb.ZPixels[:n]) != want.Z {
						t.Fatalf("container %d depth differs", want.Loc)
					}
					frames++
				}
			}
			for _, want := range entry.Audio {
				got, err := DecodeAudio(file.Data(want.Loc), file.Order)
				if err != nil {
					t.Fatalf("audio %d: %v", want.Loc, err)
				}
				pcm := make([]byte, len(got.Samples)*4)
				for i, s := range got.Samples {
					binary.LittleEndian.PutUint32(pcm[i*4:], math.Float32bits(s))
				}
				if got.SampleRate != want.Rate || len(got.Samples) != want.Samples || digest(pcm) != want.PCM {
					t.Fatalf("audio %d differs", want.Loc)
				}
				chunks++
			}
		})
	}
	t.Logf("Compared %d files, %d frames, %d audio chunks", len(corpus), frames, chunks)
}
