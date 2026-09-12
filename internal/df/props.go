package df

import (
	"fmt"
	"slices"
	"strings"
)

type PropState struct {
	Identifier                                     string
	Location, Record                               int
	Records, Frames, RefScales, Degrees, PlayOrder []int
	Animated                                       bool
}
type PropGroup struct {
	Name                              string
	Location, ScriptContainerLocation int
	States                            []PropState
}
type Shop struct {
	File               *File `json:"-"`
	RefName            string
	MainScriptLocation int
	PaletteRaw         []byte
	Groups             []PropGroup
}

func ReadShop(data []byte) (*Shop, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	r := d.reader(0, nil)
	v := Version(r.Data, nil)
	if v != 1 && v != 4 {
		return nil, fmt.Errorf("unsupported SHP version %d", v)
	}
	s := &Shop{File: d.file, RefName: r.nameAt(2344), MainScriptLocation: r.i32At(2340), PaletteRaw: r.bytesAt(36, 2048), Groups: []PropGroup{}}
	n := r.table(r.i32At(2360), 2364, 16)
	for i := 0; i < n; i++ {
		s.Groups = append(s.Groups, d.propGroup(r.i32At(2364+i*16)))
	}
	return s, d.err()
}
func (d *assetDecoder) propGroup(loc int) PropGroup {
	r := d.reader(loc, nil)
	g := PropGroup{Name: r.nameAt(42, 47), Location: loc, ScriptContainerLocation: r.i32At(38), States: []PropState{}}
	n := r.table(r.i32At(90), 94, 32)
	for i := 0; i < n; i++ {
		at := 94 + i*32
		s := PropState{Identifier: r.nameAt(at+16, 15), Location: r.i32At(at), Record: at, Records: []int{}, Frames: []int{}, RefScales: []int{}, Degrees: []int{}}
		q := d.reader(s.Location, nil)
		count := q.table(q.i32At(114), 118, 44)
		degrees := map[int]bool{}
		dup := false
		for j := 0; j < count; j++ {
			rec := 118 + j*44
			degree, scale := q.i16At(rec+40), q.i16At(rec+42)
			if scale == 0 {
				scale = 96
			}
			s.Records = append(s.Records, rec)
			s.Frames = append(s.Frames, q.i32At(rec))
			s.Degrees = append(s.Degrees, degree)
			s.RefScales = append(s.RefScales, scale)
			if degrees[degree] {
				dup = true
			}
			degrees[degree] = true
		}
		orderCount := max(0, min(q.i16At(112), 33))
		order := make([]int, orderCount)
		valid := orderCount > 1
		for j := range order {
			order[j] = q.i16At(46+j*2) - 1
			if order[j] < 0 || order[j] >= count {
				valid = false
			}
		}
		if valid && (dup || orderCount >= count-1) {
			s.PlayOrder = order
		}
		s.Animated = s.PlayOrder != nil || dup
		g.States = append(g.States, s)
	}
	d.orientProps(g.States)
	return g
}
func (d *assetDecoder) frameSignature(loc int) [4]int {
	r := d.reader(loc, nil)
	return [4]int{int(r.I16()), int(r.I16()), int(r.I16()), int(r.I16())}
}
func (d *assetDecoder) orientProps(states []PropState) {
	settled := map[string][4]int{}
	for _, s := range states {
		if len(s.Frames) == 1 {
			settled[strings.ToLower(s.Identifier)] = d.frameSignature(s.Frames[0])
		}
	}
	for i := range states {
		s := &states[i]
		if !s.Animated || len(s.Frames) < 2 {
			continue
		}
		name := strings.ToLower(s.Identifier)
		opening := strings.HasPrefix(name, "open")
		prefix := 4
		if !opening {
			if !strings.HasPrefix(name, "close") {
				continue
			}
			prefix = 5
		}
		if len(name) <= prefix {
			continue
		}
		pose, ok := settled["idle"+name[prefix:]]
		if !ok {
			continue
		}
		first, last := 0, len(s.Frames)-1
		if s.PlayOrder != nil {
			first, last = s.PlayOrder[0], s.PlayOrder[len(s.PlayOrder)-1]
		}
		a, b := d.frameSignature(s.Frames[first]), d.frameSignature(s.Frames[last])
		if a == b || opening && a != pose || !opening && b != pose {
			continue
		}
		if s.PlayOrder != nil {
			slices.Reverse(s.PlayOrder)
		} else {
			slices.Reverse(s.Frames)
			slices.Reverse(s.RefScales)
			slices.Reverse(s.Degrees)
			slices.Reverse(s.Records)
		}
	}
}

type Sprite struct {
	Width, Height, PosYraw, PosXraw int
	Indexed, Opaque                 []byte
}

func DecodeSprite(data []byte) (Sprite, error) {
	r := NewReader(data, nil)
	s := Sprite{Height: int(r.I16()), Width: int(r.I16()), PosYraw: int(r.I16()), PosXraw: int(r.I16())}
	if r.Err != nil {
		return s, r.Err
	}
	if s.Width < 0 || s.Height < 0 || s.Width > 4096 || s.Height > 4096 {
		return s, fmt.Errorf("invalid sprite dimensions %dx%d", s.Width, s.Height)
	}
	n := s.Width * s.Height
	s.Indexed = make([]byte, n)
	s.Opaque = make([]byte, n)
	out := 0
	for row := 0; row < s.Height; row++ {
		length := int(r.I16())
		segment := r.Take(length)
		if r.Err != nil {
			return s, r.Err
		}
		q := NewReader(segment, nil)
		for q.Pos < len(segment) {
			flag := q.U8()
			count := int(flag >> 2)
			if count > n-out {
				return s, fmt.Errorf("sprite run exceeds image at row %d", row)
			}
			switch flag & 3 {
			case 1: // transparent run
			case 3:
				copy(s.Indexed[out:out+count], q.Take(count))
				for j := out; j < out+count; j++ {
					s.Opaque[j] = 1
				}
			case 2:
				color := q.U8()
				for j := out; j < out+count; j++ {
					s.Indexed[j] = color
					s.Opaque[j] = 1
				}
			case 0:
				if out < s.Width {
					return s, fmt.Errorf("sprite copies before first row")
				}
				copy(s.Indexed[out:out+count], s.Indexed[out-s.Width:out-s.Width+count])
				copy(s.Opaque[out:out+count], s.Opaque[out-s.Width:out-s.Width+count])
			}
			if q.Err != nil {
				return s, q.Err
			}
			out += count
		}
	}
	return s, nil
}
