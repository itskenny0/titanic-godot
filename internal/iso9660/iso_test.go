package iso9660

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func put32(b []byte, n uint32) {
	binary.LittleEndian.PutUint32(b, n)
	binary.BigEndian.PutUint32(b[4:], n)
}
func rec(name string, block, size uint32, flags byte) []byte {
	n := 33 + len(name)
	if n%2 != 0 {
		n++
	}
	b := make([]byte, n)
	b[0] = byte(n)
	put32(b[2:], block)
	put32(b[10:], size)
	b[25] = flags
	b[32] = byte(len(name))
	copy(b[33:], name)
	return b
}
func fixture() []byte {
	b := make([]byte, 32*sector)
	p := b[16*sector : 17*sector]
	p[0] = 1
	copy(p[1:], "CD001")
	p[6] = 1
	put32(p[80:], 32)
	binary.LittleEndian.PutUint16(p[128:], sector)
	binary.BigEndian.PutUint16(p[130:], sector)
	copy(p[156:], rec("\x00", 20, sector, 2))
	t := b[17*sector:]
	t[0] = 255
	copy(t[1:], "CD001")
	t[6] = 1
	copy(b[20*sector:], rec("DATA", 21, sector, 2))
	copy(b[21*sector:], rec("BOOTFILE.;1", 22, 4, 0))
	copy(b[22*sector:], "game")
	return b
}
func TestIndex(t *testing.T) {
	b := fixture()
	files, err := Index(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	e := files["data/bootfile"]
	if len(files) != 1 || e.Offset != 22*sector || e.Size != 4 || string(b[e.Offset:e.Offset+e.Size]) != "game" {
		t.Fatal(files)
	}
}
func TestRejectDamagedImages(t *testing.T) {
	for _, kind := range []string{"signature", "truncated", "endian", "outside", "cycle", "record", "duplicate", "multi"} {
		t.Run(kind, func(t *testing.T) {
			b := fixture()
			switch kind {
			case "signature":
				b[16*sector+1] = 0
			case "truncated":
				b = b[:18*sector]
			case "endian":
				b[16*sector+84]++
			case "outside":
				put32(b[21*sector+2:], 50)
			case "cycle":
				put32(b[20*sector+2:], 20)
			case "record":
				b[21*sector] = 10
			case "duplicate":
				copy(b[21*sector+int(b[21*sector]):], rec("BOOTFILE.;2", 22, 4, 0))
			case "multi":
				b[21*sector+25] = 128
			}
			if _, err := Index(bytes.NewReader(b), int64(len(b))); err == nil {
				t.Fatal("accepted damaged image")
			}
		})
	}
}
func TestDiscover(t *testing.T) {
	for _, names := range [][]string{{"Titanic_CD1_1996.iso", "my-cd2-copy.ISO"}, {"cd1.iso"}, {"cd1.iso", "copy-cd1.iso", "cd2.iso"}, {"cd1cd2.iso"}} {
		root := t.TempDir()
		for _, n := range names {
			os.WriteFile(filepath.Join(root, n), nil, 0600)
		}
		images, err := Discover(root)
		if len(names) == 2 {
			if err != nil || filepath.Base(images[0]) != names[0] || filepath.Base(images[1]) != names[1] {
				t.Fatal(images, err)
			}
		} else if err == nil {
			t.Fatal("accepted ambiguous or incomplete pair")
		}
	}
}
func TestOwnedDiscs(t *testing.T) {
	root := os.Getenv("TAOOT_ISO_DIR")
	if root == "" {
		t.Skip("set TAOOT_ISO_DIR for original disc checks")
	}
	images, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../godot/required_files.json")
	if err != nil {
		t.Fatal(err)
	}
	var required map[string][]string
	if err = json.Unmarshal(b, &required); err != nil {
		t.Fatal(err)
	}
	for i, name := range images {
		files, err := Open(name)
		if err != nil {
			t.Fatal(name, err)
		}
		disc := []string{"1", "2"}[i]
		for _, rel := range required[disc] {
			if files[rel].Size <= 0 {
				t.Fatalf("disc %s missing %s", disc, rel)
			}
		}
		t.Logf("disc %s: %d indexed files, all %d required files present", disc, len(files), len(required[disc]))
	}
}
