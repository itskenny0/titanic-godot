package engine

import "math"

type WorldCamera struct {
	X, Y, Z, Deg, F, CX, CY float64
	ClipW, ClipH            int
}
type Projection struct{ X, Y, Depth float64 }
type Occlusion struct {
	Z                         []byte
	W, H                      int
	Scale, Levels, GroundBias float64
}

var sin14, cos14 [256]float64

func init() {
	for i := range sin14 {
		angle := 2 * math.Pi * float64(i) / 256
		sin14[i], cos14[i] = jsRound(16384*math.Sin(angle)), jsRound(16384*math.Cos(angle))
	}
}
func jsRound(v float64) float64 { return math.Floor(v + .5) }

// Script geometry uses signed 32-bit fixed-point operations, including overflow.
func int32JS(v float64) int32 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	v = math.Mod(math.Trunc(v), 4294967296)
	if v < 0 {
		v += 4294967296
	}
	return int32(uint32(v))
}
func fix14(v float64) float64 {
	if v < 0 {
		v += 16383
	}
	return float64(int32JS(v) >> 14)
}
func ProjectPoint(cam WorldCamera, x, y, z float64) *Projection {
	dx, dy, dz := x-cam.X, y-cam.Y, z-cam.Z
	idx := int32JS(cam.Deg) & 255
	s, c := sin14[idx], cos14[idx]
	depth := fix14(dy*s + dx*c)
	if depth <= 0 {
		return nil
	}
	lateral := fix14(dy*c - dx*s)
	return &Projection{cam.CX + math.Trunc(lateral*cam.F/depth), cam.CY - math.Trunc(dz*cam.F/depth), depth}
}
func DepthLevel(depth float64, occ *Occlusion) float64 {
	return math.Max(0, math.Floor(depth/math.Max(1, occ.Scale)))
}
func SceneryOccludes(occ *Occlusion, x, y int, level float64) bool {
	if occ == nil || x < 0 || y < 0 || x >= occ.W || y >= occ.H {
		return false
	}
	i := y*occ.W + x
	return i < len(occ.Z) && float64(occ.Z[i]) < level
}
func Bearing(dx, dy float64) int {
	return int(int32JS(jsRound(math.Atan2(dy, dx)*256/(2*math.Pi))) & 255)
}

type StarPoint struct {
	Identifier                      string
	PositionX, PositionY, PositionZ float64
}
