// Package patches downloads and imports the checksum-pinned M3tox patch archive.
package patches

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	resources "github.com/itskenny0/titanic-godot"
)

type File struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Version, URL, SHA256 string
	Files                map[string]File
}

func Pinned() Manifest {
	var m Manifest
	if err := json.Unmarshal([]byte(resources.PatchManifest()), &m); err != nil {
		panic(err)
	}
	return m
}

type Request struct {
	Archive string `json:"archive"`
	Target  string `json:"target"`
}
type Status struct {
	State string `json:"state"`
	Bytes int64  `json:"bytes"`
	Total int64  `json:"total"`
	Error string `json:"error"`
}
type Manager struct {
	mu     sync.Mutex
	status Status
	cancel context.CancelFunc
}

func (m *Manager) Status() Status { m.mu.Lock(); defer m.mu.Unlock(); return m.status }
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}
func (m *Manager) Start(request Request) error {
	if !filepath.IsAbs(request.Target) {
		return fmt.Errorf("patch cache must be an absolute path")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return fmt.Errorf("a patch operation is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.status = Status{State: "starting"}
	go func() {
		err := Install(ctx, request, Pinned(), func(state string, n, total int64) {
			m.mu.Lock()
			m.status = Status{State: state, Bytes: n, Total: total}
			m.mu.Unlock()
		})
		m.mu.Lock()
		defer m.mu.Unlock()
		cancel()
		m.cancel = nil
		if err != nil {
			m.status = Status{State: "error", Error: err.Error()}
		} else {
			m.status = Status{State: "ready"}
		}
	}()
	return nil
}

type Progress func(string, int64, int64)
type reader struct {
	ctx      context.Context
	source   io.Reader
	n, total int64
	phase    string
	progress Progress
}

func (r *reader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.source.Read(b)
	r.n += int64(n)
	r.progress(r.phase, r.n, r.total)
	return n, err
}
func hashFile(ctx context.Context, path string, total int64, progress Progress) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, &reader{ctx: ctx, source: f, total: total, phase: "verifying", progress: progress})
	return hex.EncodeToString(h.Sum(nil)), err
}

// Install never changes the supplied archive or any original game files.
// A complete, verified cache is published only after every member has passed.
func Install(ctx context.Context, request Request, manifest Manifest, progress Progress) error {
	if progress == nil {
		progress = func(string, int64, int64) {}
	}
	if !filepath.IsAbs(request.Target) {
		return fmt.Errorf("patch cache must be an absolute path")
	}
	if len(manifest.Files) == 0 || len(manifest.SHA256) != 64 {
		return fmt.Errorf("invalid patch manifest")
	}
	parent := filepath.Dir(request.Target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".patch-install-")
	if err != nil {
		return err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			os.RemoveAll(stage)
		}
	}()
	archive := request.Archive
	if archive == "" {
		archive = filepath.Join(stage, "archive.zip")
		if err := download(ctx, manifest.URL, archive, progress); err != nil {
			return err
		}
	}
	info, err := os.Stat(archive)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 512<<20 {
		return fmt.Errorf("select the M3tox 1.0.3 FULL ZIP (maximum 512 MiB)")
	}
	digest, err := hashFile(ctx, archive, info.Size(), progress)
	if err != nil {
		return err
	}
	if digest != manifest.SHA256 {
		return fmt.Errorf("patch ZIP checksum does not match M3tox 1.0.3 FULL; choose the original archive or download it again")
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) != len(manifest.Files) {
		return fmt.Errorf("unexpected patch archive contents")
	}
	payload := filepath.Join(stage, "files")
	if err := os.Mkdir(payload, 0700); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, entry := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		expected, ok := manifest.Files[entry.Name]
		if !ok || seen[entry.Name] || filepath.Base(entry.Name) != entry.Name || strings.ContainsAny(entry.Name, "\\:") || entry.Mode()&os.ModeSymlink != 0 || entry.UncompressedSize64 != uint64(expected.Size) {
			return fmt.Errorf("unexpected patch file: %s", entry.Name)
		}
		seen[entry.Name] = true
		if err := extract(ctx, entry, filepath.Join(payload, entry.Name), expected); err != nil {
			return err
		}
		progress("extracting", int64(i+1), int64(len(z.File)))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Preserve an existing cache until the replacement is safely in place.
	backup := filepath.Join(stage, "previous")
	hadOld := false
	if _, err := os.Lstat(request.Target); err == nil {
		if err := os.Rename(request.Target, backup); err != nil {
			return err
		}
		hadOld = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(payload, request.Target); err != nil {
		if hadOld {
			if restore := os.Rename(backup, request.Target); restore != nil {
				// Keep the backup if even restoring it fails.
				retained := request.Target + ".previous"
				if moveErr := os.Rename(backup, retained); moveErr != nil {
					keepStage = true
					retained = backup
				}
				return fmt.Errorf("cannot install or restore patch cache: %v; previous cache: %s", err, retained)
			}
		}
		return err
	}
	return nil
}
func extract(ctx context.Context, entry *zip.File, target string, expected File) error {
	src, err := entry.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	h := sha256.New()
	stream := &reader{ctx: ctx, source: io.LimitReader(src, expected.Size+1), progress: func(string, int64, int64) {}}
	n, err := io.Copy(io.MultiWriter(out, h), stream)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n != expected.Size || hex.EncodeToString(h.Sum(nil)) != expected.SHA256 {
		return fmt.Errorf("patch file checksum mismatch: %s", entry.Name)
	}
	return nil
}
func download(ctx context.Context, url, target string, progress Progress) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("patch download requires HTTPS")
	}
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Titanic-Godot/1.0")
	client := &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" {
			return fmt.Errorf("unsafe patch download redirect")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("patch download failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("patch download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > 512<<20 {
		return fmt.Errorf("patch download exceeds 512 MiB")
	}
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	src := &reader{ctx: ctx, source: io.LimitReader(response.Body, (512<<20)+1), total: response.ContentLength, phase: "downloading", progress: progress}
	n, err := io.Copy(file, src)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n > 512<<20 {
		return fmt.Errorf("patch download exceeds 512 MiB")
	}
	return nil
}
