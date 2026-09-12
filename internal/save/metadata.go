package save

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const metadataMark = "TITANIC-MAC-SAVE\n"
const metadataLimit = 4 * 1024 * 1024

var ErrDamaged = errors.New("This restored save is incomplete or damaged. Your current game has not been replaced.")

type Global struct {
	Name  string
	Value any
}

func (g Global) MarshalJSON() ([]byte, error) { return json.Marshal([2]any{g.Name, g.Value}) }
func (g *Global) UnmarshalJSON(b []byte) error {
	var a []json.RawMessage
	if err := json.Unmarshal(b, &a); err != nil || len(a) != 2 {
		return ErrDamaged
	}
	if err := json.Unmarshal(a[0], &g.Name); err != nil {
		return ErrDamaged
	}
	return json.Unmarshal(a[1], &g.Value)
}

type Metadata struct {
	Version int      `json:"version"`
	Globals []Global `json:"globals"`
	State
}

func utf16Length(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}
func validString(v any) bool { s, ok := v.(string); return ok && utf16Length(s) <= 65536 }
func validNumber(v any) bool { n, ok := v.(float64); return ok && !math.IsNaN(n) && !math.IsInf(n, 0) }
func validFields(v any, ss, ns, bs string) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range strings.Fields(ss) {
		if !validString(m[k]) {
			return false
		}
	}
	for _, k := range strings.Fields(ns) {
		if !validNumber(m[k]) {
			return false
		}
	}
	for _, k := range strings.Fields(bs) {
		if _, ok := m[k].(bool); !ok {
			return false
		}
	}
	return true
}
func validArray(v any, check func(any) bool) bool {
	a, ok := v.([]any)
	if !ok || len(a) > 10000 {
		return false
	}
	for _, e := range a {
		if !check(e) {
			return false
		}
	}
	return true
}
func decodeMetadata(b []byte) (*Metadata, error) {
	if len(b) > metadataLimit || !utf8.Valid(b) {
		return nil, ErrDamaged
	}
	// TextDecoder removes an initial UTF-8 BOM in the existing save reader.
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	var v any
	if json.Unmarshal(b, &v) != nil || !validFields(v, "disk set scene view", "frame", "") {
		return nil, ErrDamaged
	}
	m := v.(map[string]any)
	if m["version"] != float64(1) || m["frame"].(float64) < 0 {
		return nil, ErrDamaged
	}
	names := map[string]bool{}
	if !validArray(m["globals"], func(v any) bool {
		a, ok := v.([]any)
		if !ok || len(a) != 2 || !validString(a[0]) {
			return false
		}
		name := a[0].(string)
		key := strings.ToLower(name)
		if name == "" || utf16Length(name) > 512 || strings.HasPrefix(name, "__") || names[key] {
			return false
		}
		names[key] = true
		return validString(a[1]) || validNumber(a[1])
	}) {
		return nil, ErrDamaged
	}
	if !validArray(m["inventory"], func(v any) bool {
		return validFields(v, "name view owner", "x y deg dist scale value zclip", "visible is3d")
	}) {
		return nil, ErrDamaged
	}
	if !validArray(m["actors"], func(v any) bool {
		return validFields(v, "name owner", "value", "") && validFields(v.(map[string]any)["placement"], "set star pose", "x y z deg speed turn scale zclip", "visible")
	}) {
		return nil, ErrDamaged
	}
	if !validArray(m["loops"], func(v any) bool {
		if !validFields(v, "kind name handler", "period", "") {
			return false
		}
		switch v.(map[string]any)["kind"] {
		case "actor", "prop", "scene", "flat":
			return true
		}
		return false
	}) {
		return nil, ErrDamaged
	}
	if !validArray(m["crickets"], func(v any) bool { return validFields(v, "name set", "x y radius base jitter next", "") }) {
		return nil, ErrDamaged
	}
	if !validArray(m["walks"], func(v any) bool {
		if !validFields(v, "actor star", "type turnTo deg startX startY startZ destX destY destZ progress dist", "hasPayload paused") {
			return false
		}
		w := v.(map[string]any)
		switch w["type"] {
		case float64(0), float64(1), float64(3):
		default:
			return false
		}
		if p, ok := w["path"]; ok {
			if !validArray(p, func(v any) bool { return validFields(v, "", "x y z cum", "") }) {
				return false
			}
		}
		if w["type"] == float64(3) {
			p, ok := w["path"].([]any)
			if !ok || len(p) < 2 {
				return false
			}
		}
		return true
	}) {
		return nil, ErrDamaged
	}
	if !validArray(m["castFiles"], validString) || !validArray(m["trackFiles"], validString) {
		return nil, ErrDamaged
	}
	if theme, ok := m["theme"]; !ok || theme != nil && !validFields(theme, "track", "volume extras", "") {
		return nil, ErrDamaged
	}
	var metadata Metadata
	if json.Unmarshal(b, &metadata) != nil {
		return nil, ErrDamaged
	}
	return &metadata, nil
}
func split(b []byte) ([]byte, *Metadata, error) {
	if len(b) < 8 {
		return b, nil, nil
	}
	size := uint64(binary.LittleEndian.Uint32(b[4:]))
	mark := []byte(metadataMark)
	marked := func(at int) bool { return at >= 0 && at <= len(b)-len(mark) && bytes.Equal(b[at:at+len(mark)], mark) }
	start := false
	if size <= uint64(len(b)) {
		start = marked(int(size))
	}
	end := marked(len(b) - len(mark))
	if size >= dataStart && size < uint64(len(b)) {
		remainder := len(b) - int(size)
		if remainder < len(mark) && bytes.Equal(b[int(size):], mark[:remainder]) {
			return nil, nil, ErrDamaged
		}
	}
	if !start && !end {
		return b, nil, nil
	}
	if !start || !end || size < dataStart || uint64(len(b)) > size+metadataLimit+uint64(len(mark)*2+8) {
		return nil, nil, ErrDamaged
	}
	footer := len(b) - len(mark) - 8
	if footer < int(size)+len(mark) {
		return nil, nil, ErrDamaged
	}
	length := uint64(binary.LittleEndian.Uint32(b[footer:]))
	if length > metadataLimit || size+uint64(len(mark))+length != uint64(footer) || crc32.ChecksumIEEE(b[:footer]) != binary.LittleEndian.Uint32(b[footer+4:]) {
		return nil, nil, ErrDamaged
	}
	metadata, err := decodeMetadata(b[int(size)+len(mark) : footer])
	if err != nil {
		return nil, nil, err
	}
	return b[:int(size)], metadata, nil
}
func AppendMetadata(b []byte, m Metadata) ([]byte, error) {
	raw, _, err := split(b)
	if err != nil {
		return nil, err
	}
	if _, err = ParseLegacy(raw); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	if _, err = decodeMetadata(payload); err != nil {
		return nil, err
	}
	footer := len(raw) + len(metadataMark) + len(payload)
	out := make([]byte, footer+8+len(metadataMark))
	copy(out, raw)
	copy(out[len(raw):], metadataMark)
	copy(out[len(raw)+len(metadataMark):], payload)
	binary.LittleEndian.PutUint32(out[footer:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(out[footer+4:], crc32.ChecksumIEEE(out[:footer]))
	copy(out[footer+8:], metadataMark)
	return out, nil
}
func Parse(b []byte) (*Game, error) {
	raw, m, err := split(b)
	if err != nil {
		return nil, err
	}
	g, err := ParseLegacy(raw)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return g, nil
	}
	g.State = m.State
	g.NumGlobals = map[string]float64{}
	g.StrGlobals = map[string]string{}
	g.NumGlobalOrder, g.StrGlobalOrder = nil, nil
	for _, v := range m.Globals {
		switch value := v.Value.(type) {
		case string:
			g.StrGlobals[v.Name] = value
			g.StrGlobalOrder = append(g.StrGlobalOrder, v.Name)
		case float64:
			g.NumGlobals[v.Name] = value
			g.NumGlobalOrder = append(g.NumGlobalOrder, v.Name)
		}
	}
	g.Hallside = g.StrGlobals["hallside"]
	g.Savedeck = g.StrGlobals["savedeck"]
	g.Clock = g.StrGlobals["clock"]
	if _, ok := g.StrGlobals["clock"]; !ok {
		if n, ok := g.NumGlobals["clock"]; ok {
			g.Clock = numberString(n)
		}
	}
	return g, nil
}

func numberString(n float64) string {
	if n == 0 {
		return "0"
	}
	if a := math.Abs(n); a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(n, 'e', -1, 64)
		at := strings.IndexByte(s, 'e')
		return s[:at+2] + strings.TrimLeft(s[at+2:], "0")
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}
