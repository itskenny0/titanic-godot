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
	image *image.NRGBA
	used  uint64
}

// SVGRenderer returns straight-alpha RGBA pixels at the requested dimensions.
// Godot supplies the decoder; headless Go callers can leave it unset.
type SVGRenderer func(string, []byte, int, int) ([]byte, error)

type Reader struct {
	SVG      SVGRenderer
	Fallback *Reader
	Manifest Manifest
	Root     string
	Read     func(string) ([]byte, error)
	Log      func(string)
	cache    map[string]cached
	clock    uint64
	bytes    int
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
func (r *Reader) fallback(key string, w, h int) *image.NRGBA {
	if r.Fallback != nil {
		return r.Fallback.Get(key, w, h)
	}
	return nil
}
func (r *Reader) Get(key string, w, h int) *image.NRGBA {
	e, ok := r.Manifest.Images[key]
	if !ok || e.Width != w*Scale || e.Height != h*Scale {
		return r.fallback(key, w, h)
	}
	r.clock++
	if c, ok := r.cache[key]; ok {
		c.used = r.clock
		r.cache[key] = c
		if c.image == nil {
			return r.fallback(key, w, h)
		}
		return c.image
	}
	c := cached{used: r.clock}
	data, err := r.Read(r.Root + "/" + e.Filename(key))
	if err == nil && len(data) > MaxImageBytes {
		err = fmt.Errorf("HD image exceeds size limit")
	}
	if err == nil && fmt.Sprintf("%x", sha256.Sum256(data)) != e.SHA256 {
		err = fmt.Errorf("HD image checksum mismatch")
	}
	var decoded image.Image
	if err == nil && e.Format == "svg" {
		if r.SVG == nil {
			err = fmt.Errorf("SVG decoder unavailable")
		} else {
			var rgba []byte
			rgba, err = r.SVG(r.Root+"/"+e.Filename(key), data, e.Width, e.Height)
			if err == nil && len(rgba) != e.Width*e.Height*4 {
				err = fmt.Errorf("invalid SVG pixel buffer")
			}
			if err == nil {
				decoded = &image.NRGBA{Pix: rgba, Stride: e.Width * 4, Rect: image.Rect(0, 0, e.Width, e.Height)}
			}
		}
	} else if err == nil {
		var cfg image.Config
		cfg, err = e.DecodeConfig(bytes.NewReader(data))
		if err == nil && (cfg.Width != e.Width || cfg.Height != e.Height) {
			err = fmt.Errorf("HD image dimensions do not match manifest")
		}
		if err == nil {
			decoded, err = e.Decode(bytes.NewReader(data))
		}
	}
	if err == nil {
		c.image = image.NewNRGBA(image.Rect(0, 0, e.Width, e.Height))
		draw.Draw(c.image, c.image.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
		r.bytes += len(c.image.Pix)
	} else {
		r.Log(fmt.Sprintf("HD image %s: %v; using original", key, err))
	}
	r.cache[key] = c
	for r.bytes > Budget || len(r.cache) > 128 {
		oldest := ""
		age := r.clock
		for k, v := range r.cache {
			if k != key && v.used < age {
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
	if c.image == nil {
		return r.fallback(key, w, h)
	}
	return c.image
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
