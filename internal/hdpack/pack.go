// Package hdpack loads locally generated artwork without changing game data.
package hdpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/image/webp"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Version = 1

// VectorVersion allows authored sprite transparency; older players reject it
// instead of drawing smooth vector sprites through the original pixel masks.
const VectorVersion = 2
const Scale = 2
const Budget = 24 << 20
const MaxImageBytes = 8 << 20

type Entry struct {
	SpriteAlpha bool   `json:"sprite_alpha,omitempty"`
	Format      string `json:"format,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	SHA256      string `json:"sha256"`
}
type Manifest struct {
	Version    int              `json:"version"`
	Scale      int              `json:"scale"`
	Characters bool             `json:"characters,omitempty"`
	Model      string           `json:"model"`
	Images     map[string]Entry `json:"images"`
}

// Key uses the exact displayed source pixels, including palette and alpha.
// Different editions, mods and palette effects can never select unrelated art.
func Key(w, h int, rgba []byte) string {
	sum := sha256.New()
	var size [8]byte
	binary.LittleEndian.PutUint32(size[:4], uint32(w))
	binary.LittleEndian.PutUint32(size[4:], uint32(h))
	sum.Write(size[:])
	sum.Write(rgba)
	return hex.EncodeToString(sum.Sum(nil))
}
func validHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && s == strings.ToLower(s)
}
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if len(data) > 16<<20 {
		return m, fmt.Errorf("HD manifest exceeds 16 MiB")
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if (m.Version != Version && m.Version != VectorVersion) || m.Scale != Scale || len(m.Images) == 0 || len(m.Images) > 100000 {
		return m, fmt.Errorf("unsupported or empty HD manifest")
	}
	for key, e := range m.Images {
		if (e.SpriteAlpha || e.Format == "svg") && m.Version != VectorVersion {
			return m, fmt.Errorf("authored sprite alpha needs HD manifest version 2")
		}
		if (e.Format != "" && e.Format != "png" && e.Format != "webp" && e.Format != "svg") || !validHash(key) || !validHash(e.SHA256) || e.Width < 1 || e.Height < 1 || e.Width > 1024 || e.Height > 768 {
			return m, fmt.Errorf("invalid HD image entry %q", key)
		}
	}
	return m, nil
}

func (r *Reader) SpriteAlpha(key string, w, h int) bool {
	e, ok := r.Manifest.Images[key]
	if ok && e.Width == w*Scale && e.Height == h*Scale && r.Get(key, w, h) != nil && r.cache[key].image != nil {
		return r.Manifest.Version == VectorVersion && e.SpriteAlpha && e.Width == w*Scale && e.Height == h*Scale
	}
	return r.Fallback != nil && r.Fallback.SpriteAlpha(key, w, h)
}

type cached struct {
	image   *image.NRGBA
	used    uint64
	pending bool
}

// SVGRenderer returns straight-alpha RGBA pixels at the requested dimensions.
// Godot supplies the decoder; headless Go callers can leave it unset.
type SVGRenderer func(string, []byte, int, int) ([]byte, error)

// asyncWorkers bounds background decodes. Two leave the game and audio
// threads their own cores on a four-core handheld.
const asyncWorkers = 2

// State describes what a lookup found.
type State int

const (
	Absent  State = iota // no artwork replaces this image
	Ready                // decoded and returned
	Pending              // decoding in the background; a repaint follows
	Busy                 // not started: every worker is occupied, or loading was not requested
	Failed               // unreadable or invalid; the original is used
)

// loadJob is one image decoding off the game thread. The worker owns it until
// done closes; only then does the game thread read the result.
type loadJob struct {
	key   string
	entry Entry
	path  string
	data  []byte
	done  chan struct{}
	image *image.NRGBA
	err   error
}

// Reader serves replacement artwork from a bounded cache.
//
// With Async set, a missing image is verified and decoded on a worker
// goroutine while lookups keep returning nil, so the caller draws the
// original until Collect reports that new artwork arrived. All cache state
// still changes only on the caller's thread. SVG entries always decode
// immediately: their renderer belongs to the caller's thread.
type Reader struct {
	SVG      SVGRenderer
	Fallback *Reader
	Manifest Manifest
	Root     string
	Read     func(string) ([]byte, error)
	Log      func(string)
	Async    bool
	cache    map[string]cached
	sizes    map[[2]int]bool
	clock    uint64
	bytes    int
	jobs     []*loadJob
	local    int8
}

// HasSize reports whether any artwork replaces a w x h source image. Callers
// use it to avoid hashing pixels that cannot match: a small pack such as the
// interface controls never replaces a full-screen picture.
func (r *Reader) HasSize(w, h int) bool {
	if r == nil {
		return false
	}
	if r.sizes == nil {
		r.sizes = make(map[[2]int]bool, len(r.Manifest.Images))
		for _, e := range r.Manifest.Images {
			r.sizes[[2]int{e.Width, e.Height}] = true
		}
	}
	return r.sizes[[2]int{w * Scale, h * Scale}] || r.Fallback.HasSize(w, h)
}

func Open(root string, read func(string) ([]byte, error), log func(string)) (*Reader, error) {
	root = strings.TrimRight(root, "/\\")
	data, err := read(root + "/manifest.json")
	if err != nil {
		return nil, err
	}
	m, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = func(string) {}
	}
	return &Reader{Manifest: m, Root: root, Read: read, Log: log, cache: map[string]cached{}}, nil
}

// localRoot reports whether workers may read the pack with the operating
// system directly. Packs behind a Godot or Android storage scheme can only be
// read through Read, on the caller's thread.
func (r *Reader) localRoot() bool {
	if r.local == 0 {
		r.local = -1
		if filepath.IsAbs(r.Root) && !strings.Contains(r.Root, "://") && !strings.HasPrefix(r.Root, "saf-hd:") {
			if info, err := os.Stat(filepath.Join(r.Root, "manifest.json")); err == nil && info.Mode().IsRegular() {
				r.local = 1
			}
		}
	}
	return r.local > 0
}
func decodeEntry(e Entry, data []byte) (*image.NRGBA, error) {
	if len(data) > MaxImageBytes {
		return nil, fmt.Errorf("HD image exceeds size limit")
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != e.SHA256 {
		return nil, fmt.Errorf("HD image checksum mismatch")
	}
	cfg, err := e.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width != e.Width || cfg.Height != e.Height {
		return nil, fmt.Errorf("HD image dimensions do not match manifest")
	}
	decoded, err := e.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, e.Width, e.Height))
	draw.Draw(out, out.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	return out, nil
}
func (j *loadJob) run(local bool) {
	defer close(j.done)
	if local {
		info, err := os.Stat(j.path)
		if err == nil && info.Size() > MaxImageBytes {
			err = fmt.Errorf("HD image exceeds size limit")
		}
		if err == nil {
			j.data, err = os.ReadFile(j.path)
		}
		if err != nil {
			j.err = err
			return
		}
	}
	j.image, j.err = decodeEntry(j.entry, j.data)
	j.data = nil
}

// Get returns decoded artwork, or nil when the original should be drawn.
func (r *Reader) Get(key string, w, h int) *image.NRGBA {
	img, _ := r.Lookup(key, w, h, true)
	return img
}

// Lookup is Get with the reason for a nil result. With load false an
// asynchronous reader answers from its cache only and starts nothing.
func (r *Reader) Lookup(key string, w, h int, load bool) (*image.NRGBA, State) {
	if r == nil {
		return nil, Absent
	}
	e, ok := r.Manifest.Images[key]
	if !ok || e.Width != w*Scale || e.Height != h*Scale {
		return r.Fallback.Lookup(key, w, h, load)
	}
	r.clock++
	if c, ok := r.cache[key]; ok {
		c.used = r.clock
		r.cache[key] = c
		if c.image != nil {
			return c.image, Ready
		}
		state := Failed
		if c.pending {
			state = Pending
		}
		if img, fallback := r.Fallback.Lookup(key, w, h, load); img != nil {
			return img, fallback
		}
		return nil, state
	}
	path := r.Root + "/" + e.Filename(key)
	c := cached{used: r.clock}
	var err error
	if r.Async && e.Format != "svg" {
		// Not cached while every worker is busy: the next repaint asks again.
		if !load || len(r.jobs) >= asyncWorkers {
			if img, fallback := r.Fallback.Lookup(key, w, h, load); img != nil {
				return img, fallback
			}
			return nil, Busy
		}
		job := &loadJob{key: key, entry: e, path: path, done: make(chan struct{})}
		local := r.localRoot()
		if !local {
			job.data, err = r.Read(path)
		}
		if err == nil {
			c.pending = true
			r.cache[key] = c
			r.jobs = append(r.jobs, job)
			go job.run(local)
			if img, fallback := r.Fallback.Lookup(key, w, h, load); img != nil {
				return img, fallback
			}
			return nil, Pending
		}
	} else {
		var data []byte
		data, err = r.Read(path)
		if err == nil && e.Format == "svg" {
			if len(data) > MaxImageBytes {
				err = fmt.Errorf("HD image exceeds size limit")
			} else if fmt.Sprintf("%x", sha256.Sum256(data)) != e.SHA256 {
				err = fmt.Errorf("HD image checksum mismatch")
			} else if r.SVG == nil {
				err = fmt.Errorf("SVG decoder unavailable")
			} else {
				var rgba []byte
				rgba, err = r.SVG(path, data, e.Width, e.Height)
				if err == nil && len(rgba) != e.Width*e.Height*4 {
					err = fmt.Errorf("invalid SVG pixel buffer")
				}
				if err == nil {
					c.image = &image.NRGBA{Pix: rgba, Stride: e.Width * 4, Rect: image.Rect(0, 0, e.Width, e.Height)}
				}
			}
		} else if err == nil {
			c.image, err = decodeEntry(e, data)
		}
	}
	r.store(key, c, err)
	if c.image == nil {
		if img, fallback := r.Fallback.Lookup(key, w, h, load); img != nil {
			return img, fallback
		}
		return nil, Failed
	}
	return c.image, Ready
}

// store records a finished image, or its failure, and trims the cache.
func (r *Reader) store(key string, c cached, err error) {
	if err != nil {
		c.image = nil
		r.Log(fmt.Sprintf("HD image %s: %v; using original", key, err))
	}
	if c.image != nil {
		r.bytes += len(c.image.Pix)
	}
	c.pending = false
	r.cache[key] = c
	for r.bytes > Budget || len(r.cache) > 128 {
		oldest := ""
		age := r.clock
		for k, v := range r.cache {
			if k != key && !v.pending && v.used < age {
				oldest, age = k, v.used
			}
		}
		if oldest == "" {
			break
		}
		if old := r.cache[oldest].image; old != nil {
			r.bytes -= len(old.Pix)
		}
		delete(r.cache, oldest)
	}
}

// Collect adopts images that finished decoding in the background, here and
// in the fallback pack. It reports whether anything arrived: the caller
// should then repaint, which also retries lookups that found workers busy.
func (r *Reader) Collect() bool {
	if r == nil {
		return false
	}
	changed := r.Fallback.Collect()
	pending := r.jobs[:0]
	for _, job := range r.jobs {
		select {
		case <-job.done:
			c := r.cache[job.key]
			c.image = job.image
			r.store(job.key, c, job.err)
			changed = true
		default:
			pending = append(pending, job)
		}
	}
	clear(r.jobs[len(pending):])
	r.jobs = pending
	return changed
}

// Loading reports how many images are still decoding in the background.
func (r *Reader) Loading() int {
	if r == nil {
		return 0
	}
	return len(r.jobs) + r.Fallback.Loading()
}
func (r *Reader) CachedBytes() int { return r.bytes }

func (e Entry) Filename(key string) string {
	ext := e.Format
	if ext == "" {
		ext = "png"
	}
	return "images/" + key + "." + ext
}
func (e Entry) DecodeConfig(r io.Reader) (image.Config, error) {
	if e.Format == "webp" {
		return webp.DecodeConfig(r)
	}
	return png.DecodeConfig(r)
}
func (e Entry) Decode(r io.Reader) (image.Image, error) {
	if e.Format == "webp" {
		return webp.Decode(r)
	}
	return png.Decode(r)
}
