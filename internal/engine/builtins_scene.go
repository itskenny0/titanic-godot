package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"math"
	"strings"
)

func registerSceneBuiltins(c builtinContext) {
	s := c.s
	c.action("opensetfile", func(a []script.Value, _ *script.Frame) error {
		return s.OpenSetFile(strArg(a, 0), strArg(a, 1), strArg(a, 2))
	})
	c.action("closesetfile", func([]script.Value, *script.Frame) error {
		if s.CurrentBinding != nil {
			s.CurrentBinding.CloseSet()
		}
		s.SetCurrentSetName("none")
		return nil
	})
	c.v("currentset", func([]script.Value, *script.Frame) script.Value { return script.Str(s.SetName) })
	c.v("camerahi", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) == 0 {
			return script.Num(s.CameraHiBias)
		}
		s.CameraHiBias = float64(int32JS(propertyNumber(a[0])))
		return script.Num(0)
	})
	c.v("currentscene", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) == 0 {
			return script.Str(s.CurrentSceneName())
		}
		d := strings.ToLower(a[0].String())
		if strings.HasPrefix(d, "scene") {
			s.OnSceneJump(d)
		} else {
			s.OnNavigate(d)
		}
		return script.Num(0)
	})
	c.v("currentview", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) == 0 {
			return script.Str(s.CurrentViewName())
		}
		s.OnViewJump(strings.ToLower(a[0].String()))
		return script.Num(0)
	})
	c.v("setvisible", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) == 0 {
			return script.Bool(s.ViewShowing())
		}
		s.SetVisible = a[0].Truthy()
		return script.Num(0)
	})
	c.v("stagevisible", func([]script.Value, *script.Frame) script.Value { return script.Bool(s.StageOpen()) })
	c.v("currentstage", func([]script.Value, *script.Frame) script.Value {
		s.LastResult = s.StageCtrl.Name
		name := s.StageCtrl.StageRefName()
		if name == "" {
			name = s.StageCtrl.Name
		}
		return script.Str(name)
	})
	c.v("currentflat", func([]script.Value, *script.Frame) script.Value { return script.Str(s.StageCtrl.CurrentFlat) })
	c.v("openstagefile", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Bool(s.StageCtrl.OpenStageFile(strArg(a, 0)))
	})
	c.action("closestagefile", func([]script.Value, *script.Frame) error { s.StageCtrl.CloseStageFile(); return nil })
	c.action("gotoflat", func(a []script.Value, _ *script.Frame) error { s.StageCtrl.GotoFlat(strArg(a, 0)); return nil })
	c.v("flattoindex", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(s.StageCtrl.FlatToIndex(strArg(a, 0))))
	})
	c.v("indextoflat", func(a []script.Value, _ *script.Frame) script.Value {
		idx := numArg(a, 0, 0) - 1
		if idx < 0 || idx >= float64(len(s.StageCtrl.FlatNames)) || idx != math.Trunc(idx) {
			return script.Str("none")
		}
		return script.Str(s.StageCtrl.FlatNames[int(idx)])
	})
	c.v("countflats", func([]script.Value, *script.Frame) script.Value {
		return script.Num(float64(len(s.StageCtrl.FlatNames)))
	})
	scenes := func() []df.Scene {
		if s.CurrentBinding != nil {
			return s.CurrentBinding.Set.Scenes
		}
		return nil
	}
	scene := func(name string) *df.Scene {
		for _, sc := range scenes() {
			if strings.EqualFold(sc.SceneName, name) {
				return &sc
			}
		}
		return nil
	}
	view := func(sn, vn string) *df.SceneView {
		sc := scene(sn)
		if sc != nil {
			for _, v := range sc.Views {
				if strings.EqualFold(v.ViewName, vn) {
					return &v
				}
			}
		}
		return nil
	}
	c.v("countscenes", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(len(scenes()))) })
	c.v("indextoscene", func(a []script.Value, _ *script.Frame) script.Value {
		names := []string{}
		for _, sc := range scenes() {
			names = append(names, sc.SceneName)
		}
		return script.Str(stringIndex(names, numArg(a, 0, 0)-1))
	})
	c.v("countviews", func(a []script.Value, _ *script.Frame) script.Value {
		if sc := scene(strArg(a, 0)); sc != nil {
			return script.Num(float64(len(sc.Views)))
		}
		return script.Num(0)
	})
	c.v("indextoview", func(a []script.Value, _ *script.Frame) script.Value {
		names := []string{}
		if sc := scene(strArg(a, 0)); sc != nil {
			for _, v := range sc.Views {
				names = append(names, v.ViewName)
			}
		}
		return script.Str(stringIndex(names, numArg(a, 1, 0)-1))
	})
	c.v("scenexyz", func(a []script.Value, _ *script.Frame) script.Value {
		sc := scene(strArg(a, 0))
		if sc == nil {
			return script.Num(0)
		}
		switch numArg(a, 1, 1) {
		case 1:
			return script.Num(float64(sc.XAxisMap))
		case 2:
			return script.Num(float64(sc.ZAxisMap))
		case 3:
			return script.Num(float64(sc.YAxisMap))
		case 4:
			return script.Num(float64(PackPoint(float64(sc.XAxisMap), float64(sc.ZAxisMap))))
		}
		return script.Num(0)
	})
	c.v("countshops", func([]script.Value, *script.Frame) script.Value { return script.Num(float64(s.Props.Shops.Len())) })
	c.v("indextoshop", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.Props.Shops.keys, numArg(a, 0, 0)-1))
	})
	c.v("countbuttons", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(len(s.StageCtrl.FlatButtonNames(strArg(a, 0)))))
	})
	c.v("indextobutton", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.StageCtrl.FlatButtonNames(strArg(a, 0)), numArg(a, 1, 0)-1))
	})
	c.v("countpaintings", func(a []script.Value, _ *script.Frame) script.Value {
		if v := view(strArg(a, 0), strArg(a, 1)); v != nil {
			return script.Num(float64(len(v.Objects)))
		}
		return script.Num(0)
	})
	c.v("indextopainting", func(a []script.Value, _ *script.Frame) script.Value {
		names := []string{}
		if v := view(strArg(a, 0), strArg(a, 1)); v != nil {
			for _, o := range v.Objects {
				names = append(names, o.Identifier)
			}
		}
		return script.Str(stringIndex(names, numArg(a, 2, 0)-1))
	})
	c.v("roadahead", func(a []script.Value, _ *script.Frame) script.Value {
		v := view(strArg(a, 0), strArg(a, 1))
		if v != nil && s.CurrentBinding != nil {
			for _, tr := range s.CurrentBinding.Set.Transitions {
				if tr.ViewIDstart == v.ViewID || tr.ViewIDend == v.ViewID {
					return script.Num(1)
				}
			}
		}
		return script.Num(0)
	})
	c.action("playmovie", func(a []script.Value, _ *script.Frame) error {
		name := strArg(a, 0)
		if s.HasRealFrames || s.ModalMovies {
			return s.OnPlayMovie(name, nil)
		}
		// In simulated mode the movie starts without holding its calling script.
		s.Track("movie:"+name, true, func(*Task) error { return s.OnPlayMovie(name, nil) })
		return nil
	})
	c.v("actionframe", func(a []script.Value, _ *script.Frame) script.Value {
		n := numArg(a, 0, 0)
		return script.Bool(n == math.Trunc(n) && s.MovieActions[int(n)])
	})
	c.action("openshopfile", func(a []script.Value, _ *script.Frame) error { s.OpenShop(strArg(a, 0)); return nil })
	c.action("closeshopfile", func(a []script.Value, _ *script.Frame) error { s.CloseShop(strArg(a, 0)); return nil })
	c.v("fileexists", func(a []script.Value, _ *script.Frame) script.Value {
		key := strings.ToLower(strArg(a, 0))
		if key == "" {
			return script.Num(0)
		}
		b, err := s.Read(key)
		return script.Bool(err == nil && b != nil)
	})
	for _, name := range []string{"plain", "nodraw", "barndoorclose", "barndooropen", "irisclose", "irisopen", "scrolldown", "scrollup", "scrollright", "scrolleft", "venetian", "wipedown", "wipeup", "wiperight", "wipeleft", "turnright", "turnleft", "turnup", "turndown", "turnhalfleft", "turnhalfright"} {
		c.v(name, func([]script.Value, *script.Frame) script.Value { return script.Str(name) })
	}
	fade := func(to float64, a []script.Value) error {
		steps := propertyNumberOrZero(arg(a, 1, script.Num(0)))
		if steps == 0 {
			steps = 10
		}
		steps = math.Max(1, steps)
		f := &s.Fade
		if to == 1 && f.Snapshot == nil && s.CaptureFrame != nil {
			f.Snapshot = s.CaptureFrame()
		}
		f.Queue = append(f.Queue, FadeRamp{To: to, Steps: steps})
		f.PendingReveal = false
		f.Blanked = false
		if s.HasRealFrames || s.ModalMovies {
			task, err := c.task()
			if err != nil {
				return err
			}
			task.Sleep(float64(steps*50) / 3)
		}
		return nil
	}
	c.action("screentoblack", func(a []script.Value, _ *script.Frame) error { return fade(1, a) })
	c.action("blacktoscreen", func(a []script.Value, _ *script.Frame) error {
		queued := false
		for _, r := range s.Fade.Queue {
			queued = queued || r.To == 1
		}
		if s.Fade.Level == 0 && !queued {
			s.Fade.Level = 1
		}
		return fade(0, a)
	})
	blackNow := func() {
		s.Fade.Queue = nil
		s.Fade.Snapshot = nil
		s.Fade.Level = 1
		s.Fade.PendingReveal = false
		s.Fade.Blanked = true
	}
	c.action("blackscreen", func([]script.Value, *script.Frame) error { blackNow(); return nil })
	c.v("currenttheme", func([]script.Value, *script.Frame) script.Value { return script.Str(s.CurrentThemeName) })
	c.v("framerate", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) == 0 {
			return script.Num(s.Clock.FrameRate)
		}
		s.Clock.FrameRate = a[0].Num()
		return script.Num(0)
	})
	c.v("wavevolume", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) == 0 {
			return script.Num(s.WaveVolume)
		}
		return script.Num(s.SetWaveVolume(a[0].Num()))
	})
	c.v("themevol", func(a []script.Value, _ *script.Frame) script.Value {
		if len(a) < 2 {
			return script.Num(s.ThemeVolume)
		}
		s.SetThemeVolume(a[1].Num(), strArg(a, 0))
		return script.Num(0)
	})
	c.action("mixclut", func(a []script.Value, _ *script.Frame) error {
		if strings.EqualFold(strArg(a, 1), "black") {
			s.OnClut(strArg(a, 0), &ClutDim{Lo: numArg(a, 2, 0), Hi: numArg(a, 3, 255), Amt: numArg(a, 4, 0)})
		}
		return nil
	})
	c.action("clut", func(a []script.Value, _ *script.Frame) error {
		target := strings.ToLower(strArg(a, 0))
		if target == "black" {
			blackNow()
		} else if target != "" {
			s.OnClut(target, nil)
		}
		return nil
	})
	c.action("visualeffect", func(a []script.Value, _ *script.Frame) error {
		name := strings.ToLower(arg(a, 0, script.Str("plain")).String())
		if name == "plain" {
			if s.RepaintNow != nil {
				s.RepaintNow()
			}
			return nil
		}
		s.Fade.Queue = nil
		s.Fade.Snapshot = nil
		s.Fade.PendingReveal = false
		s.Fade.Level = 0
		dir := map[string]string{"wipeleft": "left", "wiperight": "right", "barndooropen": "open", "barndoorclose": "close", "turnleft": "turnleft", "turnhalfleft": "turnleft", "turnright": "turnright", "turnhalfright": "turnright"}[name]
		if dir == "" || s.CaptureFrame == nil {
			return nil
		}
		from := s.CaptureFrame()
		if from == nil {
			return nil
		}
		var to *RGBAFrame
		if dir == "turnleft" || dir == "turnright" {
			if s.RepaintNow != nil {
				s.RepaintNow()
			}
			to = s.CaptureFrame()
			if to == nil {
				return nil
			}
		}
		w := &s.Wipe
		w.From = from
		w.To = to
		w.Dir = dir
		w.Settled = false
		w.Span = 1
		if name == "turnhalfleft" || name == "turnhalfright" {
			w.Span = .5
		}
		steps := numArg(a, 1, 0)
		if steps == 0 {
			steps = 1
		}
		w.Steps = math.Min(1000, math.Max(1, steps))
		w.Step = 0
		w.LastTick = 0
		task, err := c.task()
		if err != nil {
			return err
		}
		cap := float64(w.Steps*4) + 60
		for i := 0.; i < cap && w.Active(); i++ {
			s.NextFrame(task)
		}
		if w.Active() {
			w.End()
		}
		return nil
	})
	c.action("hidecursor", func([]script.Value, *script.Frame) error { s.CursorDepth--; return nil })
	c.action("showcursor", func([]script.Value, *script.Frame) error { s.CursorDepth++; return nil })
	for _, name := range []string{"debugger", "exportclut", "propwarm", "actorwarm", "shopwarm", "propscript", "buttonscript", "scenescript", "flatscript", "stagescript", "bootscript", "postscript", "setscript", "paintingscript", "puppetscript", "castscript", "actorscript", "shopscript", "serverscript", "optionkey", "commandkey"} {
		c.v(name, func([]script.Value, *script.Frame) script.Value { return script.Num(0) })
	}
	c.v("shiftkey", func([]script.Value, *script.Frame) script.Value { return script.Bool(s.ShiftDown) })
}
