package engine

import (
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"sync/atomic"
	"unicode/utf16"
)

var drawID atomic.Uint64

func newDrawID() uint64 { return drawID.Add(1) }

// DrawSignature tracks visual state without serializing the scene each frame.
// Identity values belong to loaded assets and do not keep unloaded assets alive.
type DrawSignature struct{ Lo, Hi uint32 }

func (s *DrawSignature) Reset() *DrawSignature { s.Lo = 0x811c9dc5; s.Hi = 0x9e3779b9; return s }
func (s *DrawSignature) word(v uint32) {
	s.Lo = (s.Lo ^ v) * 0x01000193
	s.Hi = (s.Hi + v) * 0x85ebca6b
	s.Hi ^= s.Hi >> 15
}
func (s *DrawSignature) Num(v float64) *DrawSignature {
	if float64(int32JS(v)) == v {
		s.word(uint32(int32JS(v)))
	} else {
		bits := math.Float64bits(v)
		s.word(uint32(bits))
		s.word(uint32(bits >> 32))
	}
	return s
}
func (s *DrawSignature) Bool(v bool) *DrawSignature {
	if v {
		return s.Num(1)
	}
	return s.Num(2)
}
func (s *DrawSignature) Str(v string) *DrawSignature {
	n := 0
	for _, r := range v {
		if r > 0xffff {
			n += 2
		} else {
			n++
		}
	}
	s.word(uint32(n) ^ 0x5bf03fd7)
	for _, r := range v {
		if r > 0xffff {
			a, b := utf16.EncodeRune(r)
			s.word(uint32(a))
			s.word(uint32(b))
		} else {
			s.word(uint32(r))
		}
	}
	return s
}
func (s *DrawSignature) Value(v script.Value) *DrawSignature {
	if v.IsString {
		return s.Str(v.Text)
	}
	return s.Num(v.Number)
}
func (s *DrawSignature) ID(id uint64) *DrawSignature { return s.Num(float64(id)) }
