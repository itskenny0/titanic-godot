package df

import "strings"

type PuppetDialogue struct {
	Ident                                    string
	Stance                                   int
	Text, Raw                                string
	AudioLocation, AnimLogicLocation, Record int
}
type PuppetLayer struct {
	Frames           []int
	AnchorY, AnchorX int
}
type PuppetStance struct {
	Location int
	Layers   []PuppetLayer
}
type PuppetScriptRef struct {
	Name     string
	Location int
}
type IdleTimer struct{ MinTicks, MaxTicks int }
type Puppet struct {
	File          *File `json:"-"`
	PaletteRaw    []byte
	Dialogue      map[string]PuppetDialogue
	DialogueOrder []string `json:"-"`
	Scripts       []PuppetScriptRef
	Stances       []PuppetStance
	BandLocation  int
	PupName       string
	IdleTimers    []IdleTimer
	Encoding      string
}
type PuppetAnimLayer struct{ Frame, Y, X int }
type PuppetAnimFrame struct{ Layers []PuppetAnimLayer }

func ReadPuppet(data []byte) (*Puppet, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	order := d.file.Order
	r := d.reader(0, order)
	p := &Puppet{File: d.file, PaletteRaw: r.bytesAt(58, 2048), Dialogue: map[string]PuppetDialogue{}, Scripts: []PuppetScriptRef{}, Stances: []PuppetStance{}, IdleTimers: []IdleTimer{}, Encoding: "macintosh"}
	count := r.table(r.i16At(2158), 2160, 312)
	for i := 0; i < count; i++ {
		at := 2160 + i*312
		line := PuppetDialogue{Ident: r.nameAt(at+280, 31), Stance: r.i16At(at), Raw: r.nameAt(at+24, 255), AudioLocation: r.i32At(at + 8), AnimLogicLocation: r.i32At(at + 12), Record: at}
		line.Text = DecodeMacRoman(line.Raw)
		key := strings.ToLower(line.Ident)
		if _, found := p.Dialogue[key]; !found {
			p.DialogueOrder = append(p.DialogueOrder, key)
		}
		p.Dialogue[key] = line
	}
	q := d.reader(2, order)
	count = max(0, min(q.i16At(22), (len(q.Data)-24)/40))
	for i := 0; i < count; i++ {
		at := 24 + i*40
		p.Scripts = append(p.Scripts, PuppetScriptRef{Name: strings.ToLower(q.nameAt(at+8, 31)), Location: q.i32At(at)})
	}
	readStance := func(loc int) *PuppetStance {
		data := d.file.Data(loc)
		if len(data) < 22+11*262 {
			return nil
		}
		s := &PuppetStance{Location: loc, Layers: []PuppetLayer{}}
		r := d.reader(loc, order)
		for i := 0; i < 11; i++ {
			at := 22 + i*262
			count := max(0, min(r.i16At(at), 32))
			l := PuppetLayer{Frames: []int{}, AnchorY: r.i16At(at + 2), AnchorX: r.i16At(at + 4)}
			for k := 0; k < count; k++ {
				l.Frames = append(l.Frames, r.i32At(at+6+k*4))
			}
			s.Layers = append(s.Layers, l)
		}
		return s
	}
	if Version(r.Data, nil) == 1 {
		if s := readStance(3); s != nil {
			p.Stances = append(p.Stances, *s)
		}
	} else {
		q := d.reader(3, order)
		for i := 0; i < 64; i++ {
			loc := q.i32At(22 + i*4)
			if loc <= 0 || loc >= len(d.file.Containers) {
				break
			}
			s := readStance(loc)
			if s == nil {
				break
			}
			p.Stances = append(p.Stances, *s)
		}
	}
	for i := 0; i < 4; i++ {
		p.IdleTimers = append(p.IdleTimers, IdleTimer{r.i32At(0x83a + i*4), r.i32At(0x84a + i*4)})
	}
	p.BandLocation = r.i32At(0x85a)
	p.PupName = r.PString(15)
	return p, d.err()
}
func (p *Puppet) AnimLogic(loc int) []PuppetAnimFrame {
	out := []PuppetAnimFrame{}
	data := p.File.Data(loc)
	if len(data) < 82 || len(data)%82 != 0 {
		return out
	}
	r := NewReader(data, p.File.Order)
	for at := 0; at < len(data); at += 82 {
		f := PuppetAnimFrame{Layers: make([]PuppetAnimLayer, 11)}
		r.Seek(at + 16)
		for i := range f.Layers {
			f.Layers[i] = PuppetAnimLayer{int(r.I16()), int(r.I16()), int(r.I16())}
		}
		out = append(out, f)
	}
	return out
}
