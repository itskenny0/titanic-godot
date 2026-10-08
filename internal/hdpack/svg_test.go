package hdpack

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func svgFixture(t *testing.T) (*Reader, string, map[string][]byte) {
	r, key, files := fixture(t)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"><path fill="#800000" d="M0 0H1V1H0Z"/></svg>`)
	entry := r.Manifest.Images[key]
	entry.Format = "svg"
	entry.SpriteAlpha = true
	entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(svg))
	r.Manifest.Version = VectorVersion
	r.Manifest.Images[key] = entry
	files["pack/images/"+key+".svg"] = svg
	return r, key, files
}
func TestSVGCacheAndHDLayering(t *testing.T) {
	r, key, _ := svgFixture(t)
	calls := 0
	r.SVG = func(path string, svg []byte, w, h int) ([]byte, error) {
		calls++
		if path != "pack/images/"+key+".svg" || w != 2 || h != 2 || len(svg) == 0 {
			t.Fatal("invalid SVG request")
		}
		return []byte{128, 0, 0, 128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, nil
	}
	fallback, _, _ := fixture(t)
	r.Fallback = fallback
	image := r.Get(key, 1, 1)
	if image == nil || image.Pix[3] != 128 || !r.SpriteAlpha(key, 1, 1) {
		t.Fatal("SVG alpha lost")
	}
	for i := 0; i < 5; i++ {
		if r.Get(key, 1, 1) != image {
			t.Fatal("SVG cache not reused")
		}
	}
	if calls != 1 {
		t.Fatal("SVG rerasterized", calls)
	}
	if r.Get(key, 2, 1) != nil {
		t.Fatal("wrong source dimensions matched")
	}
	// An unmatched UI state can still use an external HD image.
	delete(r.Manifest.Images, key)
	if r.Get(key, 1, 1) != fallback.Get(key, 1, 1) || r.SpriteAlpha(key, 1, 1) {
		t.Fatal("HD fallback failed")
	}
}
func TestBadSVGUsesFallbackWithoutRepeatedDecoding(t *testing.T) {
	for _, failure := range []string{"checksum", "decoder", "size", "unavailable"} {
		t.Run(failure, func(t *testing.T) {
			r, key, files := svgFixture(t)
			fallback, _, _ := fixture(t)
			r.Fallback = fallback
			calls := 0
			r.SVG = func(string, []byte, int, int) ([]byte, error) {
				calls++
				if failure == "decoder" {
					return nil, fmt.Errorf("bad SVG")
				}
				return []byte{1}, nil
			}
			if failure == "checksum" {
				files["pack/images/"+key+".svg"] = []byte("changed")
			}
			if failure == "unavailable" {
				r.SVG = nil
			}
			for i := 0; i < 3; i++ {
				if r.Get(key, 1, 1) != fallback.Get(key, 1, 1) || r.SpriteAlpha(key, 1, 1) {
					t.Fatal("failed SVG poisoned fallback")
				}
			}
			if calls > 1 || (failure == "checksum" && calls != 0) {
				t.Fatal("invalid SVG repeatedly decoded")
			}
		})
	}
}
