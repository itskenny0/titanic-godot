package save

type SavedProp struct {
	Name    string  `json:"name"`
	View    string  `json:"view"`
	Owner   string  `json:"owner"`
	Visible bool    `json:"visible"`
	Is3d    bool    `json:"is3d"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Deg     float64 `json:"deg"`
	Dist    float64 `json:"dist"`
	Scale   float64 `json:"scale"`
	Value   float64 `json:"value"`
	Zclip   float64 `json:"zclip"`
}
type SavedLoop struct {
	Kind    string  `json:"kind"`
	Name    string  `json:"name"`
	Handler string  `json:"handler"`
	Period  float64 `json:"period"`
}
type SavedCricket struct {
	Name   string  `json:"name"`
	Set    string  `json:"set"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Base   float64 `json:"base"`
	Jitter float64 `json:"jitter"`
	Next   float64 `json:"next"`
}
type Waypoint struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Z   float64 `json:"z"`
	Cum float64 `json:"cum"`
}
type SavedWalk struct {
	Actor      string     `json:"actor"`
	Star       string     `json:"star"`
	Type       float64    `json:"type"`
	TurnTo     float64    `json:"turnTo"`
	Deg        float64    `json:"deg"`
	StartX     float64    `json:"startX"`
	StartY     float64    `json:"startY"`
	StartZ     float64    `json:"startZ"`
	DestX      float64    `json:"destX"`
	DestY      float64    `json:"destY"`
	DestZ      float64    `json:"destZ"`
	Progress   float64    `json:"progress"`
	Dist       float64    `json:"dist"`
	HasPayload bool       `json:"hasPayload"`
	Paused     bool       `json:"paused"`
	Path       []Waypoint `json:"path,omitempty"`
}
type SavedTheme struct {
	Track  string  `json:"track"`
	Volume float64 `json:"volume"`
	Extras float64 `json:"extras"`
}
type Placement struct {
	Set     string  `json:"set"`
	Star    string  `json:"star"`
	Pose    string  `json:"pose"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
	Deg     float64 `json:"deg"`
	Speed   float64 `json:"speed"`
	Turn    float64 `json:"turn"`
	Scale   float64 `json:"scale"`
	Zclip   float64 `json:"zclip"`
	Visible bool    `json:"visible"`
}
type SavedActor struct {
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	Value     float64   `json:"value"`
	Placement Placement `json:"placement"`
}
type State struct {
	Disk       string         `json:"disk"`
	Set        string         `json:"set"`
	Scene      string         `json:"scene"`
	View       string         `json:"view"`
	Frame      float64        `json:"frame"`
	Inventory  []SavedProp    `json:"inventory"`
	Actors     []SavedActor   `json:"actors"`
	Loops      []SavedLoop    `json:"loops"`
	Crickets   []SavedCricket `json:"crickets"`
	Walks      []SavedWalk    `json:"walks"`
	Theme      *SavedTheme    `json:"theme"`
	CastFiles  []string       `json:"castFiles"`
	TrackFiles []string       `json:"trackFiles"`
}

type Index struct{ Actors, Casts, Inventory, Shops, Tracks, TrackCount, Globals, Pool, Loops, Crickets, Walks int }
type Game struct {
	State
	Title, Stage, Clock, Hallside, Savedeck string
	Vars                                    []SavedVar
	NumGlobals                              map[string]float64
	StrGlobals                              map[string]string
	NumGlobalOrder, StrGlobalOrder          []string `json:"-"`
	Raw                                     *RawFile
	Index                                   Index
}
