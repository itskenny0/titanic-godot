package engine

import (
	"github.com/itskenny0/titanic-godot/internal/script"
	"strings"
)

func registerAudioBuiltins(c builtinContext) {
	s := c.s
	var voice PlayHandle
	var voiceName string
	c.r("currentvoice", func([]script.Value, *script.Frame) (script.Value, error) {
		if err := c.yieldFrame(); err != nil {
			return script.Value{}, err
		}
		if voice != nil && !voice.Done() {
			return script.Str(voiceName), nil
		}
		return script.Str(""), nil
	})
	c.action("voicesound", func(a []script.Value, _ *script.Frame) error {
		name := strArg(a, 0)
		audio, err := s.AudioLib.Sound(name)
		if err != nil {
			return err
		}
		if audio == nil {
			banks := strings.Join(s.AudioLib.BankNames(), ", ")
			if banks == "" {
				banks = "none"
			}
			c.log("sound not found: " + name + " (banks: " + banks + ")")
			return nil
		}
		voiceName = strings.ToLower(name)
		voice = s.Audio.Play(VoiceChannel, audio, PlayOptions{})
		return nil
	})
	for _, name := range []string{"singlesound", "multiplesound", "bothsound", "dualsound"} {
		c.action(name, func(a []script.Value, _ *script.Frame) error {
			return s.Scheduler.PlaySound(strArg(a, 0), name == "multiplesound" || name == "dualsound")
		})
	}
	c.action("haltsound", func([]script.Value, *script.Frame) error { s.Scheduler.HaltSounds(); return nil })
	c.action("haltvoice", func([]script.Value, *script.Frame) error { s.Audio.Halt(VoiceChannel); return nil })
	c.action("halttheme", func([]script.Value, *script.Frame) error {
		s.Audio.Halt(ThemeChannel)
		s.CurrentThemeName = "none"
		return nil
	})
	for _, channel := range []AudioChannel{SoundChannel, VoiceChannel} {
		c.r(string(channel)+"done", func([]script.Value, *script.Frame) (script.Value, error) {
			if err := c.yieldFrame(); err != nil {
				return script.Value{}, err
			}
			return script.Bool(s.Audio.IsDone(channel)), nil
		})
	}
	c.r("currentsound", func(a []script.Value, _ *script.Frame) (script.Value, error) {
		if err := c.yieldFrame(); err != nil {
			return script.Value{}, err
		}
		return script.Str(s.Scheduler.CurrentSound(int(numArg(a, 0, 1)))), nil
	})
	for _, name := range []string{"soundvol", "soundpan"} {
		c.v(name, func(a []script.Value, _ *script.Frame) script.Value {
			key := strArg(a, 0)
			get, set := s.Scheduler.GetSoundVol, s.Scheduler.SetSoundVol
			if name == "soundpan" {
				get, set = s.Scheduler.GetSoundPan, s.Scheduler.SetSoundPan
			}
			if len(a) < 2 {
				return script.Num(get(key))
			}
			v := a[1].Num()
			set(key, v)
			return script.Num(v)
		})
	}
	c.action("playtheme", func(a []script.Value, _ *script.Frame) error {
		name := strArg(a, 0)
		theme, err := s.AudioLib.Theme(name)
		if err != nil {
			return err
		}
		if theme == nil {
			line := "playtheme: no theme available"
			if len(a) > 0 {
				line += " (" + name + ")"
			}
			c.log(line)
			return nil
		}
		s.Audio.Play(ThemeChannel, theme, PlayOptions{Loop: true})
		s.CurrentThemeName = arg(a, 0, script.Str("none")).String()
		volume, ok := s.VolumeForTrack(name)
		if name == "" || !ok {
			volume = 255
			if v, found := s.Interp.Globals.Get("themevolume"); found {
				volume = v.Num()
			}
		}
		s.SetThemeVolume(volume, name)
		return nil
	})
	c.v("countsounds", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Num(float64(len(s.AudioLib.SoundNames(strArg(a, 0)))))
	})
	c.v("indextosound", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.AudioLib.SoundNames(strArg(a, 0)), numArg(a, 1, 0)-1))
	})
	c.v("counttracks", func([]script.Value, *script.Frame) script.Value {
		return script.Num(float64(len(s.AudioLib.TrackNames())))
	})
	c.v("indextotrack", func(a []script.Value, _ *script.Frame) script.Value {
		return script.Str(stringIndex(s.AudioLib.TrackNames(), numArg(a, 0, 0)-1))
	})
	c.action("opentrackfile", func(a []script.Value, _ *script.Frame) error { s.OpenTrackFile(strArg(a, 0)); return nil })
	c.action("closetrackfile", func(a []script.Value, _ *script.Frame) error {
		name := strArg(a, 0)
		closing, _ := s.AudioLib.TrackNameOf(name)
		playing, ok := s.AudioLib.TrackNameOf(s.CurrentThemeName)
		if !ok {
			playing = strings.ToLower(s.CurrentThemeName)
		}
		s.AudioLib.CloseBank(name)
		if closing != "" && closing == playing {
			s.Audio.Halt(ThemeChannel)
			s.CurrentThemeName = "none"
		}
		return nil
	})
}
