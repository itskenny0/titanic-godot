// Package save reads original Titanic saves and the Mac-compatible restoration
// trailer. It never writes a file or changes the caller's input buffers.
package save

import (
	"encoding/binary"
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"slices"
	"unicode/utf16"
)

const dataStart = 1536
const signature = "ODTRTRFD"

type RawFile struct {
	Header, Table []byte
	Containers    []df.Container
}

func ReadRaw(b []byte) (*RawFile, error) {
	if len(b) < dataStart || binary.LittleEndian.Uint32(b) != 0x10000 || string(b[32:40]) != signature {
		return nil, fmt.Errorf("not a Titanic save file")
	}
	n := int(binary.LittleEndian.Uint32(b[20:]))
	if n < 0 || n > (dataStart-df.HeaderSize)/4 {
		return nil, fmt.Errorf("invalid save container count %d", n)
	}
	raw := &RawFile{Header: slices.Clone(b[:df.HeaderSize]), Table: slices.Clone(b[df.HeaderSize:dataStart]), Containers: make([]df.Container, n)}
	for i := range raw.Containers {
		p := int(binary.LittleEndian.Uint32(b[df.HeaderSize+i*4:]))
		if p <= df.HeaderSize {
			raw.Containers[i] = df.Container{ID: int32(i), Data: []byte{}}
			continue
		}
		if p > len(b)-8 {
			return nil, fmt.Errorf("save container %d is outside the file", i)
		}
		c, err := df.ReadContainer(b, p, binary.LittleEndian)
		if err != nil {
			return nil, err
		}
		c.Data = slices.Clone(c.Data)
		raw.Containers[i] = c
	}
	return raw, nil
}
func WriteRaw(raw *RawFile) ([]byte, error) {
	if raw == nil || len(raw.Containers) > (dataStart-df.HeaderSize)/4 {
		return nil, fmt.Errorf("invalid save container table")
	}
	positions := make([]int, len(raw.Containers))
	size := dataStart
	for i, c := range raw.Containers {
		if len(c.Data) > 1<<30 || size > 1<<30-len(c.Data)-71 {
			return nil, fmt.Errorf("save is too large")
		}
		positions[i] = size
		size = (size + 8 + len(c.Data) + 63) / 64 * 64
	}
	out := make([]byte, size)
	copy(out[:df.HeaderSize], raw.Header)
	copy(out[df.HeaderSize:dataStart], raw.Table)
	binary.LittleEndian.PutUint32(out[4:], uint32(size))
	binary.LittleEndian.PutUint32(out[20:], uint32(len(raw.Containers)))
	for i, c := range raw.Containers {
		p := positions[i]
		binary.LittleEndian.PutUint32(out[df.HeaderSize+i*4:], uint32(p))
		binary.LittleEndian.PutUint32(out[p:], uint32(c.ID))
		binary.LittleEndian.PutUint32(out[p+4:], uint32(len(c.Data)))
		copy(out[p+8:], c.Data)
	}
	return out, nil
}
func NeutralTemplate() []byte {
	raw := &RawFile{Header: make([]byte, 1024), Table: make([]byte, 512)}
	binary.LittleEndian.PutUint32(raw.Header, 0x10000)
	copy(raw.Header[32:], signature)
	for i, size := range []int{272, 786, 0, 0, 0, 0, 0, 28, 0, 32 * 42, 16 * 74, 16 * 110} {
		raw.Containers = append(raw.Containers, df.Container{ID: int32(i), Data: make([]byte, size)})
	}
	writeField(raw.Containers[0].Data, 0, "Titanic 1.0", 15)
	writeField(raw.Containers[1].Data, 520, "main.stg", 15)
	b, err := WriteRaw(raw)
	if err != nil {
		panic(err)
	}
	return b
}

type bytesView []byte

func (b bytesView) u16(at int) uint16 {
	if at < 0 || at > len(b)-2 {
		return 0
	}
	return binary.LittleEndian.Uint16(b[at:])
}
func (b bytesView) i16(at int) float64 { return float64(int16(b.u16(at))) }
func (b bytesView) u32(at int) uint32 {
	if at < 0 || at > len(b)-4 {
		return 0
	}
	return binary.LittleEndian.Uint32(b[at:])
}
func (b bytesView) i32(at int) float64 { return float64(int32(b.u32(at))) }
func checkedString(b []byte, at, minLen, maxLen int) *string {
	if at < 0 || at >= len(b) {
		return nil
	}
	n := int(b[at])
	if n < minLen || n > maxLen || n > len(b)-at-1 {
		return nil
	}
	for _, c := range b[at+1 : at+1+n] {
		if c < 32 || c > 126 {
			return nil
		}
	}
	s := string(b[at+1 : at+1+n])
	return &s
}
func field(b []byte, at int) string {
	if s := checkedString(b, at, 1, 40); s != nil {
		return *s
	}
	return ""
}
func writeField(b []byte, at int, s string, limit int) {
	chars := utf16.Encode([]rune(s))
	n := min(len(chars), limit)
	if at < 0 || at >= len(b) {
		return
	}
	b[at] = byte(n)
	for i := 0; i < n && at+1+i < len(b); i++ {
		b[at+1+i] = byte(chars[i])
	}
}
