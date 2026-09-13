package patches

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPSDownload(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.NotFound(w, r)
		case "/redirect":
			http.Redirect(w, r, "http://example.invalid/patch.zip", http.StatusFound)
		default:
			w.Write([]byte("test archive"))
		}
	}))
	defer server.Close()
	// Trust only this test server's certificate. Production uses system roots.
	transport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = transport }()
	for _, scenario := range []string{"success", "missing", "redirect", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancel" {
				cancel()
			}
			target := filepath.Join(t.TempDir(), "archive.zip")
			var received int64
			err := download(ctx, server.URL+"/"+scenario, target, func(_ string, n, _ int64) { received = n })
			if scenario == "success" {
				data, readErr := os.ReadFile(target)
				if err != nil || readErr != nil || string(data) != "test archive" || received != int64(len(data)) {
					t.Fatalf("download: %q, %v, %v, bytes %d", data, err, readErr, received)
				}
			} else if err == nil {
				t.Fatal("accepted failed, unsafe or cancelled download")
			}
		})
	}
}

func TestInstallVerifiesBeforeReplacingCache(t *testing.T) {
	for _, failure := range []string{"none", "archive checksum", "member checksum", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			var data bytes.Buffer
			w := zip.NewWriter(&data)
			entry, err := w.Create("test.SET")
			if err != nil {
				t.Fatal(err)
			}
			content := []byte("synthetic patch")
			if _, err := entry.Write(content); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(root, "patch.zip")
			if err := os.WriteFile(archive, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "cache")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			old := filepath.Join(target, "original")
			if err := os.WriteFile(old, []byte("keep me"), 0600); err != nil {
				t.Fatal(err)
			}
			manifest := Manifest{
				SHA256: fmt.Sprintf("%x", sha256.Sum256(data.Bytes())),
				Files:  map[string]File{"test.SET": {Size: int64(len(content)), SHA256: fmt.Sprintf("%x", sha256.Sum256(content))}},
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch failure {
			case "archive checksum":
				manifest.SHA256 = fmt.Sprintf("%064d", 0)
			case "member checksum":
				manifest.Files["test.SET"] = File{Size: int64(len(content)), SHA256: fmt.Sprintf("%064d", 0)}
			case "cancelled":
				cancel()
			}
			err = Install(ctx, Request{Archive: archive, Target: target}, manifest, nil)
			if failure == "none" {
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(target, "test.SET"))
				if err != nil || !bytes.Equal(got, content) {
					t.Fatalf("installed content: %q, %v", got, err)
				}
			} else {
				if err == nil {
					t.Fatal("invalid installation succeeded")
				}
				got, err := os.ReadFile(old)
				if err != nil || string(got) != "keep me" {
					t.Fatalf("existing cache changed: %q, %v", got, err)
				}
			}
			got, err := os.ReadFile(archive)
			if err != nil || !bytes.Equal(got, data.Bytes()) {
				t.Fatal("input archive changed")
			}
			stages, err := filepath.Glob(filepath.Join(root, ".patch-install-*"))
			if err != nil || len(stages) != 0 {
				t.Fatalf("temporary files remain: %v, %v", stages, err)
			}
		})
	}
}
