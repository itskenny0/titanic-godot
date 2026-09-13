package engine

import (
	"math"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
)

type RoomLayer interface {
	RoomVersion() int
	RoomAnimating() bool
	RoomFrame() *CachedFrame
	RoomPalette() []byte
	RoomPropPalette() []byte
	BandPropPalette([]byte) []byte
	RoomCamera() *WorldCamera
	RoomOcclusion() *Occlusion
	ApplyRoomClut(*ClutDim)
	RefreshRoomGamma()
	AdvanceRoom(float64) (*CachedFrame, error)
	DrawRoomHotspots(*DrawContext)
	RoomSignature(*DrawSignature)
	PointInRoomImage(float64, float64) bool
	RoomClickAt(float64, float64) (bool, error)
	RoomHitTest(float64, float64) (*HitTarget, error)
	SendRoomPainting(string, string, script.Value)
	RoomKeyDown(string) (bool, error)
	ArmNavHooks() NavHooks
	DisarmNavHooks(*NavHooks)
}

var _ RoomLayer = (*SetViewer)(nil)
var _ ViewerDirector = (*ScreenDirector)(nil)

// referenceSlot tracks just the latest input. It cannot retain a growing history
// of old room images, photos or fade snapshots.
type referenceSlot[T comparable] struct {
	value T
	id    uint64
}

func (s *referenceSlot[T]) token(v T) uint64 {
	var zero T
	if v == zero {
		s.value, s.id = zero, 0
		return 0
	}
	if s.value != v {
		s.value, s.id = v, newDrawID()
	}
	return s.id
}

type ScreenDirector struct {
	Session      *Session
	Screen       *ScreenPresenter
	Movies       *MoviePlayer
	PuppetView   *PuppetView
	Gamma        *ScreenGamma
	Log          func(string)
	OnRoomReveal func()
	OnCursor     func(string)
	room         RoomLayer
	sig          DrawSignature
	stageDim     *ClutDim
	flatPal      struct {
		base, out  []byte
		dim        *ClutDim
		generation uint64
	}
	matteSeen struct {
		pixels []byte
		matte  bool
	}
	snapshotRef referenceSlot[*RGBAFrame]
	photoRef    referenceSlot[*RGBAFrame]
	flatRef     referenceSlot[*FlatImage]
	dimRef      referenceSlot[*ClutDim]
	roomRef     referenceSlot[*CachedFrame]
	cursorGate  string
}

func NewScreenDirector(s *Session, w, h int, gamma *ScreenGamma) *ScreenDirector {
	if w <= 0 {
		w = ScreenWidth
	}
	if h <= 0 {
		h = ScreenHeight
	}
	if gamma == nil {
		gamma = NewScreenGamma()
	}
	d := &ScreenDirector{Session: s, Screen: NewScreenPresenter(w, h), Gamma: gamma, Log: func(string) {}}
	d.Movies = NewMoviePlayer(s, gamma, func() {
		if d.OnRoomReveal != nil {
			d.OnRoomReveal()
		}
	})
	d.Movies.Log = func(line string) { d.Log(line) }
	d.PuppetView = NewPuppetView(s, gamma)
	s.OnClut = d.SetClut
	s.RepaintNow = func() { _, err := d.PaintWorldInto(); d.report(err) }
	s.GrabPhoto = d.GrabPhoto
	s.CaptureFrame = func() *RGBAFrame { f, err := d.CaptureFrame(); d.report(err); return f }
	s.OnPlayMovie = func(name string, start *int) error {
		at := 0
		if start != nil {
			at = *start
		}
		return d.Movies.Play(s.Executor.Current(), name, at)
	}
	s.HitTestAt = func(x, y float64) HitTarget { hit, err := d.HitTestAt(x, y); d.report(err); return hit }
	s.PointInSet = func(x, y float64) bool { return d.room != nil && d.room.PointInRoomImage(x, y) }
	s.PointInStage = func(float64, float64) bool { return s.StageOpen() && s.StageCtrl.CurrentFlat != "none" }
	return d
}
func (d *ScreenDirector) report(err error) {
	if err != nil {
		d.Log(err.Error())
	}
}
func (d *ScreenDirector) SetRoom(room RoomLayer) {
	d.room = room
	d.stageDim = nil
	d.flatPal.base, d.flatPal.out = nil, nil
}
func (d *ScreenDirector) CurrentRoom() RoomLayer { return d.room }
func (d *ScreenDirector) roomFrame() *CachedFrame {
	if d.room == nil {
		return nil
	}
	return d.room.RoomFrame()
}
func (d *ScreenDirector) roomCamera() *WorldCamera {
	if d.room == nil {
		return nil
	}
	return d.room.RoomCamera()
}
func (d *ScreenDirector) roomOcclusion() *Occlusion {
	if d.room == nil {
		return nil
	}
	return d.room.RoomOcclusion()
}
func (d *ScreenDirector) roomAnimating() bool { return d.room != nil && d.room.RoomAnimating() }
func (d *ScreenDirector) flatImage() *FlatImage {
	s := d.Session.StageCtrl
	return s.FlatImage(s.CurrentFlat)
}
func (d *ScreenDirector) Tick(rawNow float64) (*CachedFrame, error) {
	s := d.Session
	now := s.Clock.GameTime(rawNow)
	if d.room != nil {
		d.room.RefreshRoomGamma()
	}
	s.Props.Tick(now, EngineStepMS)
	s.Fade.Tick(now, s.ScriptBusy())
	s.Wipe.Tick(now)
	if err := s.TickTime(now); err != nil {
		return nil, err
	}
	s.Scheduler.ServiceFrameLoops()
	d.serviceCursor()
	if d.Movies.Playing() {
		f, err := d.Movies.Tick(now)
		if err != nil {
			return nil, err
		}
		if f != nil {
			return &CachedFrame{Pixels: f.Pixels, Width: f.Width, Height: f.Height, ID: f.ID}, nil
		}
		return d.roomFrame(), nil
	}
	if d.room != nil {
		return d.room.AdvanceRoom(now)
	}
	return nil, nil
}
func (d *ScreenDirector) ScreenOwner() string {
	if d.Movies.Playing() {
		return "movie"
	}
	if d.Session.Fade.PendingReveal {
		return "held"
	}
	if d.Conversing() && d.Session.Fade.Snapshot == nil {
		return "puppet"
	}
	if d.Session.Fade.Snapshot != nil {
		return "faded"
	}
	return "world"
}
func (d *ScreenDirector) MovingCamera() bool { return d.roomAnimating() || d.Session.ScriptBusy() }
func (d *ScreenDirector) Busy() bool {
	return d.roomAnimating() || d.Movies.Playing() || d.Conversing() || d.Session.Fade.Active()
}
func (d *ScreenDirector) InputLocked() bool { return d.Busy() || d.Session.ScriptBusy() }
func (d *ScreenDirector) Conversing() bool {
	p := d.Session.PuppetCtrl.Puppet
	return p != nil && p.Visible
}
func (d *ScreenDirector) Speaking() bool {
	p := d.Session.PuppetCtrl.Puppet
	return p != nil && p.SpeakSkip != nil
}
func (d *ScreenDirector) ConversingWith() string {
	if d.Conversing() {
		return d.Session.PuppetCtrl.Puppet.Name
	}
	return ""
}
func (d *ScreenDirector) AwaitingChoice() bool {
	p := d.Session.PuppetCtrl.Puppet
	return p != nil && p.EventWaiter != nil
}
func (d *ScreenDirector) AwaitingInput() bool {
	return len(d.Movies.WaitingRegions()) > 0 || d.AwaitingChoice()
}
func (d *ScreenDirector) Quiescent() bool { return d.AwaitingInput() || !d.InputLocked() }
func (d *ScreenDirector) Choices() []Bevel {
	if !d.AwaitingChoice() {
		return []Bevel{}
	}
	return append([]Bevel{}, d.Session.PuppetCtrl.Puppet.Bevels...)
}
func (d *ScreenDirector) ChoiceRects() []ScreenRect {
	if d.AwaitingChoice() {
		return d.PuppetView.BevelRects()
	}
	return []ScreenRect{}
}
func (d *ScreenDirector) SetClut(target string, dim *ClutDim) {
	s := d.Session
	t := strings.ToLower(target)
	if t == "current" {
		t = "stage"
		if s.ViewShowing() {
			t = "set"
		}
	}
	if t == "set" && d.room != nil {
		d.room.ApplyRoomClut(dim)
	} else if t == "stage" {
		d.stageDim = dim
	}
	showing := ""
	if s.ViewShowing() {
		showing = "set"
	} else if s.StageCtrl.CurrentFlat != "none" {
		showing = "stage"
	}
	if t == showing && !d.Conversing() {
		s.Fade.Queue = nil
		s.Fade.Snapshot = nil
		s.Fade.PendingReveal = false
		s.Fade.Blanked = false
		s.Fade.Level = 0
	}
}
func (d *ScreenDirector) FlatPalette(base []byte) []byte {
	hit := &d.flatPal
	if sameBytes(hit.base, base) && hit.dim == d.stageDim && hit.generation == d.Gamma.Generation && hit.out != nil {
		return hit.out
	}
	next := base
	if d.stageDim != nil {
		next = DimPalette(base, *d.stageDim)
	}
	hit.base, hit.dim, hit.generation, hit.out = base, d.stageDim, d.Gamma.Generation, d.Gamma.DisplayPalette(next)
	return hit.out
}
func (d *ScreenDirector) GrabPhoto(cx, cy float64) *RGBAFrame {
	s := d.Screen
	if !s.FrameValid || s.Width < PhotoWidth || s.Height < PhotoHeight {
		return nil
	}
	x0 := int(math.Max(0, math.Min(float64(s.Width-PhotoWidth), jsRound(cx)-PhotoWidth/2)))
	y0 := int(math.Max(0, math.Min(float64(s.Height-PhotoHeight), jsRound(cy)-PhotoHeight/2)))
	rgba := make([]byte, PhotoWidth*PhotoHeight*4)
	for y := 0; y < PhotoHeight; y++ {
		from := ((y0+y)*s.Width + x0) * 4
		copy(rgba[y*PhotoWidth*4:], s.Frame[from:from+PhotoWidth*4])
	}
	return &RGBAFrame{RGBA: rgba, Width: PhotoWidth, Height: PhotoHeight}
}
func (d *ScreenDirector) CaptureFrame() (*RGBAFrame, error) {
	if d.Conversing() && d.Session.Fade.Snapshot == nil {
		if err := d.CompositePuppetScreen(); err != nil {
			return nil, err
		}
	}
	if shot := d.Screen.Capture(); shot != nil {
		return shot, nil
	}
	f := d.roomFrame()
	if f == nil {
		return nil, nil
	}
	rgba := make([]byte, f.Width*f.Height*4)
	if err := df.IndexedRGBA(f.Pixels, d.room.RoomPalette(), rgba); err != nil {
		return nil, err
	}
	return &RGBAFrame{RGBA: rgba, Width: f.Width, Height: f.Height}, nil
}
