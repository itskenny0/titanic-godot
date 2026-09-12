package engine

import (
	"github.com/itskenny0/titanic-godot/internal/script"
	"slices"
)

func registerSaveBuiltins(c builtinContext) {
	s := c.s
	c.action("savegame", func(a []script.Value, _ *script.Frame) error {
		data, err := s.SnapshotSave()
		if err != nil {
			return err
		}
		s.Clock.Freeze()
		defer s.Clock.Thaw()
		return s.OnSaveGame(data, strArg(a, 0))
	})
	c.action("opengame", func(a []script.Value, _ *script.Frame) error {
		f := &s.Fade
		level, queue, snapshot, pending := f.Level, slices.Clone(f.Queue), f.Snapshot, f.PendingReveal
		back := func() {
			f.Level = level
			f.Queue = append(f.Queue, queue...)
			f.Snapshot = snapshot
			f.PendingReveal = pending
		}
		f.Queue = nil
		f.Snapshot = nil
		f.Level = 1
		f.PendingReveal = false
		data, err := func() ([]byte, error) { s.Clock.Freeze(); defer s.Clock.Thaw(); return s.OnLoadGame(strArg(a, 0)) }()
		if err != nil {
			return err
		}
		if data == nil {
			back()
			return nil
		}
		loaded, err := s.LoadGame(data)
		if err != nil {
			return err
		}
		if !loaded {
			back()
			return nil
		}
		f.Level = 0
		return nil
	})
}
