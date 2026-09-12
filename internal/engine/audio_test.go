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

func TestAudioLibraryReference(t *testing.T) {
	path := os.Getenv("TAOOT_AUDIO_LIBRARY_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_AUDIO_LIBRARY_REFERENCE to owned-data audio hashes")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type fingerprint struct {
		Rate, Samples int
		PCM           string
	}
	var corpus []struct {
		Path, Name, TrackName string
		Sounds                []struct {
			Name  string
			Audio *fingerprint
		}
		Theme     *fingerprint
		LoopTable struct {
			Chunks []struct {
				Index int
				Name  string
			}
			Order []int
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	sounds, themes := 0, 0
	for _, e := range corpus {
		data, err := os.ReadFile(e.Path)
		if err != nil {
			t.Fatal(err)
		}
		lib := NewAudioLibrary()
		if err = lib.OpenBank(e.Name, data); err != nil {
			t.Fatal(err)
		}
		name, ok := lib.TrackNameOf(e.Name)
		if !ok || name != e.TrackName {
			t.Fatal("track name differs: " + e.Path)
		}
		check := func(want *fingerprint, got *df.Audio, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(e.Path, err)
			}
			if want == nil {
				if got != nil {
					t.Fatal("expected missing audio")
				}
				return
			}
			if got == nil {
				t.Fatal("missing audio: " + e.Path)
			}
			bytes := make([]byte, len(got.Samples)*4)
			for i, s := range got.Samples {
				binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(s))
			}
			sum := sha256.Sum256(bytes)
			if want.Rate != got.SampleRate || want.Samples != len(got.Samples) || want.PCM != hex.EncodeToString(sum[:]) {
				t.Fatal("PCM differs: " + e.Path)
			}
		}
		for _, s := range e.Sounds {
			a, err := lib.Sound(s.Name)
			check(s.Audio, a, err)
			again, err := lib.Sound(s.Name)
			if err != nil || a != again {
				t.Fatal("named sound was decoded twice")
			}
			sounds++
		}
		table, err := lib.LoopTable(e.Name)
		if err != nil || table == nil {
			t.Fatal("missing loop table", e.Path, err)
		}
		if !reflect.DeepEqual(table.Order, e.LoopTable.Order) || len(table.Records) != len(e.LoopTable.Chunks) {
			t.Fatal("loop table differs", e.Path)
		}
		for i, c := range table.Records {
			if c.ContainerLoc != e.LoopTable.Chunks[i].Index || c.Identifier != e.LoopTable.Chunks[i].Name {
				t.Fatal("loop chunk differs", e.Path)
			}
		}
		theme, err := lib.Theme(e.Name)
		check(e.Theme, theme, err)
		if theme != nil {
			themes++
		}
		lib.CloseBank(e.Name)
		if names := lib.BankNames(); len(names) != 0 {
			t.Fatal("bank did not close")
		}
		if a, err := lib.Theme(e.Name); a != nil || err != nil {
			t.Fatal("closed theme remains available")
		}
	}
	t.Logf("Compared %d named sounds and %d assembled themes", sounds, themes)
}
