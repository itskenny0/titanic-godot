package df

import "fmt"

type FrameInfo struct {
	PosX, PosZ, PosY, AxisX                                           float64
	PosX16, PosZ16, PosY16, AxisX8                                    int
	MotionInfo, FrameContainerLoc, FramePairID, TransitionLog, ViewID int
}
type FrameRegister struct {
	Destination int
	Frames      []FrameInfo
}
type ObjectEntry struct {
	Rotation8, StartRegionX, StartRegionY, EndRegionX, EndRegionY, LocationScript int
	Identifier                                                                    string
	Record                                                                        int
}
type SceneView struct {
	Rotation                float64
	Rotation8, ViewPairType int
	CameraHeight            float64
	ViewID, LocationObjects int
	ViewName                string
	Objects                 []ObjectEntry
	Record                  int
}
type Scene struct {
	Index                                                               int
	SceneName                                                           string
	Record, XAxisMap, ZAxisMap, YAxisMap, LocationViews, LocationScript int
	SceneLocation                                                       [3]float64
	Views                                                               []SceneView
	Turns                                                               [2]FrameRegister
}
type Transition struct {
	LocationTransitionInfo, ViewIDstart, ViewIDend int
	Start, End                                     [3]float64
	TransitionName                                 string
	Waypoints                                      [][3]float64
	FrameRegisters                                 [2]FrameRegister
}
type Actor struct {
	Rotation8, PositionX, PositionZ, PositionY int
	Identifier                                 string
	Record, IDLimit                            int
}
type StarPath struct {
	A, B      string
	Container int
}
type StarPathPoint struct{ X, Y, Z, FromPrev int }
type Set struct {
	File                                                                                                                                    *File `json:"-"`
	Version, MainSceneRegister, TransitionRegister, ActorRegister                                                                           int
	SetName, DefaultSceneName, DefaultViewName                                                                                              string
	ViewPortWidth, ViewPortHeight, ZFarMax, ZLevelCount, MapLight, MapDark, MapWidth, MapHeight, SetDimensionsX, SetDimensionsY, MainScript int
	PaletteRaw                                                                                                                              []byte
	ColorCount                                                                                                                              int
	Scenes                                                                                                                                  []Scene
	Transitions                                                                                                                             []Transition
	Actors                                                                                                                                  []Actor
	StarPaths                                                                                                                               []StarPath
}

func (d *assetDecoder) frameRegister(loc int) FrameRegister {
	r := d.reader(loc, nil)
	count := r.table(r.i32At(4), 12, 60)
	out := FrameRegister{Destination: r.i32At(8), Frames: []FrameInfo{}}
	for i := 0; i < count; i++ {
		r.Seek(12 + i*60)
		out.Frames = append(out.Frames, FrameInfo{r.F64(), r.F64(), r.F64(), r.F64(), int(r.I16()), int(r.I16()), int(r.I16()), int(r.I16()), int(r.I32()), int(r.I32()), int(r.I32()), int(r.I32()), int(r.I32())})
	}
	return out
}
func (d *assetDecoder) objects(loc int) []ObjectEntry {
	out := []ObjectEntry{}
	if loc == 0 {
		return out
	}
	r := d.reader(loc, nil)
	count := r.table(r.i32At(0), 8, 36)
	for i := 0; i < count; i++ {
		at := 8 + i*36
		out = append(out, ObjectEntry{Rotation8: r.i16At(at + 4), StartRegionY: r.i16At(at + 8), StartRegionX: r.i16At(at + 10), EndRegionY: r.i16At(at + 12), EndRegionX: r.i16At(at + 14), LocationScript: r.i32At(at + 16), Identifier: r.nameAt(at+20, 15), Record: at})
	}
	return out
}
func (d *assetDecoder) scene(index, register int) Scene {
	r := d.reader(register, nil)
	at := index * 42
	s := Scene{Index: index, Record: at, XAxisMap: r.i16At(at + 4), ZAxisMap: r.i16At(at + 6), YAxisMap: r.i16At(at + 8), LocationViews: r.i32At(at + 10), LocationScript: r.i32At(at + 22), SceneName: r.nameAt(at + 26), Views: []SceneView{}}
	right, left := r.i32At(at+14), r.i32At(at+18)
	v := d.reader(s.LocationViews, nil)
	s.SceneLocation = [3]float64{v.F64(), v.F64(), v.F64()}
	count := v.table(v.i32At(48), 52, 46)
	for i := 0; i < count; i++ {
		at := 52 + i*46
		v.Seek(at)
		view := SceneView{Rotation: v.F64(), Rotation8: int(v.I16()), ViewPairType: int(v.I32()), CameraHeight: v.F64(), ViewID: int(v.I32()), LocationObjects: int(v.I32()), ViewName: v.PString(15), Record: at}
		view.Objects = d.objects(view.LocationObjects)
		s.Views = append(s.Views, view)
	}
	s.Turns = [2]FrameRegister{d.frameRegister(right), d.frameRegister(left)}
	return s
}
func (d *assetDecoder) transitions(loc int) []Transition {
	r := d.reader(loc, nil)
	count := r.table(r.i32At(4), 8, 16)
	out := []Transition{}
	for i := 0; i < count; i++ {
		at := 8 + i*16
		info, a, b := r.i32At(at), r.i32At(at+4), r.i32At(at+8)
		q := d.reader(info, nil)
		t := Transition{LocationTransitionInfo: info, ViewIDstart: q.i32At(6), ViewIDend: q.i32At(10), Waypoints: [][3]float64{}}
		q.Seek(14)
		t.Start = [3]float64{q.F64(), q.F64(), q.F64()}
		t.End = [3]float64{q.F64(), q.F64(), q.F64()}
		t.TransitionName = q.PString(15)
		n := q.table(int(q.I32()), 82, 24)
		for j := 0; j < n; j++ {
			t.Waypoints = append(t.Waypoints, [3]float64{q.F64(), q.F64(), q.F64()})
		}
		t.FrameRegisters = [2]FrameRegister{d.frameRegister(a), d.frameRegister(b)}
		out = append(out, t)
	}
	return out
}
func validStarID(s string) bool {
	if len(s) < 1 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func actorAt(r *Reader, at, limit int) Actor {
	r.Seek(at)
	return Actor{int(r.I16()), int(r.I16()), int(r.I16()), int(r.I16()), r.PString(), at, limit}
}
func (d *assetDecoder) actors(loc int) ([]Actor, []StarPath) {
	r := d.reader(loc, nil)
	count := r.table(r.i32At(0), 8, 54)
	actors, paths := []Actor{}, []StarPath{}
	for i := 0; i < count; i++ {
		at := 8 + i*54
		a, b := actorAt(r, at+4, 17), actorAt(r, at+30, 15)
		actors = append(actors, a)
		if validStarID(b.Identifier) && (b.PositionX != 0 || b.PositionZ != 0 || b.PositionY != 0) {
			actors = append(actors, b)
		}
		loc := r.i16At(at + 28)
		if loc != 0 && validStarID(a.Identifier) && validStarID(b.Identifier) {
			paths = append(paths, StarPath{a.Identifier, b.Identifier, loc})
		}
	}
	return actors, paths
}
func ReadSet(data []byte) (*Set, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	r := d.reader(0, nil)
	if Version(r.Data, nil) != 4 {
		return nil, fmt.Errorf("unsupported SET version %d", Version(r.Data, nil))
	}
	s := &Set{File: d.file, Version: 4, ColorCount: 128, Scenes: []Scene{}}
	s.MapLight = r.i32At(0x18)
	s.MapDark = r.i32At(0x1c)
	s.MapHeight = r.i16At(0x24)
	s.MapWidth = r.i16At(0x26)
	s.SetDimensionsY = r.i16At(0x2c)
	s.SetDimensionsX = r.i16At(0x2e)
	s.TransitionRegister = r.i32At(0x54)
	s.ActorRegister = r.i32At(0x58)
	s.MainScript = r.i32At(0x5c)
	s.MainSceneRegister = r.i32At(0x60)
	count := r.i32At(0x64)
	sceneTable := d.reader(s.MainSceneRegister, nil)
	count = sceneTable.table(count, 0, 42)
	s.SetName = r.nameAt(0x70)
	s.ViewPortWidth = r.i16At(0x84)
	s.ViewPortHeight = r.i16At(0x86)
	s.ZLevelCount = r.i16At(0x9fa)
	s.ZFarMax = r.i16At(0xa08)
	s.PaletteRaw = r.bytesAt(0xf2, 2048)
	s.DefaultSceneName = r.nameAt(0xa0e, 15)
	s.DefaultViewName = r.PString()
	for i := 0; i < count; i++ {
		s.Scenes = append(s.Scenes, d.scene(i, s.MainSceneRegister))
	}
	s.Transitions = d.transitions(s.TransitionRegister)
	s.Actors, s.StarPaths = d.actors(s.ActorRegister)
	return s, d.err()
}
func ReadStarPath(file *File, loc, version int) ([]StarPathPoint, error) {
	out := []StarPathPoint{}
	data := file.Data(loc)
	if data == nil {
		return out, nil
	}
	r := NewReader(data, nil)
	countAt, first := 8, 20
	if version == 1 {
		countAt, first = 0, 16
	}
	n := r.table(r.i32At(countAt), first, 8)
	for i := 0; i < n; i++ {
		r.Seek(first + i*8)
		x, z, y, dist := int(r.I16()), int(r.I16()), int(r.I16()), int(r.I16())
		out = append(out, StarPathPoint{x, y, z, dist})
	}
	return out, r.Err
}

type Road struct {
	Transition             *Transition
	Register, ArriveViewID int
}

func (s *Set) RoadsAt(viewID int) []Road {
	out := []Road{}
	for i := range s.Transitions {
		t := &s.Transitions[i]
		if t.ViewIDstart == viewID && len(t.FrameRegisters[0].Frames) > 0 {
			out = append(out, Road{t, 0, t.ViewIDend})
		} else if t.ViewIDend == viewID && len(t.FrameRegisters[1].Frames) > 0 {
			out = append(out, Road{t, 1, t.ViewIDstart})
		}
	}
	return out
}
func (s *Scene) TurnRing(view, dir int) (int, []FrameInfo) {
	if dir < 0 || dir > 1 {
		return view, nil
	}
	ring := s.Turns[dir].Frames
	from := -1
	for i, f := range ring {
		if f.ViewID == view && f.MotionInfo > 0 {
			from = i
			break
		}
	}
	if from < 0 {
		return view, nil
	}
	out := make([]FrameInfo, 0, len(ring))
	for n := 1; n <= len(ring); n++ {
		f := ring[(from+n)%len(ring)]
		out = append(out, f)
		if f.ViewID >= 0 && f.MotionInfo > 0 {
			return f.ViewID, out
		}
	}
	return view, out
}
