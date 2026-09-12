package engine

// This adapter keeps the scheduler's narrow contract separate from script
// dispatch, which also carries arguments and parent interpreter frames.
type sessionScheduler struct{ session *Session }

func (h sessionScheduler) CurrentSet() string          { return h.session.SetName }
func (h sessionScheduler) CurrentFlatName() string     { return h.session.StageCtrl.CurrentFlat }
func (h sessionScheduler) ScriptBusy() bool            { return h.session.ScriptBusy() }
func (h sessionScheduler) AmbientRandom() float64      { return h.session.AmbientRandom() }
func (h sessionScheduler) ListenerPosition() *Listener { return h.session.Listener() }
func (h sessionScheduler) HasHandler(kind, name, handler string) bool {
	switch kind {
	case "actor":
		return hasHandler(h.session.CastScripts.Get(name), handler)
	case "flat":
		return hasHandler(h.session.FlatScripts.Get(name), handler)
	}
	return false
}
func (h sessionScheduler) HasGlobal(name string) bool { return h.session.HasGlobal(name) }
func (h sessionScheduler) RunGlobal(_ *Task, name string) error {
	_, err := h.session.RunGlobal(name, nil)
	return err
}
func (h sessionScheduler) SendEvent(_ *Task, command, target, handler, caller string) error {
	_, err := h.session.SendEvent(command, target, handler, nil, caller, nil)
	return err
}
func (h sessionScheduler) Track(name string, idle bool, run func(*Task) error) {
	h.session.Track(name, idle, run)
}
func (h sessionScheduler) WithNavigation(run func() error) error {
	s := h.session
	nav, scene, view, active, fromScript := s.OnNavigate, s.OnSceneJump, s.OnViewJump, s.NavGestureActive, s.NavFromScript
	s.OnNavigate, s.OnSceneJump, s.OnViewJump = s.NavDriver, s.SceneJumpDriver, s.ViewJumpDriver
	s.NavGestureActive, s.NavFromScript = true, true
	defer func() {
		s.OnNavigate, s.OnSceneJump, s.OnViewJump = nav, scene, view
		s.NavGestureActive, s.NavFromScript = active, fromScript
	}()
	return run()
}
func (h sessionScheduler) AdvanceClock(now float64) { h.session.Executor.AdvanceClock(now) }
func (h sessionScheduler) Log(line string)          { h.session.Log(line) }
