package engine

import "github.com/itskenny0/titanic-godot/internal/df"

func standFrameInfo(scene *df.Scene, view int) *df.FrameInfo {
	for i := range scene.Turns[RightTurns].Frames {
		f := &scene.Turns[RightTurns].Frames[i]
		if f.ViewID == view && f.MotionInfo > 0 {
			return f
		}
	}
	return nil
}
func (v *SetViewer) twinImage(fi *df.FrameInfo, scene *df.Scene, dir, motion int) (*CachedFrame, error) {
	reg := &scene.Turns[dir]
	for _, f := range reg.Frames {
		if f.MotionInfo == motion && f.FramePairID == fi.FramePairID {
			images, err := v.Rings.Ensure(reg)
			if err != nil {
				return nil, err
			}
			return images[f.FrameContainerLoc], nil
		}
	}
	return nil, nil
}
func (v *SetViewer) standpointFrames(fi *df.FrameInfo, own *CachedFrame, scene *df.Scene) ([]*CachedFrame, error) {
	soft, sharp := own, own
	var err error
	if fi.MotionInfo != 1 {
		soft, err = v.twinImage(fi, scene, RightTurns, 1)
		if err != nil {
			return nil, err
		}
	}
	if fi.MotionInfo != 2 {
		sharp, err = v.twinImage(fi, scene, LeftTurns, 2)
		if err != nil {
			return nil, err
		}
	}
	if soft == nil && sharp == nil {
		return nil, nil
	}
	bestSharp, bestSoft := sharp, soft
	if bestSharp == nil {
		bestSharp = soft
	}
	if bestSoft == nil {
		bestSoft = sharp
	}
	both := func() []*CachedFrame {
		if soft != nil && sharp != nil {
			return []*CachedFrame{soft, sharp}
		}
		return []*CachedFrame{bestSharp}
	}
	switch v.Session.PictureMode {
	case "sharp":
		return []*CachedFrame{bestSharp}, nil
	case "soft":
		return []*CachedFrame{bestSoft}, nil
	case "transition":
		return both(), nil
	default:
		if fi.MotionInfo == 1 {
			return both(), nil
		}
		return []*CachedFrame{bestSharp}, nil
	}
}
func (v *SetViewer) standFrame() (*CachedFrame, error) {
	fi := standFrameInfo(v.Scene(), v.ViewIdx)
	if fi == nil {
		return nil, nil
	}
	images, err := v.Rings.Ensure(&v.Scene().Turns[RightTurns])
	if err != nil {
		return nil, err
	}
	own := images[fi.FrameContainerLoc]
	if v.Session.PictureMode == "soft" && own != nil {
		return own, nil
	}
	hi, err := v.twinImage(fi, v.Scene(), LeftTurns, 2)
	if hi != nil || err != nil {
		return hi, err
	}
	return own, nil
}
func (v *SetViewer) turnFrames(ring []df.FrameInfo, images map[int]*CachedFrame) ([]*CachedFrame, error) {
	out := []*CachedFrame{}
	for _, fi := range ring {
		own := images[fi.FrameContainerLoc]
		if fi.MotionInfo > 0 {
			landing, err := v.standpointFrames(&fi, own, v.Scene())
			if err != nil {
				return nil, err
			}
			if len(landing) > 0 {
				out = append(out, landing...)
				continue
			}
		}
		if own != nil {
			out = append(out, own)
		}
	}
	return out, nil
}
func (v *SetViewer) warmNeighbourRing() error {
	if v.warmDone {
		return nil
	}
	candidates := []*df.FrameRegister{&v.Scene().Turns[RightTurns], &v.Scene().Turns[LeftTurns]}
	for _, road := range v.AvailableRoads() {
		candidates = append(candidates, &road.Transition.FrameRegisters[road.Register])
	}
	for _, reg := range candidates {
		if v.Rings.NeedsDecode(reg) {
			_, err := v.Rings.Ensure(reg)
			return err
		}
	}
	v.warmDone = true
	return nil
}
func (v *SetViewer) startAnimation(frames []*CachedFrame, pace float64, done func() error) error {
	if len(frames) == 0 {
		return done()
	}
	v.animation = frames
	v.animationPos = 0
	v.animationPace = pace
	v.animationDone = done
	v.lastTick = 0
	return nil
}
func (v *SetViewer) AdvanceRoom(now float64) (*CachedFrame, error) {
	if v.animation != nil {
		if v.lastTick == 0 {
			v.lastTick = now - v.animationPace
		}
		pace := v.animationPace
		catchUp := pace < EngineStepMS
		for v.animation != nil && now-v.lastTick >= pace {
			if catchUp && pace > 0 {
				v.lastTick += pace
			} else {
				v.lastTick = now
			}
			v.current = v.animation[v.animationPos]
			v.animationPos++
			if v.animationPos >= len(v.animation) {
				done := v.animationDone
				v.animation = nil
				v.animationDone = nil
				if err := done(); err != nil {
					return nil, err
				}
			}
			if !catchUp {
				break
			}
		}
		return v.current, nil
	}
	if (v.Director == nil || !v.Director.MovingCamera()) && v.Session.Events.Len() > 0 {
		v.drainOneEvent()
	}
	if !v.Session.ScriptBusy() {
		if err := v.warmNeighbourRing(); err != nil {
			return nil, err
		}
	}
	return v.current, nil
}
