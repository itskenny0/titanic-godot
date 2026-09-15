package personalbuild

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/hdpack"
)

func personalHDFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{12, 34, 56, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	key := hdpack.Key(1, 1, []byte{1, 2, 3, 255})
	name := "images/" + key + ".png"
	if err := os.WriteFile(filepath.Join(root, name), b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	m := hdpack.Manifest{Version: 1, Scale: 2, Images: map[string]hdpack.Entry{key: {Width: 2, Height: 2, SHA256: fmt.Sprintf("%x", sha256.Sum256(b.Bytes()))}}}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "private-save.ti"), []byte("never bundle"), 0600)
	return root, name
}

func TestHDBundleAndValidation(t *testing.T) {
	for _, target := range []string{"android", "portmaster"} {
		t.Run(target, func(t *testing.T) {
			root, name := personalHDFixture(t)
			o, _ := fixture(t)
			before, _ := os.ReadFile(filepath.Join(root, name))
			if err := bundleHD(o.Base, target, root); err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(o.Base)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			prefix := "titanic/hdpack/"
			if target == "android" {
				prefix = "assets/hdpack/"
			}
			count := 0
			for _, f := range z.File {
				if strings.Contains(f.Name, "private-save") {
					t.Fatal("bundled unrelated file")
				}
				if !strings.HasPrefix(f.Name, prefix) {
					continue
				}
				count++
				if f.Name == prefix+name {
					r, _ := f.Open()
					data, e := io.ReadAll(r)
					r.Close()
					if e != nil || !bytes.Equal(data, before) {
						t.Fatal("image changed")
					}
				}
			}
			if count != 2 {
				t.Fatalf("got %d HD entries", count)
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte("corrupt"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := hdInventory(root); err == nil {
				t.Fatal("corrupt HD image accepted")
			}
		})
	}
}

func TestPersonalHDRejectsOldPlayer(t *testing.T) {
	o, _ := fixture(t)
	o.HDPack, _ = personalHDFixture(t)
	if err := build(o, fixturePatchManifest()); err == nil || !strings.Contains(err.Error(), "hdpack-support.json") {
		t.Fatalf("expected unsupported player error, got %v", err)
	}
	if _, err := os.Stat(o.Output); !os.IsNotExist(err) {
		t.Fatal("published an unsupported build")
	}
}

func TestPersonalCharacterPackRequiresCharacterSupport(t *testing.T) {
	root, _ := personalHDFixture(t)
	path := filepath.Join(root, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m hdpack.Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m.Characters = true
	data, _ = json.Marshal(m)
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"android", "portmaster"} {
		for _, supported := range []bool{false, true} {
			basePath := filepath.Join(t.TempDir(), "base.zip")
			f, err := os.Create(basePath)
			if err != nil {
				t.Fatal(err)
			}
			z := zip.NewWriter(f)
			marker := "titanic/hdpack-support.json"
			if target == "android" {
				marker = "assets/hdpack-support.json"
			}
			w, err := z.Create(marker)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(w, `{"version":1,"scale":2,"characters":%t}`, supported)
			if err = z.Close(); err != nil {
				t.Fatal(err)
			}
			f.Close()
			base, err := zip.OpenReader(basePath)
			if err != nil {
				t.Fatal(err)
			}
			err = validateHDPlayer(base, target, root)
			base.Close()
			if supported && err != nil || !supported && (err == nil || !strings.Contains(err.Error(), "does not support HD characters")) {
				t.Fatalf("%s support=%t: %v", target, supported, err)
			}
		}
	}
}

func TestPersonalHDPreservesGameAndPatches(t *testing.T) {
	o, _ := fixture(t)
	o.HDPack, _ = personalHDFixture(t)
	src, err := zip.OpenReader(o.Base)
	if err != nil {
		t.Fatal(err)
	}
	updated := o.Base + "-hd"
	f, err := os.Create(updated)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, entry := range src.File {
		if err := z.Copy(entry); err != nil {
			t.Fatal(err)
		}
	}
	marker, err := z.Create("titanic/hdpack-support.json")
	if err != nil {
		t.Fatal(err)
	}
	marker.Write([]byte(`{"version":1,"scale":2}`))
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	src.Close()
	o.Base = updated
	if err := build(o, fixturePatchManifest()); err != nil {
		t.Fatal(err)
	}
	result, err := zip.OpenReader(o.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	names := map[string]bool{}
	for _, entry := range result.File {
		names[entry.Name] = true
	}
	for _, name := range []string{"Titanic.sh", "titanic/gamedata/cd1/data/bootfile", "titanic/patches/files/test.SET", "titanic/hdpack/manifest.json"} {
		if !names[name] {
			t.Fatal("personal HD build lost", name)
		}
	}
}
