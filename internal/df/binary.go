// Package df reads the DreamFactory formats used by Titanic.
// Ported from Daniel Hobi's dreamREfactory and M3tox's DFET. See CREDITS.md.
package df

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Order = binary.ByteOrder

func DetectOrder(data []byte) Order {
	if len(data) >= 8 && binary.LittleEndian.Uint32(data[4:]) != uint32(len(data)) && binary.BigEndian.Uint32(data[4:]) == uint32(len(data)) {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// Reader retains the first bounds error. Callers check Err before returning.
type Reader struct {
	Data  []byte
	Pos   int
	Order Order
	Err   error
}

func NewReader(data []byte, order Order) *Reader {
	if order == nil {
		order = binary.LittleEndian
	}
	return &Reader{Data: data, Order: order}
}

func (r *Reader) Take(n int) []byte {
	if r.Err != nil {
		return nil
	}
	if n < 0 || r.Pos < 0 || r.Pos > len(r.Data) || n > len(r.Data)-r.Pos {
		r.Err = fmt.Errorf("truncated DreamFactory data at %d: need %d bytes, have %d", r.Pos, n, len(r.Data))
		return nil
	}
	b := r.Data[r.Pos : r.Pos+n]
	r.Pos += n
	return b
}
func (r *Reader) U8() byte {
	b := r.Take(1)
	if b == nil {
		return 0
	}
	return b[0]
}
func (r *Reader) U16() uint16 {
	b := r.Take(2)
	if b == nil {
		return 0
	}
	return r.Order.Uint16(b)
}
func (r *Reader) I16() int16 { return int16(r.U16()) }
func (r *Reader) U32() uint32 {
	b := r.Take(4)
	if b == nil {
		return 0
	}
	return r.Order.Uint32(b)
}
func (r *Reader) I32() int32 { return int32(r.U32()) }
func (r *Reader) F64() float64 {
	b := r.Take(8)
	if b == nil {
		return 0
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}
func (r *Reader) F32() float32 {
	b := r.Take(4)
	if b == nil {
		return 0
	}
	return math.Float32frombits(binary.BigEndian.Uint32(b))
}
func (r *Reader) Seek(pos int) { r.Pos = pos }
func (r *Reader) Skip(n int)   { r.Take(n) }
func (r *Reader) PString(fieldSize ...int) string {
	n := int(r.U8())
	size := n
	if len(fieldSize) != 0 {
		size = fieldSize[0]
	}
	b := r.Take(size)
	if n > len(b) {
		if r.Err == nil {
			r.Err = fmt.Errorf("Pascal string length %d exceeds field %d", n, size)
		}
		return ""
	}
	return Latin1(b[:n])
}
func Latin1(b []byte) string {
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}
	return string(out)
}
