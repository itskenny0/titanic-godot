package df

import (
	"encoding/binary"
	"fmt"
)

// Version reads the one header field whose offset differs on Macintosh files.
func Version(data []byte, order Order) int {
	if order == binary.BigEndian {
		if len(data) >= 2 {
			return int(binary.BigEndian.Uint16(data))
		}
		return 0
	}
	if len(data) < 6 {
		return 0
	}
	return int(int32(binary.LittleEndian.Uint32(data[2:])))
}

type assetDecoder struct {
	file    *File
	readers []*Reader
}

func openAsset(data []byte) (*assetDecoder, error) {
	f, err := ReadFile(data)
	if err != nil {
		return nil, err
	}
	if len(f.Containers) == 0 {
		return nil, fmt.Errorf("asset has no header")
	}
	return &assetDecoder{file: f}, nil
}
func (d *assetDecoder) reader(loc int, order Order) *Reader {
	r := NewReader(d.file.Data(loc), order)
	d.readers = append(d.readers, r)
	return r
}
func (d *assetDecoder) err() error {
	for _, r := range d.readers {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}
func (r *Reader) i32At(at int) int                  { r.Seek(at); return int(r.I32()) }
func (r *Reader) i16At(at int) int                  { r.Seek(at); return int(r.I16()) }
func (r *Reader) nameAt(at int, size ...int) string { r.Seek(at); return r.PString(size...) }
func (r *Reader) bytesAt(at, n int) []byte          { r.Seek(at); return r.Take(n) }
func (r *Reader) table(count, first, size int) int {
	if r.Err != nil {
		return 0
	}
	if count < 0 || first < 0 || first > len(r.Data) || size <= 0 || count > (len(r.Data)-first)/size {
		r.Err = fmt.Errorf("invalid asset table: %d records of %d bytes at %d in %d bytes", count, size, first, len(r.Data))
		return 0
	}
	return count
}
func clippedName(data []byte, at, limit int) string {
	if at < 0 || at >= len(data) {
		return ""
	}
	n := min(int(data[at]), limit, len(data)-at-1)
	return Latin1(data[at+1 : at+1+n])
}
func checkedName(data []byte, at, limit int) string {
	if at < 0 || at >= len(data) {
		return ""
	}
	n := int(data[at])
	if n < 1 || n > limit || n > len(data)-at-1 {
		return ""
	}
	for _, c := range data[at+1 : at+1+n] {
		if c < 32 || c > 126 {
			return ""
		}
	}
	return string(data[at+1 : at+1+n])
}
