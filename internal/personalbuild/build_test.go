package personalbuild

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/patches"
)

func fixture(t *testing.T) (Options, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	required := map[string][]string{
		"1": {"data/bootfile", "data/bedsit1.set", "data/main.stg", "data/ctl.stg"},
		"2": {"data/a14.set", "data/deckbd.set", "data/cargo.set"},
	}
	expected := map[string][]byte{}
	for disc, names := range required {
		for i, name := range names {
			data := make([]byte, 1536+i*4)
			binary.LittleEndian.PutUint32(data[4:], uint32(len(data)))
			binary.LittleEndian.PutUint32(data[20:], 1)
			rel := "cd" + disc + "/" + name
			dest := filepath.Join(root, "gamedata", strings.ToUpper(rel))
			if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, data, 0600); err != nil {
				t.Fatal(err)
			}
			expected[rel] = data
		}
	}
	os.WriteFile(filepath.Join(root, "gamedata", "my-save.ti"), []byte("precious save"), 0600)
	os.WriteFile(filepath.Join(root, "gamedata", "CD1", "DATA", "extra.set"), []byte("unrequested mod"), 0600)
	manifest, _ := json.Marshal(required)
	manifestPath := filepath.Join(root, "required.json")
	os.WriteFile(manifestPath, manifest, 0600)
	basePath := filepath.Join(root, "base.zip")
	f, _ := os.Create(basePath)
	z := zip.NewWriter(f)
	for name, value := range map[string]string{"Titanic.sh": "#!/bin/sh\n", "titanic/titanic.pck": "game frontend", "titanic/native/libtitanic.aarch64.so": "engine", "titanic/gamedata/PUT_GAME_FILES_HERE.txt": "import"} {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0644)
		if name == "Titanic.sh" {
			h.SetMode(0755)
		}
		w, _ := z.CreateHeader(h)
		w.Write([]byte(value))
	}
	z.Close()
	f.Close()
	return Options{Target: "portmaster", Base: basePath, GameData: filepath.Join(root, "gamedata"), Output: filepath.Join(root, "personal.zip"), Manifest: manifestPath, PatchArchive: fixturePatchArchive(t, root)}, expected
}

func TestPersonalPortMaster(t *testing.T) {
	options, expected := fixture(t)
	if err := build(options, fixturePatchManifest()); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(options.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	count := 0
	patchFound := false
	for _, entry := range z.File {
		if entry.Name == "titanic/patches/files/test.SET" {
			stream, _ := entry.Open()
			data, err := io.ReadAll(stream)
			stream.Close()
			if err != nil || string(data) != "synthetic patch" {
				t.Fatal("incorrect bundled patch")
			}
			patchFound = true
		}
		if entry.Name == "Titanic.sh" && entry.Mode().Perm() != 0755 {
			t.Fatal("launcher lost executable permissions")
		}
		if strings.Contains(entry.Name, ".ti") || strings.Contains(entry.Name, "extra.set") || strings.Contains(entry.Name, "PUT_GAME_FILES") {
			t.Fatalf("unwanted file: %s", entry.Name)
		}
		if strings.HasPrefix(entry.Name, "titanic/gamedata/") {
			stream, _ := entry.Open()
			data, err := io.ReadAll(stream)
			stream.Close()
			if err != nil {
				t.Fatal(err)
			}
			rel := strings.TrimPrefix(entry.Name, "titanic/gamedata/")
			if !bytes.Equal(data, expected[rel]) {
				t.Fatalf("changed asset %s", rel)
			}
			count++
		}
	}
	if !patchFound {
		t.Fatal("personal build lost patches from lean base")
	}
	if count != len(expected) {
		t.Fatalf("got %d files", count)
	}
	before, _ := os.ReadFile(options.Output)
	if err := build(options, fixturePatchManifest()); err == nil {
		t.Fatal("overwrote output")
	}
	after, _ := os.ReadFile(options.Output)
	if !bytes.Equal(before, after) {
		t.Fatal("existing output changed")
	}
	save, _ := os.ReadFile(filepath.Join(options.GameData, "my-save.ti"))
	if string(save) != "precious save" {
		t.Fatal("original save changed")
	}
}

func TestInventoryFailures(t *testing.T) {
	for _, scenario := range []string{"missing", "empty", "damaged", "ambiguous", "link", "traversal", "save"} {
		t.Run(scenario, func(t *testing.T) {
			o, _ := fixture(t)
			target := filepath.Join(o.GameData, "CD1", "DATA", "BOOTFILE")
			switch scenario {
			case "missing":
				os.Remove(target)
			case "empty":
				os.WriteFile(target, nil, 0600)
			case "damaged":
				os.WriteFile(target, make([]byte, 1536), 0600)
			case "ambiguous":
				os.WriteFile(filepath.Join(filepath.Dir(target), "bootfile"), []byte("duplicate"), 0600)
			case "link":
				os.Remove(target)
				if err := os.Symlink(o.Manifest, target); err != nil {
					t.Skip(err)
				}
			case "traversal", "save":
				rel := "../outside"
				if scenario == "save" {
					rel = "data/precious.ti"
				}
				data, _ := json.Marshal(map[string][]string{"1": {rel}, "2": {"data/cargo.set"}})
				os.WriteFile(o.Manifest, data, 0600)
			}
			if err := build(o, fixturePatchManifest()); err == nil {
				t.Fatal("accepted invalid input")
			}
			if _, err := os.Stat(o.Output); !os.IsNotExist(err) {
				t.Fatal("published a failed build")
			}
		})
	}
}

func TestArgs(t *testing.T) {
	var input bytes.Buffer
	args := []string{"--xr_mode_regular", "--use_immersive", "--", "--patches=all", "--game-data=/old"}
	binary.Write(&input, binary.LittleEndian, uint32(len(args)))
	for _, arg := range args {
		binary.Write(&input, binary.LittleEndian, uint32(len(arg)))
		input.WriteString(arg)
	}
	out, err := bundledArgs(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("/old")) || !bytes.Contains(out, []byte("--patches=all")) || !bytes.Contains(out, []byte("--game-data=res://gamedata")) {
		t.Fatal("lost or incorrect arguments")
	}
	again, err := bundledArgs(out)
	if err != nil || !bytes.Equal(out, again) {
		t.Fatal("arguments not stable")
	}
	for _, bad := range [][]byte{nil, {255, 255, 255, 255}, input.Bytes()[:10], append(input.Bytes(), 0)} {
		if _, err := bundledArgs(bad); err == nil {
			t.Fatal("accepted broken command line")
		}
	}
}

func TestAndroidRewrite(t *testing.T) {
	o, _ := fixture(t)
	assets, err := inventory(o.GameData, o.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	var base bytes.Buffer
	z := zip.NewWriter(&base)
	for name, value := range map[string][]byte{"assets/_cl_": {0, 0, 0, 0}, "META-INF/CERT.RSA": []byte("old signature"), "META-INF/services/needed": []byte("service"), "assets/patches/manifest.json": []byte("patch chooser")} {
		w, _ := z.Create(name)
		w.Write(value)
	}
	z.Close()
	os.WriteFile(o.Base, base.Bytes(), 0600)
	input, err := zip.OpenReader(o.Base)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var output bytes.Buffer
	if err := rewrite(input, &output, "android", assets, "", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	preserved := false
	for _, entry := range reader.File {
		if signature(entry.Name) {
			t.Fatal("kept stale signature")
		}
		if entry.Name == "META-INF/services/needed" {
			preserved = true
		}
		if strings.HasPrefix(entry.Name, "assets/gamedata/") && entry.Method != zip.Store {
			t.Fatal("Android game data must support direct seeks")
		}
	}
	if !preserved {
		t.Fatal("removed nonsignature metadata")
	}
}

func TestUnsafeBaseAndOutput(t *testing.T) {
	for _, scenario := range []string{"traversal", "duplicate", "bundled", "symlink", "inside-data"} {
		t.Run(scenario, func(t *testing.T) {
			o, _ := fixture(t)
			if scenario == "inside-data" {
				o.Output = filepath.Join(o.GameData, "personal.zip")
			} else {
				input, err := zip.OpenReader(o.Base)
				if err != nil {
					t.Fatal(err)
				}
				var data bytes.Buffer
				writer := zip.NewWriter(&data)
				for _, entry := range input.File {
					if err := writer.Copy(entry); err != nil {
						t.Fatal(err)
					}
				}
				input.Close()
				name := "../escape"
				switch scenario {
				case "duplicate":
					name = "Titanic.sh"
				case "bundled":
					name = "titanic/gamedata/cd1/data/bootfile"
				case "symlink":
					name = "titanic/unsafe"
				}
				header := &zip.FileHeader{Name: name}
				header.SetMode(0644)
				if scenario == "symlink" {
					header.SetMode(os.ModeSymlink | 0777)
				}
				dest, _ := writer.CreateHeader(header)
				dest.Write([]byte("bad"))
				writer.Close()
				os.WriteFile(o.Base, data.Bytes(), 0600)
			}
			if err := build(o, fixturePatchManifest()); err == nil {
				t.Fatal("accepted unsafe package or output")
			}
			if _, err := os.Stat(o.Output); !os.IsNotExist(err) {
				t.Fatal("published failed build")
			}
			matches, _ := filepath.Glob(filepath.Join(filepath.Dir(o.Base), ".titanic-personal-*"))
			if len(matches) != 0 {
				t.Fatal("left partial packages behind")
			}
		})
	}
}

func fixturePatchData() []byte {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	entry, _ := writer.Create("test.SET")
	entry.Write([]byte("synthetic patch"))
	writer.Close()
	return data.Bytes()
}
func fixturePatchManifest() patches.Manifest {
	return patches.Manifest{SHA256: fmt.Sprintf("%x", sha256.Sum256(fixturePatchData())), Files: map[string]patches.File{"test.SET": {Size: int64(len("synthetic patch")), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("synthetic patch")))}}}
}
func fixturePatchArchive(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "patch.zip")
	if err := os.WriteFile(path, fixturePatchData(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
