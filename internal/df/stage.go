package df

type Size struct{ Width, Height int }
type StageFlat struct {
	Condition, LocationScript, LocationFrame, LocationClickLogic, Width, Height int
	Name                                                                        string
	Record                                                                      int
}
type StageRegion struct {
	Top, Left, Bottom, Right, Script int
	Name                             string
	Record                           int
}
type Stage struct {
	File               *File `json:"-"`
	Version            int
	PaletteRaw         []byte
	RefName            string
	Screen             Size
	MainScriptLocation int
	Flats              []StageFlat
}

func ReadStage(data []byte) (*Stage, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	r := d.reader(0, nil)
	s := &Stage{File: d.file, Version: 4, Flats: []StageFlat{}}
	palette, screen, script, name, countAt, first, stride := 56, 40, 44, 2104, 2120, 2124, 46
	if Version(r.Data, nil) == 1 {
		s.Version = 1
		palette, screen, script, name, countAt, first, stride = 36, 28, 32, 2084, 2100, 2104, 28
	}
	s.PaletteRaw = r.bytesAt(palette, 2048)
	s.Screen = Size{r.i16At(screen), r.i16At(screen + 2)}
	s.MainScriptLocation = r.i32At(script)
	s.RefName = clippedName(r.Data, name, 15)
	count := r.table(r.i32At(countAt), first, stride)
	for i := 0; i < count; i++ {
		at := first + i*stride
		f := StageFlat{Record: at}
		if s.Version == 1 {
			f.LocationScript = r.i32At(at)
			f.LocationFrame = r.i32At(at + 4)
			f.LocationClickLogic = r.i32At(at + 8)
			f.Width = s.Screen.Width
			f.Height = s.Screen.Height
			f.Name = clippedName(r.Data, at+12, 15)
		} else {
			f.Condition = r.i32At(at)
			f.LocationScript = r.i32At(at + 6)
			f.LocationFrame = r.i32At(at + 10)
			f.LocationClickLogic = r.i32At(at + 14)
			f.Height = r.i16At(at + 22)
			f.Width = r.i16At(at + 24)
			f.Name = clippedName(r.Data, at+30, 15)
		}
		s.Flats = append(s.Flats, f)
	}
	return s, d.err()
}
func ReadStageRegions(data []byte, version int) []StageRegion {
	countAt, first := 1028, 1032
	if version == 1 {
		countAt, first = 0, 4
	}
	out := []StageRegion{}
	if len(data) < first {
		return out
	}
	r := NewReader(data, nil)
	count := r.table(r.i32At(countAt), first, 32)
	for i := 0; i < count; i++ {
		at := first + i*32
		out = append(out, StageRegion{Top: r.i16At(at + 4), Left: r.i16At(at + 6), Bottom: r.i16At(at + 8), Right: r.i16At(at + 10), Script: r.i32At(at + 12), Name: clippedName(data, at+16, 15), Record: at})
	}
	return out
}
