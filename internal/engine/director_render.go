package engine

import (
	"math"
	"sort"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func (d *ScreenDirector) BuildSignature(ctx *DrawContext) *DrawSignature {
	s := d.Session
	sig := d.sig.Reset()
	sig.Num(float64(ctx.Width)).Num(float64(ctx.Height)).Bool(d.Conversing()).Bool(s.ViewShowing())
	sig.ID(d.snapshotRef.token(s.Fade.Snapshot)).Num(s.Fade.Level)
	sig.Str(d.Movies.PlayingFile()).Num(float64(d.Movies.FramePos()))
	sig.ID(d.roomRef.token(d.roomFrame())).ID(d.flatRef.token(d.flatImage()))
	if xr := s.XRay; xr == nil || !xr.Aimed {
		sig.Bool(false)
	} else {
		sig.Str(xr.Hidden).Str(xr.Mask).Num(xr.X).Num(xr.Y)
	}
	sig.ID(d.dimRef.token(d.stageDim))
	if c := d.roomCamera(); c == nil {
		sig.Bool(false)
	} else {
		sig.Num(c.X).Num(c.Y).Num(c.Z).Num(c.Deg).Num(c.F).Num(c.CX).Num(c.CY).Num(float64(c.ClipW)).Num(float64(c.ClipH))
	}
	if d.room != nil {
		d.room.RoomSignature(sig)
	}
	s.Actors.DrawSignature(sig)
	s.Props.DrawSignature(sig)
	d.PuppetView.DrawSignature(sig)
	var photo *RGBAFrame
	if s.PhotoOverlay != nil {
		photo = s.PhotoOverlay.Photo
	}
	sig.ID(d.photoRef.token(photo)).Num(float64(len(s.TextOverlay)))
	for _, e := range s.TextOverlay {
		sig.Str(e.Text).Num(e.X).Num(e.Y).Num(e.Size).Num(e.Color)
	}
	sig.Num(s.Wipe.Step).Str(s.Wipe.Dir).Num(s.Wipe.Span).Bool(s.Wipe.Settled)
	// A held movie or conversation must also repaint after a gamma adjustment.
	sig.ID(d.Gamma.Generation)
	return sig
}
func (d *ScreenDirector) Render(ctx *DrawContext) error {
	if d.ScreenOwner() == "held" {
		return nil
	}
	if d.Screen.ShouldPaint(d.BuildSignature(ctx)) {
		return d.paint(ctx)
	}
	return nil
}
func (d *ScreenDirector) CompositePuppetScreen() error {
	screen := d.Screen
	screen.ClearFrame()
	if flat := d.flatImage(); flat != nil {
		pal := d.FlatPalette(flat.Palette)
		buf := screen.ScratchFor(flat.Width * flat.Height * 4)
		if err := df.IndexedRGBA(flat.Pixels, pal, buf[:flat.Width*flat.Height*4]); err != nil {
			return err
		}
		screen.BlitTop(buf, flat.Width, flat.Height)
		if err := d.compositeWorld(screen.Frame, pal, nil); err != nil {
			return err
		}
	}
	var backdrop *PuppetBackdrop
	if cur := d.roomFrame(); cur != nil {
		backdrop = &PuppetBackdrop{Pixels: cur.Pixels, Width: cur.Width, Height: cur.Height, Palette: d.room.RoomPalette()}
	}
	if err := d.PuppetView.Composite(screen.Frame, backdrop); err != nil {
		return err
	}
	screen.FrameValid = true
	if screen.HD != nil {
		screen.HD.Valid = false
	} // Puppet composition writes the logical screen directly.
	return nil
}
func (d *ScreenDirector) paint(ctx *DrawContext) error {
	owner := d.ScreenOwner()
	s, screen := d.Session, d.Screen
	if owner == "puppet" {
		if err := d.CompositePuppetScreen(); err != nil {
			return err
		}
		screen.Blit(ctx)
		d.PuppetView.DrawOverlay(ctx)
		screen.ApplyFade(ctx, s.Fade.Level)
		return nil
	}
	if owner == "movie" {
		f, err := d.Movies.Frame()
		if err != nil {
			return err
		}
		if f != nil {
			covers := f.OriginX <= 0 && f.OriginY <= 0 && f.Width >= screen.Width && f.Height >= screen.Height
			if covers {
				screen.ClearFrame()
			} else {
				drew, err := d.PaintWorldInto()
				if err != nil {
					return err
				}
				if drew == "" {
					screen.ClearFrame()
				}
			}
			buf := screen.ScratchFor(f.Width * f.Height * 4)
			if err := df.IndexedRGBA(f.Pixels, f.Palette, buf[:f.Width*f.Height*4]); err != nil {
				return err
			}
			screen.BlitAt(buf, f.Width, f.Height, f.OriginX, f.OriginY)
			screen.FrameValid = true
			screen.Blit(ctx)
			if !covers {
				screen.ApplyFadeExcept(ctx, s.Fade.Level, ScreenRect{float64(f.OriginX), float64(f.OriginY), float64(f.Width), float64(f.Height)})
			}
			return nil
		}
	}
	if owner == "faded" && s.Fade.Snapshot != nil {
		f := s.Fade.Snapshot
		screen.ClearFrame()
		screen.BlitTop(f.RGBA, f.Width, f.Height)
		screen.FrameValid = true
		screen.Blit(ctx)
		screen.ApplyFade(ctx, s.Fade.Level)
		return nil
	}
	drew, err := d.PaintWorldInto()
	if err != nil || drew == "" {
		return err
	}
	d.coverWithWipe()
	if shot := s.PhotoOverlay; shot != nil && shot.Photo != nil && shot.Photo.Width > 0 && shot.Photo.Height > 0 {
		screen.BlitAt(shot.Photo.RGBA, shot.Photo.Width, shot.Photo.Height, shot.X, shot.Y)
	}
	screen.FrameValid = true
	screen.Blit(ctx)
	screen.DrawTextOverlay(ctx, s.TextOverlay)
	screen.ApplyFade(ctx, s.Fade.Level)
	if (drew == "set" || s.ViewShowing()) && d.room != nil {
		d.room.DrawRoomHotspots(ctx)
	}
	return nil
}
func (d *ScreenDirector) flatIsMatte(flat *FlatImage) bool {
	if d.room == nil {
		return false
	}
	if sameBytes(d.matteSeen.pixels, flat.Pixels) {
		return d.matteSeen.matte
	}
	matte := flat.Width >= d.Screen.Width && flat.Height >= PuppetArtHeight
	if matte {
		first := flat.Pixels[0]
	scan:
		for y := 0; y < PuppetArtHeight; y++ {
			for x := 0; x < d.Screen.Width; x++ {
				if flat.Pixels[y*flat.Width+x] != first {
					matte = false
					break scan
				}
			}
		}
	}
	d.matteSeen.pixels, d.matteSeen.matte = flat.Pixels, matte
	return matte
}
func (d *ScreenDirector) PaintWorldInto() (string, error) {
	s, screen := d.Session, d.Screen
	flat, cur := d.flatImage(), d.roomFrame()
	if flat != nil {
		if !(s.ViewShowing() && cur != nil) && d.flatIsMatte(flat) {
			return "", nil
		}
		screen.ClearFrame()
		flatPal := d.FlatPalette(flat.Palette)
		buf := screen.ScratchFor(flat.Width * flat.Height * 4)
		if err := df.IndexedRGBA(flat.Pixels, flatPal, buf[:flat.Width*flat.Height*4]); err != nil {
			return "", err
		}
		screen.BlitTop(buf, flat.Width, flat.Height)
		if s.ViewShowing() && cur != nil {
			buf = screen.ScratchFor(cur.Width * cur.Height * 4)
			if err := df.IndexedRGBA(cur.Pixels, d.room.RoomPalette(), buf[:cur.Width*cur.Height*4]); err != nil {
				return "", err
			}
			screen.BlitTop(buf, cur.Width, cur.Height)
		}
		if err := d.compositeXRay(flatPal); err != nil {
			return "", err
		}
		propPal := flatPal
		var cam *WorldCamera
		if s.ViewShowing() {
			cam = d.roomCamera()
			if d.room != nil {
				propPal = d.room.BandPropPalette(flat.Palette)
			}
		}
		if err := d.compositeWorld(screen.Frame, propPal, cam); err != nil {
			return "", err
		}
		return "flat", nil
	}
	if cur == nil {
		return "", nil
	}
	screen.ClearFrame()
	buf := screen.ScratchFor(cur.Width * cur.Height * 4)
	if err := df.IndexedRGBA(cur.Pixels, d.room.RoomPalette(), buf[:cur.Width*cur.Height*4]); err != nil {
		return "", err
	}
	screen.BlitTop(buf, cur.Width, cur.Height)
	if err := d.compositeWorld(screen.Frame, d.room.RoomPropPalette(), d.room.RoomCamera()); err != nil {
		return "", err
	}
	return "set", nil
}
func (d *ScreenDirector) compositeXRay(pal []byte) error {
	xr := d.Session.XRay
	if xr == nil || !xr.Aimed {
		return nil
	}
	if d.Screen.HD != nil {
		d.Screen.HD.Valid = false
	}
	hidden := d.Session.StageCtrl.FlatImage(xr.Hidden)
	mask := d.Session.Props.Get(xr.Mask)
	if hidden == nil || mask == nil {
		return nil
	}
	st := mask.State()
	if st == nil || len(st.Frames) == 0 {
		return nil
	}
	f, err := mask.frame(st, mask.CurrentFrameIdx(st))
	if err != nil {
		return err
	}
	dx, dy := xr.X-float64(f.PosXraw), xr.Y-float64(f.PosYraw)
	screen := d.Screen
	for y := 0; y < f.Height; y++ {
		ty := dy + float64(y)
		if ty < 0 || ty >= float64(min(screen.Height, hidden.Height)) {
			continue
		}
		for x := 0; x < f.Width; x++ {
			tx := dx + float64(x)
			if tx < 0 || tx >= float64(min(screen.Width, hidden.Width)) || f.Opaque[y*f.Width+x] == 0 {
				continue
			}
			from, to := float64(ty*float64(hidden.Width))+tx, float64((float64(ty*float64(screen.Width))+tx)*4)
			if to != math.Trunc(to) {
				continue
			}
			dest := int(to)
			// A fractional indexed-array lookup is undefined and writes black RGB.
			if from != math.Trunc(from) {
				clear(screen.Frame[dest:min(dest+3, len(screen.Frame))])
				continue
			}
			c := int(hidden.Pixels[int(from)]) * 4
			copy(screen.Frame[dest:min(dest+3, len(screen.Frame))], pal[c:c+3])
		}
	}
	return nil
}
func (d *ScreenDirector) compositeWorld(data, palette []byte, cam *WorldCamera) error {
	s := d.Session
	w, h := d.Screen.Width, d.Screen.Height
	animating := d.roomAnimating()
	occ := d.roomOcclusion()
	if cam != nil && d.room != nil && d.room.RoomVersion() == 1 {
		type job struct {
			depth float64
			draw  func() error
		}
		jobs := []job{}
		for _, e := range s.Actors.DrawList(*cam) {
			jobs = append(jobs, job{e.Proj.Depth, func() error { return s.Actors.CompositeOne(e, data, w, h, palette, *cam, occ, d.Screen.HD) }})
		}
		for _, e := range s.Props.WorldDrawList(*cam) {
			jobs = append(jobs, job{e.Proj.Depth, func() error { return s.Props.CompositeWorldOne(e, data, w, h, palette, *cam, occ, d.Screen.HD) }})
		}
		sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].depth > jobs[j].depth })
		for _, j := range jobs {
			if err := j.draw(); err != nil {
				return err
			}
		}
		if err := s.Actors.CompositeScreen(data, w, h, palette, d.Screen.HD); err != nil {
			return err
		}
		return s.Props.Composite(data, w, h, palette, math.Inf(-1), nil, animating || s.ViewShowing(), occ, d.Screen.HD)
	}
	if cam != nil {
		if err := s.Actors.Composite(data, w, h, palette, *cam, occ, d.Screen.HD); err != nil {
			return err
		}
	}
	if err := s.Actors.CompositeScreen(data, w, h, palette, d.Screen.HD); err != nil {
		return err
	}
	return s.Props.Composite(data, w, h, palette, math.Inf(-1), cam, animating || s.ViewShowing(), occ, d.Screen.HD)
}
