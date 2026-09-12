package script

import (
	"math"
	"strconv"
	"strings"
)

type Value struct {
	Number   float64
	Text     string
	IsString bool
}

func Num(n float64) Value { return Value{Number: n} }
func Str(s string) Value  { return Value{Text: s, IsString: true} }
func Bool(v bool) Value {
	if v {
		return Num(1)
	}
	return Num(0)
}
func (v Value) Truthy() bool {
	if v.IsString {
		return len(v.Text) > 0
	}
	return v.Number != 0
}
func (v Value) Num() float64 {
	if !v.IsString {
		return v.Number
	}
	s := strings.TrimSpace(v.Text)
	end := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		end++
	}
	start := end
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == start {
		return 0
	}
	n, err := strconv.ParseFloat(s[:end], 64)
	if err != nil && !math.IsInf(n, 0) {
		return 0
	}
	return n
}
func (v Value) String() string {
	if v.IsString {
		return v.Text
	}
	n := v.Number
	if n == 0 {
		return "0"
	}
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	if a := math.Abs(n); a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(n, 'e', -1, 64)
		at := strings.IndexByte(s, 'e')
		mantissa, exponent := s[:at+2], s[at+2:]
		return mantissa + strings.TrimLeft(exponent, "0")
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}
func Equal(a, b Value) bool {
	if !a.IsString && !b.IsString {
		return a.Number == b.Number
	}
	return strings.ToLower(a.String()) == strings.ToLower(b.String())
}

// Values keeps the declaration order needed by indextoglobal().
type Values struct {
	data map[string]Value
	keys []string
}

func (v *Values) Get(name string) (Value, bool) { x, ok := v.data[strings.ToLower(name)]; return x, ok }
func (v *Values) Has(name string) bool          { _, ok := v.Get(name); return ok }
func (v *Values) Set(name string, value Value) {
	name = strings.ToLower(name)
	if v.data == nil {
		v.data = make(map[string]Value)
	}
	if _, ok := v.data[name]; !ok {
		v.keys = append(v.keys, name)
	}
	v.data[name] = value
}
func (v *Values) Delete(name string) {
	name = strings.ToLower(name)
	if !v.Has(name) {
		return
	}
	delete(v.data, name)
	for i, key := range v.keys {
		if key == name {
			v.keys = append(v.keys[:i], v.keys[i+1:]...)
			break
		}
	}
}
func (v *Values) Keys() []string { return append([]string(nil), v.keys...) }
func (v *Values) Len() int       { return len(v.keys) }
