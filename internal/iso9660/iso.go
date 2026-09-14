// Package iso9660 reads the primary ISO9660 filesystem of original PC discs.
// Directory layout follows ECMA-119 (2nd edition), sections 8.4 and 9.1.
// It indexes file ranges without mounting, extracting, or buffering the image.
package iso9660

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const sector = 2048

type Entry struct {
	Offset int64 `json:"offset"`
	Size   int64 `json:"size"`
}

func Discover(root string) ([2]string, error) {
	var result [2]string
	entries, err := os.ReadDir(root)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if entry.IsDir() || !strings.HasSuffix(name, ".iso") {
			continue
		}
		one, two := strings.Contains(name, "cd1"), strings.Contains(name, "cd2")
		if !one && !two {
			continue
		}
		if one && two {
			return result, fmt.Errorf("ISO filename matches both cd1 and cd2: %s", entry.Name())
		}
		disc := 0
		if two {
			disc = 1
		}
		if result[disc] != "" {
			return result, fmt.Errorf("multiple ISOs match cd%d; keep one image for each disc in the folder", disc+1)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return result, fmt.Errorf("ISO must be a regular file: %s", entry.Name())
		}
		result[disc] = filepath.Join(root, entry.Name())
	}
	if (result[0] == "") != (result[1] == "") {
		return result, fmt.Errorf("select a folder with both cd1 and cd2 ISO images")
	}
	return result, nil
}

func Open(path string) (map[string]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("ISO must be a regular file")
	}
	return Index(f, info.Size())
}

func both32(b []byte) (int64, error) {
	v := binary.LittleEndian.Uint32(b)
	if v != binary.BigEndian.Uint32(b[4:]) {
		return 0, fmt.Errorf("inconsistent ISO byte orders")
	}
	return int64(v), nil
}

func record(b []byte, limit int64) (Entry, error) {
	if len(b) < 34 || int(b[0]) != len(b) || int(b[32])+33 > len(b) {
		return Entry{}, fmt.Errorf("invalid ISO directory record")
	}
	block, err := both32(b[2:10])
	if err != nil {
		return Entry{}, err
	}
	size, err := both32(b[10:18])
	if err != nil {
		return Entry{}, err
	}
	offset := (block + int64(b[1])) * sector
	if offset > limit || size > limit-offset {
		return Entry{}, fmt.Errorf("ISO file extends beyond image")
	}
	if b[26] != 0 || b[27] != 0 || b[25]&128 != 0 {
		return Entry{}, fmt.Errorf("interleaved or multi-extent ISO files are unsupported")
	}
	return Entry{offset, size}, nil
}

func Index(r io.ReaderAt, size int64) (map[string]Entry, error) {
	var primary []byte
	terminated := false
	for block := int64(16); block < 80; block++ {
		b := make([]byte, sector)
		if block*sector+sector > size {
			return nil, fmt.Errorf("truncated ISO volume descriptors")
		}
		if _, err := r.ReadAt(b, block*sector); err != nil {
			return nil, err
		}
		if string(b[1:6]) != "CD001" || b[6] != 1 {
			return nil, fmt.Errorf("not a supported ISO9660 image")
		}
		if b[0] == 1 {
			if primary != nil {
				return nil, fmt.Errorf("multiple primary ISO volumes")
			}
			primary = b
		}
		if b[0] == 255 {
			terminated = true
			break
		}
	}
	if primary == nil || !terminated {
		return nil, fmt.Errorf("missing ISO primary volume or terminator")
	}
	if binary.LittleEndian.Uint16(primary[128:]) != sector || binary.BigEndian.Uint16(primary[130:]) != sector {
		return nil, fmt.Errorf("ISO requires 2048-byte logical blocks")
	}
	blocks, err := both32(primary[80:88])
	if err != nil {
		return nil, err
	}
	limit := blocks * sector
	if limit > size || limit < 18*sector {
		return nil, fmt.Errorf("truncated ISO volume")
	}
	root, err := record(primary[156:190], limit)
	if err != nil {
		return nil, err
	}
	if primary[181]&2 == 0 {
		return nil, fmt.Errorf("ISO root is not a directory")
	}
	type directory struct {
		path  string
		entry Entry
		depth int
	}
	queue := []directory{{"", root, 0}}
	visited := map[int64]bool{}
	names := map[string]bool{}
	files := map[string]Entry{}
	var scanned int64
	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		scanned += d.entry.Size
		if visited[d.entry.Offset] || d.depth > 16 || len(visited) >= 4096 || d.entry.Size > 16<<20 || scanned > 32<<20 {
			return nil, fmt.Errorf("cyclic or excessively large ISO directory tree")
		}
		visited[d.entry.Offset] = true
		data := make([]byte, int(d.entry.Size))
		if _, err := r.ReadAt(data, d.entry.Offset); err != nil {
			return nil, err
		}
		for pos := 0; pos < len(data); {
			n := int(data[pos])
			if n == 0 {
				pos = (pos/sector + 1) * sector
				continue
			}
			if pos+n > len(data) || pos%sector+n > sector {
				return nil, fmt.Errorf("truncated ISO directory record")
			}
			b := data[pos : pos+n]
			pos += n
			e, err := record(b, limit)
			if err != nil {
				return nil, err
			}
			name := string(b[33 : 33+int(b[32])])
			if name == "\x00" || name == "\x01" || b[25]&4 != 0 {
				continue
			} // dot entries and associated Mac resource forks
			name = strings.ToLower(strings.TrimSuffix(strings.SplitN(name, ";", 2)[0], "."))
			if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
				return nil, fmt.Errorf("invalid ISO filename")
			}
			full := name
			if d.path != "" {
				full = d.path + "/" + name
			}
			if names[full] || len(names) >= 20000 {
				return nil, fmt.Errorf("ambiguous or excessive ISO entries: %s", full)
			}
			names[full] = true
			if b[25]&2 != 0 {
				queue = append(queue, directory{full, e, d.depth + 1})
			} else {
				files[full] = e
			}
		}
	}
	return files, nil
}
