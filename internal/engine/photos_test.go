package engine

import (
	"errors"
	"github.com/itskenny0/titanic-godot/internal/script"
	"reflect"
	"testing"
)

type testPhotoStore struct {
	all func() ([]PhotoEntry, error)
	put func(float64, *RGBAFrame) error
}

func (s testPhotoStore) All() ([]PhotoEntry, error)         { return s.all() }
func (s testPhotoStore) Put(id float64, p *RGBAFrame) error { return s.put(id, p) }
func TestPhotoStoreFailureKeepsAcceptedShots(t *testing.T) {
	a := new(PhotoAlbum)
	p := &RGBAFrame{Width: 1, Height: 1, RGBA: []byte{17, 23, 29, 255}}
	reads, writes := 0, 0
	a.Store = testPhotoStore{all: func() ([]PhotoEntry, error) { reads++; return []PhotoEntry{{ID: 7, Photo: &RGBAFrame{Width: 2}}}, nil }, put: func(float64, *RGBAFrame) error { writes++; return errors.New("disk unavailable") }}
	if a.Save(7, p) != CameraOK {
		t.Fatal("fresh shot refused")
	}
	a.Open(nil)
	a.Open(nil)
	if reads != 1 || a.Get(7) != p || writes != 1 || a.Save(7, p) != CameraIDTaken {
		t.Fatal("hydration replaced a fresh photo or failed write caused false retry")
	}
	a.Reset()
	if a.Count() != 0 {
		t.Fatal("album reset failed")
	}
	a.Open(nil)
	if reads != 2 || a.Get(7).Width != 2 {
		t.Fatal("reset discarded the persistent store")
	}
	a.Reset()
	a.Store = testPhotoStore{all: func() ([]PhotoEntry, error) { return nil, errors.New("store unavailable") }}
	a.Open(nil)
	if a.Store != nil || a.Save(8, p) != CameraOK || a.Get(8) != p {
		t.Fatal("read failure disabled the camera")
	}
}
func TestPhotoHydrationCoalescesConcurrentCalls(t *testing.T) {
	e := NewExecutor()
	defer e.Close()
	a := new(PhotoAlbum)
	reads := 0
	photo := &RGBAFrame{Width: 1}
	a.Store = testPhotoStore{all: func() ([]PhotoEntry, error) {
		reads++
		e.Current().Sleep(50)
		return []PhotoEntry{{ID: 8, Photo: photo}}, nil
	}}
	one := e.Start("first open", func(task *Task) error { a.Open(task); return nil })
	two := e.Start("second open", func(task *Task) error { a.Open(task); return nil })
	e.Pump(0, false, 100)
	if reads != 1 || one.Done() || two.Done() {
		t.Fatal("album did not share the pending read")
	}
	e.Pump(50, false, 100)
	if !one.Done() || !two.Done() || a.Get(8) != photo || reads != 1 {
		t.Fatal("shared read did not complete both callers")
	}
}
func TestPhotoAndXRayPluginLifecycle(t *testing.T) {
	s := builtinTestSession(t)
	p := &RGBAFrame{Width: 4, Height: 3, RGBA: make([]byte, 48)}
	var captured [2]float64
	s.GrabPhoto = func(x, y float64) *RGBAFrame { captured = [2]float64{x, y}; return p }
	call := func(args ...script.Value) script.Value { return callBuiltin(t, s, "pluginfx", args...) }
	if call(script.Str("camera"), script.Str("unused"), script.Num(7)).Num() != CameraNoPhoto {
		t.Fatal("missing photo not reported")
	}
	point := script.Num(float64(PackPoint(-10, 20)))
	if call(script.Str("camera"), script.Str("unused"), script.Num(7), point).Num() != CameraOK || captured != [2]float64{-10, 20} {
		t.Fatal("camera did not capture at requested coordinates")
	}
	if call(script.Str("camera"), script.Str("unused"), script.Num(7), point).Num() != CameraIDTaken {
		t.Fatal("photo ID overwritten")
	}
	s.Fade.Level = 1
	s.Fade.PendingReveal = true
	s.Fade.Queue = []FadeRamp{{To: 1, Steps: 3}}
	s.Fade.Snapshot = p
	if call(script.Str("camera"), script.Str("unused"), script.Num(7)).Num() != CameraOK || s.PhotoOverlay == nil || s.PhotoOverlay.Photo != p || s.PhotoOverlay.X != PhotoX || s.PhotoOverlay.Y != PhotoY || s.Fade.Level != 0 || s.Fade.PendingReveal || s.Fade.Snapshot != nil || len(s.Fade.Queue) != 0 {
		t.Fatal("photo display did not replace fade with album image")
	}
	s.TextOverlay = []TextOverlay{{Text: "old"}}
	s.ClearTextOverlay()
	if s.PhotoOverlay != nil || len(s.TextOverlay) != 0 {
		t.Fatal("old overlays survived a screen change")
	}
	call(script.Str("xray"), script.Str("hidden"), script.Str("base"), script.Str("mask"), script.Str("light"))
	call(script.Str("xray"), point)
	want := &XRayReveal{Hidden: "hidden", Base: "base", Mask: "mask", Light: "light", X: -10, Y: 20, Aimed: true}
	if !reflect.DeepEqual(s.XRay, want) {
		t.Fatal("xray did not retain its target", s.XRay)
	}
	call(script.Str("xray"))
	if s.XRay != nil {
		t.Fatal("xray did not disarm")
	}
}
