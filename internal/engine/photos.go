package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/script"
)

const (
	PhotoWidth     = 320
	PhotoHeight    = 240
	PhotoX         = 160
	PhotoY         = 120
	CameraOK       = 0
	CameraIDTaken  = 1
	CameraNoPhoto  = 2
	PhotoExposures = 36
)

type PhotoEntry struct {
	ID    float64
	Photo *RGBAFrame
}
type PhotoOverlay struct {
	Photo *RGBAFrame
	X, Y  int
}
type PhotoStore interface {
	All() ([]PhotoEntry, error)
	Put(float64, *RGBAFrame) error
}

// PhotoAlbum keeps accepted exposures even when persistence is unavailable.
// Only an ID collision may ask the shutter script to retry a photograph.
// Store callbacks may yield through the session executor for host operations.
type PhotoAlbum struct {
	Store               PhotoStore
	Log                 func(string)
	photos              orderedMap[*RGBAFrame]
	hydrated, hydrating bool
}

func (a *PhotoAlbum) log(line string) {
	if a.Log != nil {
		a.Log(line)
	}
}
func photoKey(id float64) string { return script.Num(id).String() }
func (a *PhotoAlbum) Open(task *Task) int {
	if a.hydrated {
		return CameraOK
	}
	if a.Store == nil {
		a.hydrated = true
		return CameraOK
	}
	if a.hydrating {
		task.Wait(func() bool { return !a.hydrating })
		return CameraOK
	}
	a.hydrating = true
	defer func() { a.hydrating = false }()
	entries, err := a.Store.All()
	if err != nil {
		a.log("photo album: the store could not be read (" + err.Error() + "); this session only")
		a.Store = nil
	} else {
		for _, entry := range entries {
			key := photoKey(entry.ID)
			if !a.photos.Has(key) {
				a.photos.Set(key, entry.Photo)
			}
		}
		a.log(fmt.Sprintf("photo album: %d photo(s) in the store", a.photos.Len()))
	}
	a.hydrated = true
	return CameraOK
}
func (a *PhotoAlbum) Save(id float64, photo *RGBAFrame) int {
	key := photoKey(id)
	if a.photos.Has(key) {
		return CameraIDTaken
	}
	a.photos.Set(key, photo)
	if a.Store != nil {
		if err := a.Store.Put(id, photo); err != nil {
			a.log(fmt.Sprintf("photo album: %g could not be stored (%v)", id, err))
		}
	}
	return CameraOK
}
func (a *PhotoAlbum) Get(id float64) *RGBAFrame { return a.photos.Get(photoKey(id)) }
func (a *PhotoAlbum) Count() int                { return a.photos.Len() }
func (a *PhotoAlbum) Reset() {
	a.photos = orderedMap[*RGBAFrame]{}
	a.hydrated = false
	a.hydrating = false
}
