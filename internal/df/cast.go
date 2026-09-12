package df

import "strings"

type CastFrame struct{ Location, Direction, Angle, RefScale, Record int }
type CastPose struct {
	Name               string
	Location           int
	Steps              [][]CastFrame
	Play               []int
	FrameCount, Record int
}
type CastMember struct {
	Name                          string
	LogicLocation, ScriptLocation int
	Poses                         []CastPose
}
type Cast struct {
	File               *File `json:"-"`
	PaletteRaw         []byte
	MainScriptLocation int
	Members            []CastMember
}

func ReadCast(data []byte) (*Cast, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	r := d.reader(0, nil)
	c := &Cast{File: d.file, PaletteRaw: r.bytesAt(36, 2048), MainScriptLocation: r.i32At(0x924), Members: []CastMember{}}
	n := r.table(r.i32At(0x938), 0x93c, 16)
	for i := 0; i < n; i++ {
		loc := r.i32At(0x93c + i*16)
		q := d.reader(loc, nil)
		m := CastMember{Name: strings.ToLower(q.nameAt(0x2a)), LogicLocation: loc, ScriptLocation: q.i32At(0x26), Poses: []CastPose{}}
		poses := q.table(q.i32At(0x5a), 0x5e, 32)
		for j := 0; j < poses; j++ {
			at := 0x5e + j*32
			p := CastPose{Name: strings.ToLower(q.nameAt(at+16, 15)), Location: q.i32At(at), Record: at, Steps: [][]CastFrame{}, Play: []int{}}
			v := d.reader(p.Location, nil)
			p.FrameCount = v.table(v.i32At(0x72), 0x76, 44)
			for k := 0; k < p.FrameCount; k++ {
				rec := 0x76 + k*44
				f := CastFrame{Location: v.i32At(rec), Direction: v.i16At(rec + 10), Angle: v.i16At(rec + 40), RefScale: v.i16At(rec + 42), Record: rec}
				step := v.i16At(rec + 8)
				if step < 0 {
					continue
				}
				for len(p.Steps) <= step {
					p.Steps = append(p.Steps, []CastFrame{})
				}
				p.Steps[step] = append(p.Steps[step], f)
			}
			playCount := max(0, min(v.i16At(0x70), 33))
			valid := playCount > 0
			for k := 0; k < playCount; k++ {
				step := v.i16At(0x2e+k*2) - 1
				p.Play = append(p.Play, step)
				if step < 0 || step >= len(p.Steps) {
					valid = false
				}
			}
			if !valid {
				p.Play = make([]int, len(p.Steps))
				for k := range p.Play {
					p.Play[k] = k
				}
			}
			m.Poses = append(m.Poses, p)
		}
		c.Members = append(c.Members, m)
	}
	return c, d.err()
}
