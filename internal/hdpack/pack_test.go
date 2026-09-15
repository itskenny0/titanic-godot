package hdpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Reader, string, map[string][]byte) {
	t.Helper()
	key := Key(1, 1, []byte{1, 2, 3, 255})
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{200, 10, 20, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Version: 1, Scale: 2, Model: "synthetic", Images: map[string]Entry{key: {Width: 2, Height: 2, SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))}}}
	data, _ := json.Marshal(m)
	files := map[string][]byte{"pack/manifest.json": data, "pack/images/" + key + ".png": b.Bytes()}
	r, err := Open("pack", func(p string) ([]byte, error) { return files[p], nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r, key, files
}
func TestValidatedReadAndFallback(t *testing.T) {
	r, key, files := fixture(t)
	img := r.Get(key, 1, 1)
	if img == nil || img.NRGBAAt(0, 0).R != 200 {
		t.Fatal("replacement missing")
	}
	if r.Get(key, 1, 1) != img || r.CachedBytes() != 16 {
		t.Fatal("cache did not reuse pixels")
	}
	if r.Get(key, 2, 1) != nil || r.Get(strings.Repeat("0", 64), 1, 1) != nil {
		t.Fatal("wrong source matched")
	}
	r, key, files = fixture(t)
	files["pack/images/"+key+".png"] = []byte("corrupt")
	if r.Get(key, 1, 1) != nil {
		t.Fatal("corrupt image accepted")
	}
}
func TestRejectManifest(t *testing.T) {
	for _, s := range []string{`{}`, `{"version":2,"scale":2,"images":{}}`, `{"version":1,"scale":2,"images":{"../escape":{"width":2,"height":2,"sha256":"bad"}}}`} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Fatal("accepted invalid manifest")
		}
	}
}

func TestLosslessWebP(t *testing.T) {
	// A synthetic 2x2 image, encoded losslessly with libwebp. No game data.
	data, err := hex.DecodeString("5249464620000000574542505650384c130000002f014000100f300a833c14f31ff0988088fe4700")
	if err != nil {
		t.Fatal(err)
	}
	r, key, files := fixture(t)
	entry := r.Manifest.Images[key]
	entry.Format = "webp"
	entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	r.Manifest.Images[key] = entry
	files["pack/"+entry.Filename(key)] = data
	img := r.Get(key, 1, 1)
	if img == nil || img.NRGBAAt(0, 0) != (color.NRGBA{200, 10, 20, 255}) || img.NRGBAAt(1, 1).A != 0 {
		t.Fatal("lossless color/alpha changed")
	}
}
