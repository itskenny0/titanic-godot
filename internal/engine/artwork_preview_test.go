package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/hdpack"
)

func TestOwnedArtworkExamplesDoNotBootGame(t *testing.T) {
	files := ownedFiles(t)
	read := func(path string) ([]byte, error) {
		if strings.HasPrefix(path, "res://") {
			path = filepath.Join("../../godot", strings.TrimPrefix(path, "res://"))
		}
		return os.ReadFile(path)
	}
	p := NewPlayer(&testPlayerBridge{read: read})
	defer p.Close()
	examples, err := p.ArtworkExamples(files.Index)
	if err != nil {
		t.Fatal(err)
	}
	if p.Host != nil || p.Ready || len(p.Events()) != 0 {
		t.Fatal("preview started gameplay")
	}
	pack, err := hdpack.Open("res://artwork/ui", read, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 3 {
		t.Fatalf("expected life, bag and navigation examples, got %d", len(examples))
	}
	for _, e := range examples {
		if len(e.Pixels) != e.Width*e.Height*4 || hdpack.Key(e.Width, e.Height, e.Pixels) != e.Key {
			t.Fatal("preview lost original dimensions, palette or transparency", e.Name)
		}
		if entry, ok := pack.Manifest.Images[e.Key]; !ok || entry.Format != "svg" {
			t.Fatal("original has no matching SVG", e.Name)
		}
	}
	before := fmt.Sprint(p.State())
	if _, err := p.ArtworkExamples(nil); err == nil {
		t.Fatal("missing game files produced invented examples")
	}
	if fmt.Sprint(p.State()) != before {
		t.Fatal("failed preview changed player state")
	}
}
