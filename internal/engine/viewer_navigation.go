package engine

import (
	"github.com/itskenny0/titanic-godot/internal/df"
	"strings"
)

func (v *SetViewer) Navigate(direction string) {
	s := v.Session
	s.NavHappened = true
	// A scripted turn must become visible to currentview() before the call
	// returns. The reference async driver runs immediately up to its first
	// wait; queuing an idle camera left gotowin spinning and scheduling turns.
	if s.NavFromScript && v.Busy() {
		s.Track("navigate:"+direction, false, func(task *Task) error {
			for i := 0; i < maxFadeWaitTicks && v.Busy(); i++ {
				s.NextFrame(task)
			}
			if v.Busy() {
				return nil
			}
			return v.navigateNow(direction, EngineStepMS)
		})
		return
	}
	pace := v.PlayerPace()
	if s.NavFromScript {
		pace = EngineStepMS
	}
	v.report(v.navigateNow(direction, pace))
}
func (v *SetViewer) navigateNow(direction string, pace float64) error {
	switch direction {
	case "strait":
		if v.Session.Fade.Active() {
			v.Session.Track("walkAfterFade", false, func(task *Task) error {
				for i := 0; i < maxFadeWaitTicks && v.Session.Fade.Active(); i++ {
					v.Session.NextFrame(task)
				}
				return v.Walk(pace)
			})
			return nil
		}
		return v.Walk(pace)
	case "left":
		return v.Turn(LeftTurns, pace)
	case "right":
		return v.Turn(RightTurns, pace)
	}
	return nil
}
func (v *SetViewer) SceneJump(scene string) {
	v.Session.NavHappened = true
	v.pendingJumpScene = &scene
}
func (v *SetViewer) ViewJump(view string) {
	v.Session.NavHappened = true
	scene := v.pendingJumpScene
	v.pendingJumpScene = nil
	v.report(v.Teleport(scene, view))
}
func (v *SetViewer) BindJumpHooks() {
	v.pendingJumpScene = nil
	v.Session.OnSceneJump = v.SceneJump
	v.Session.OnViewJump = v.ViewJump
}
func (v *SetViewer) ArmNavHooks() NavHooks {
	s := v.Session
	prev := NavHooks{OnNavigate: s.OnNavigate, OnSceneJump: s.OnSceneJump, OnViewJump: s.OnViewJump, Active: s.NavGestureActive}
	v.pendingJumpScene = nil
	s.NavGestureActive = true
	s.OnNavigate = v.Navigate
	s.OnSceneJump = v.SceneJump
	s.OnViewJump = v.ViewJump
	return prev
}
func (v *SetViewer) DisarmNavHooks(prev *NavHooks) {
	s := v.Session
	if prev == nil {
		s.NavGestureActive = false
		s.OnNavigate = func(string) {}
		s.OnSceneJump = v.SceneJump
		s.OnViewJump = v.ViewJump
		return
	}
	s.NavGestureActive = prev.Active
	s.OnNavigate = prev.OnNavigate
	s.OnSceneJump = prev.OnSceneJump
	s.OnViewJump = prev.OnViewJump
}
func (v *SetViewer) Teleport(sceneName *string, viewName string) error {
	target := v.SceneIdx
	if sceneName != nil && *sceneName != "" {
		target = -1
		for i, scene := range v.Set.Scenes {
			if strings.EqualFold(scene.SceneName, *sceneName) {
				target = i
				break
			}
		}
	}
	if target < 0 {
		return nil
	}
	scene := &v.Set.Scenes[target]
	view := -1
	for i, vw := range scene.Views {
		if strings.EqualFold(vw.ViewName, viewName) {
			view = i
			break
		}
	}
	changed := target != v.SceneIdx || view >= 0 && view != v.ViewIdx
	v.SceneIdx = target
	if view >= 0 {
		v.ViewIdx = view
	}
	if err := v.ShowView(); err != nil {
		return err
	}
	if v.Scripts.InLifecycle() {
		return nil
	}
	sceneIdx := v.SceneIdx
	if !changed {
		v.Session.Track("", false, func(*Task) error { v.Scripts.ViewSettled(sceneIdx); return nil })
	} else {
		v.Session.Track("jump-viewChanged", false, func(*Task) error { v.Scripts.ViewChanged(sceneIdx); return nil })
	}
	return nil
}
func (v *SetViewer) RoomKeyDown(key string) (bool, error) {
	s := v.Session
	s.NavHappened = false
	prev := v.ArmNavHooks()
	defer v.DisarmNavHooks(&prev)
	consumed := v.Scripts.KeyDown(v.SceneIdx, key)
	return consumed || s.NavHappened, nil
}
func (v *SetViewer) KeyDown(key string, special bool) (bool, error) {
	if v.Director != nil {
		return v.Director.KeyDown(key, special)
	}
	return v.RoomKeyDown(key)
}
func (v *SetViewer) PressNav(key string) error {
	handled, err := v.KeyDown(key, false)
	if err != nil || handled {
		return err
	}
	if key == "uparrow" {
		return v.Walk()
	}
	dir := RightTurns
	if key == "leftarrow" {
		dir = LeftTurns
	}
	return v.Turn(dir)
}
func (v *SetViewer) departScene(label string) *Task {
	scene := v.SceneIdx
	return v.Session.Track(label, false, func(*Task) error { v.Scripts.CloseScene(scene); return nil })
}
func (v *SetViewer) settleScene(departure *Task, scene int, label string) {
	v.Session.Track(label, false, func(task *Task) error {
		if err := task.Join(departure); err != nil {
			return err
		}
		prev := v.ArmNavHooks()
		defer v.DisarmNavHooks(&prev)
		v.Scripts.OpenScene(scene)
		return nil
	})
}
func (v *SetViewer) Turn(dir int, pace ...float64) error {
	if v.Busy() {
		return nil
	}
	target, ring := v.Scene().TurnRing(v.ViewIdx, dir)
	if ring == nil {
		return nil
	}
	images, err := v.Rings.Ensure(&v.Scene().Turns[dir])
	if err != nil {
		return err
	}
	frames, err := v.turnFrames(ring, images)
	if err != nil {
		return err
	}
	p := v.PlayerPace()
	if len(pace) > 0 {
		p = pace[0]
	}
	departure := v.departScene("turn-closescene")
	return v.startAnimation(frames, p, func() error {
		v.ViewIdx = target
		if err := v.ShowView(); err != nil {
			return err
		}
		v.settleScene(departure, v.SceneIdx, "turn-openscene")
		return nil
	})
}

type roomArrival struct{ SceneIdx, ViewIdx int }

func (v *SetViewer) roadArrival(reg *df.FrameRegister, arriveViewID int) *roomArrival {
	scene := -1
	for i, s := range v.Set.Scenes {
		if s.LocationViews == reg.Destination {
			scene = i
			break
		}
	}
	if scene < 0 {
		for i, s := range v.Set.Scenes {
			for _, vw := range s.Views {
				if vw.ViewID == arriveViewID {
					scene = i
					break
				}
			}
			if scene >= 0 {
				break
			}
		}
	}
	if scene < 0 || len(reg.Frames) == 0 {
		return nil
	}
	rotation := reg.Frames[len(reg.Frames)-1].AxisX
	return &roomArrival{scene, nearestView(&v.Set.Scenes[scene], rotation)}
}
func pinCameraHeight(frame *CachedFrame, stand *df.FrameInfo) *CachedFrame {
	if frame.Camera == nil || stand == nil || frame.Camera.Z == float64(stand.PosY16) {
		return frame
	}
	copyFrame := *frame
	pose := *frame.Camera
	pose.Z = float64(stand.PosY16)
	copyFrame.Camera = &pose
	copyFrame.ID = newDrawID()
	return &copyFrame
}
func (v *SetViewer) Walk(pace ...float64) error {
	if v.Busy() {
		return nil
	}
	roads := v.AvailableRoads()
	if len(roads) == 0 {
		return nil
	}
	road := roads[0]
	reg := &road.Transition.FrameRegisters[road.Register]
	images, err := v.Rings.Ensure(reg)
	if err != nil {
		return err
	}
	frames := []*CachedFrame{}
	for _, fi := range reg.Frames {
		if frame := images[fi.FrameContainerLoc]; frame != nil {
			frames = append(frames, frame)
		}
	}
	arrival := v.roadArrival(reg, road.ArriveViewID)
	if len(frames) > 0 {
		frames[0] = pinCameraHeight(frames[0], standFrameInfo(v.Scene(), v.ViewIdx))
		var stand *df.FrameInfo
		if arrival != nil {
			stand = standFrameInfo(&v.Set.Scenes[arrival.SceneIdx], arrival.ViewIdx)
		}
		last := len(frames) - 1
		frames[last] = pinCameraHeight(frames[last], stand)
	}
	if arrival != nil && v.Session.PictureMode == "transition" {
		scene := &v.Set.Scenes[arrival.SceneIdx]
		if fi := standFrameInfo(scene, arrival.ViewIdx); fi != nil {
			own, err := v.Rings.Ensure(&scene.Turns[RightTurns])
			if err != nil {
				return err
			}
			landing, err := v.standpointFrames(fi, own[fi.FrameContainerLoc], scene)
			if err != nil {
				return err
			}
			frames = append(frames, landing...)
		}
	}
	departure := v.departScene("walk-closescene")
	p := v.PlayerPace()
	if len(pace) > 0 {
		p = pace[0]
	}
	return v.startAnimation(frames, p, func() error {
		if arrival != nil {
			v.SceneIdx = arrival.SceneIdx
			v.ViewIdx = arrival.ViewIdx
		}
		v.settleScene(departure, v.SceneIdx, "walk-openscene")
		return v.ShowView()
	})
}
func (v *SetViewer) drainOneEvent() {
	event, ok := v.Session.Events.Take()
	if !ok {
		return
	}
	if event.Kind == "keydown" {
		v.Session.Track("", false, func(*Task) error {
			switch event.Key {
			case "uparrow", "leftarrow", "rightarrow":
				return v.PressNav(event.Key)
			}
			_, err := v.KeyDown(event.Key, event.Special)
			return err
		})
		return
	}
	v.Session.Track("queued click", false, func(*Task) error {
		if v.Director != nil {
			return v.Director.Click(event.X, event.Y)
		}
		_, err := v.RoomClickAt(event.X, event.Y)
		return err
	})
}
