package df

import (
	"fmt"
	"strings"
)

type MovieRegion struct {
	Type, X0, Y0, X1, Y1 int
	Sound, Event, Target string
	Record               int
}
type MovieFrame struct {
	Type, Height, Width, LocationFrame                int
	Name, Sound, Event, Target                        string
	Regions                                           []MovieRegion
	HoldTicks                                         int
	WaitsForVoice, HoldsDeadline, PlaysThroughRegions bool
	LocationClickRegion, Record                       int
}
type MovieCue struct {
	Tick   int
	Target string
}
type MovieSegment struct {
	File                                  *File `json:"-"`
	Bias, Width, Height, OriginX, OriginY int
	PaletteRaw                            []byte
	Frames                                []MovieFrame
	ActionFrame1, ActionFrame2            string
	Flags                                 int
	KeySkips                              bool
	MinHoldTicks                          int
	AudioChunks                           []int
	AudioLoops                            bool
	Sounds                                map[string]int
	SoundFollows                          map[string]string
	SoundOrder                            []string `json:"-"`
	Cues                                  []MovieCue
}
type Movie struct{ Segments []MovieSegment }

func ReadMovie(data []byte) (*Movie, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	m := &Movie{Segments: []MovieSegment{}}
	seen := map[int]bool{}
	for bias := 0; bias >= 0 && bias < len(d.file.Containers) && !seen[bias]; {
		seen[bias] = true
		s, next, err := d.movieSegment(bias)
		if err != nil {
			return nil, err
		}
		m.Segments = append(m.Segments, s)
		if next == 0 {
			break
		}
		bias = next
	}
	return m, d.err()
}
func (d *assetDecoder) movieSegment(bias int) (MovieSegment, int, error) {
	order := d.file.Order
	r := d.reader(bias, order)
	s := MovieSegment{File: d.file, Bias: bias, Frames: []MovieFrame{}, AudioChunks: []int{}, Sounds: map[string]int{}, SoundFollows: map[string]string{}, Cues: []MovieCue{}}
	if Version(r.Data, order) != 4 {
		return s, 0, fmt.Errorf("unsupported MOV version %d", Version(r.Data, order))
	}
	s.Flags = r.i32At(0x18)
	s.KeySkips = s.Flags&1 != 0
	s.MinHoldTicks = r.i32At(0x1c)
	next := r.i32At(0x2c)
	s.OriginX = r.i16At(0x24)
	s.OriginY = r.i16At(0x26)
	s.ActionFrame1 = r.nameAt(0x40, 15)
	s.ActionFrame2 = r.nameAt(0x50, 15)
	audio, loop, cue := r.i32At(0x60), r.i32At(0x64), r.i32At(0x68)
	s.PaletteRaw = r.bytesAt(0x6c, 2048)
	s.Height = r.i16At(0x870)
	s.Width = r.i16At(0x872)
	n := r.table(r.i32At(0x878), 0x87c, 42)
	for i := 0; i < n; i++ {
		at := 0x87c + i*42
		f := MovieFrame{Type: 6, Height: r.i16At(at + 8), Width: r.i16At(at + 10), LocationFrame: r.i32At(at+12) + bias, Name: r.nameAt(at+26, 15), Record: at, Regions: []MovieRegion{}}
		logic := r.i32At(at + 16)
		if logic != 0 {
			f.LocationClickRegion = logic + bias
		}
		data := d.file.Data(f.LocationClickRegion)
		if f.LocationClickRegion != 0 && len(data) >= 0x42 {
			q := d.reader(f.LocationClickRegion, order)
			f.Type = q.i16At(0)
			f.HoldTicks = q.i32At(2)
			flags := data[6]
			f.WaitsForVoice = flags&1 != 0
			f.HoldsDeadline = flags&8 != 0
			f.PlaysThroughRegions = flags&4 != 0
			f.Sound = checkedName(data, 0x12, 15)
			f.Event = checkedName(data, 0x22, 15)
			f.Target = checkedName(data, 0x32, 15)
			if len(data) >= 1094 {
				count := max(0, min(q.i32At(1090), (len(data)-1094)/64))
				for j := 0; j < count; j++ {
					at := 1094 + j*64
					f.Regions = append(f.Regions, MovieRegion{Type: q.i16At(at), Y0: q.i16At(at + 8), X0: q.i16At(at + 10), Y1: q.i16At(at + 12), X1: q.i16At(at + 14), Sound: checkedName(data, at+16, 15), Event: checkedName(data, at+32, 15), Target: checkedName(data, at+48, 15), Record: at})
				}
			}
		}
		s.Frames = append(s.Frames, f)
	}
	if loop > 0 && loop+bias < len(d.file.Containers) {
		chunks, err := ReadLoopChunks(d.file.Data(loop+bias), order)
		if err != nil {
			return s, 0, err
		}
		for _, c := range chunks {
			s.AudioChunks = append(s.AudioChunks, c.ContainerLoc+bias)
		}
	}
	s.AudioLoops = len(s.AudioChunks) > 0
	if audio > 0 && audio+bias < len(d.file.Containers) {
		chunks, err := ReadOneShotChunks(d.file.Data(audio+bias), 15, 15, order)
		if err != nil {
			return s, 0, err
		}
		for _, c := range chunks {
			if c.Identifier == "" {
				continue
			}
			key := strings.ToLower(c.Identifier)
			if _, found := s.Sounds[key]; !found {
				s.SoundOrder = append(s.SoundOrder, key)
			}
			s.Sounds[key] = c.ContainerLoc + bias
			if c.Follow != "" {
				s.SoundFollows[key] = c.Follow
			}
		}
	}
	if cue > 0 && cue+bias < len(d.file.Containers) {
		data := d.file.Data(cue + bias)
		if len(data) >= 4 {
			q := d.reader(cue+bias, order)
			n := max(0, min(q.i32At(0), (len(data)-4)/28))
			for i := 0; i < n; i++ {
				at := 4 + i*28
				s.Cues = append(s.Cues, MovieCue{q.i32At(at), checkedName(data, at+12, 15)})
			}
		}
	}
	return s, next, d.err()
}
