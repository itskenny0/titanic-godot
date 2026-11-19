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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestVectorTransparencyNeedsNewManifest(t *testing.T) {
	r, key, _ := fixture(t)
	entry := r.Manifest.Images[key]
	entry.SpriteAlpha = true
	r.Manifest.Images[key] = entry
	data, _ := json.Marshal(r.Manifest)
	if _, err := Parse(data); err == nil {
		t.Fatal("legacy manifest accepted new sprite alpha semantics")
	}
	r.Manifest.Version = VectorVersion
	data, _ = json.Marshal(r.Manifest)
	if _, err := Parse(data); err != nil {
		t.Fatal(err)
	}
	if !r.SpriteAlpha(key, 1, 1) || r.SpriteAlpha(key, 2, 1) || r.SpriteAlpha("missing", 1, 1) {
		t.Fatal("sprite alpha matched the wrong source")
	}
	r.Manifest.Version = 3
	data, _ = json.Marshal(r.Manifest)
	if _, err := Parse(data); err == nil {
		t.Fatal("unknown version accepted")
	}
}

func TestHasSizeCoversPackAndFallback(t *testing.T) {
	r, _, _ := fixture(t)
	if !r.HasSize(1, 1) || r.HasSize(2, 1) || r.HasSize(512, 384) {
		t.Fatal("size filter does not match the manifest")
	}
	var missing *Reader
	if missing.HasSize(1, 1) {
		t.Fatal("absent pack claims artwork")
	}
	vector := &Reader{Manifest: Manifest{Images: map[string]Entry{"x": {Width: 128, Height: 34}}}, Fallback: r}
	if !vector.HasSize(64, 17) || !vector.HasSize(1, 1) || vector.HasSize(3, 3) {
		t.Fatal("size filter ignores the fallback pack")
	}
}

// asyncFixture holds n distinct 2x2 replacements for 1x1 sources.
func asyncFixture(t *testing.T, n int) (map[string][]byte, []string) {
	t.Helper()
	files, keys := map[string][]byte{}, []string{}
	m := Manifest{Version: 1, Scale: 2, Model: "synthetic", Images: map[string]Entry{}}
	for i := 0; i < n; i++ {
		key := Key(1, 1, []byte{byte(i), 2, 3, 255})
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		img.SetNRGBA(0, 0, color.NRGBA{uint8(100 + i), 10, 20, 255})
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		m.Images[key] = Entry{Width: 2, Height: 2, SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))}
		files["images/"+key+".png"] = b.Bytes()
		keys = append(keys, key)
	}
	files["manifest.json"], _ = json.Marshal(m)
	return files, keys
}
func waitCollected(t *testing.T, r *Reader) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); r.Loading() > 0; {
		r.Collect()
		if time.Now().After(deadline) {
			t.Fatal("background decode did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestAsyncDecodesOffThreadAndRetriesWhenBusy(t *testing.T) {
	files, keys := asyncFixture(t, 4)
	reads := 0
	r, err := Open("pack", func(p string) ([]byte, error) { reads++; return files[strings.TrimPrefix(p, "pack/")], nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Async = true
	if r.Collect() {
		t.Fatal("nothing was loading")
	}
	if img, state := r.Lookup(keys[0], 1, 1, false); img != nil || state != Busy || r.Loading() != 0 {
		t.Fatal("cache-only lookup started a load")
	}
	for i, key := range keys {
		img, state := r.Lookup(key, 1, 1, true)
		want := Pending
		if i >= asyncWorkers {
			want = Busy
		}
		if img != nil || state != want {
			t.Fatalf("lookup %d: image=%v state=%v", i, img != nil, state)
		}
	}
	if _, state := r.Lookup(keys[0], 1, 1, true); state != Pending || r.Loading() != asyncWorkers || reads != 1+asyncWorkers {
		t.Fatal("pending image was requested twice")
	}
	waitCollected(t, r)
	for i, key := range keys {
		img, state := r.Lookup(key, 1, 1, true)
		if i < asyncWorkers && (state != Ready || img.NRGBAAt(0, 0).R != uint8(100+i)) {
			t.Fatalf("image %d not adopted", i)
		}
		if i >= asyncWorkers && state != Pending {
			t.Fatalf("starved image %d was not retried: %v", i, state)
		}
	}
	waitCollected(t, r)
	if r.Get(keys[3], 1, 1) == nil || r.CachedBytes() != 4*16 {
		t.Fatal("retried image missing")
	}
}
func TestAsyncReadsLocalPacksOnTheWorkerAndRejectsCorruptImages(t *testing.T) {
	files, keys := asyncFixture(t, 2)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	files["images/"+keys[1]+".png"] = []byte("corrupt")
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reads, logs := 0, []string{}
	r, err := Open(root, func(p string) ([]byte, error) { reads++; return os.ReadFile(p) }, func(s string) { logs = append(logs, s) })
	if err != nil {
		t.Fatal(err)
	}
	r.Async = true
	r.Get(keys[0], 1, 1)
	r.Get(keys[1], 1, 1)
	waitCollected(t, r)
	if reads != 1 {
		t.Fatalf("game thread read %d files from a local pack", reads)
	}
	if img, state := r.Lookup(keys[0], 1, 1, true); state != Ready || img == nil {
		t.Fatal("local image missing")
	}
	if img, state := r.Lookup(keys[1], 1, 1, true); state != Failed || img != nil || len(logs) != 1 || r.Loading() != 0 {
		t.Fatal("corrupt image accepted or retried", state, logs)
	}
}
func TestAsyncFallbackPackIsCollectedThroughTheLayeredPack(t *testing.T) {
	files, keys := asyncFixture(t, 1)
	base, err := Open("pack", func(p string) ([]byte, error) { return files[strings.TrimPrefix(p, "pack/")], nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	base.Async = true
	other, _ := asyncFixture(t, 3)
	top, err := Open("top", func(p string) ([]byte, error) { return other[strings.TrimPrefix(p, "top/")], nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	delete(top.Manifest.Images, keys[0])
	top.Fallback = base
	if img, state := top.Lookup(keys[0], 1, 1, true); img != nil || state != Pending {
		t.Fatal("fallback did not start decoding", state)
	}
	for deadline := time.Now().Add(5 * time.Second); !top.Collect(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("layered pack never reported new artwork")
		}
	}
	if top.Get(keys[0], 1, 1) == nil || !top.HasSize(1, 1) {
		t.Fatal("fallback artwork missing")
	}
}
