// Package hdexport extracts decoded artwork from the owner's files. Outputs
// are personal data, never public release inputs. Original files stay untouched.
package hdexport

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/engine"
	"github.com/itskenny0/titanic-godot/internal/hdpack"
	"github.com/itskenny0/titanic-godot/internal/personalbuild"
)

type Source struct {
	File string `json:"file"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}
type Image struct {
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Sources []Source `json:"sources"`
}
type Catalog struct {
	Version int               `json:"version"`
	Images  map[string]*Image `json:"images"`
}
type Exporter struct {
	Root          string
	Motion        bool
	Characters    bool
	StagePalettes map[string][]byte
	Catalog       Catalog
	gamma         *engine.ScreenGamma
}

func New(root string, motion bool) (*Exporter, error) {
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return nil, err
	}
	if err := os.Mkdir(root, 0700); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(root, "originals"), 0700); err != nil {
		return nil, err
	}
	return &Exporter{Root: root, Motion: motion, Catalog: Catalog{1, map[string]*Image{}}, gamma: engine.NewScreenGamma()}, nil
}
func (e *Exporter) image(pixels, opaque, palette []byte, w, h int, source Source) error {
	if w < 1 || h < 1 || w > 512 || h > 384 {
		return nil
	}
	rgba := make([]byte, w*h*4)
	if err := df.IndexedRGBA(pixels, palette, rgba); err != nil {
		return err
	}
	if opaque != nil {
		for i, on := range opaque {
			if on == 0 {
				clear(rgba[i*4 : i*4+4])
			}
		}
	}
	return e.rgba(rgba, w, h, source)
}

func (e *Exporter) rgba(rgba []byte, w, h int, source Source) error {
	key := hdpack.Key(w, h, rgba)
	if prior := e.Catalog.Images[key]; prior != nil {
		for _, existing := range prior.Sources {
			if existing == source {
				return nil
			}
		}
		prior.Sources = append(prior.Sources, source)
		return nil
	}
	p := filepath.Join(e.Root, "originals", key+".png")
	file, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		existing, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		decoded, decodeErr := png.Decode(bytes.NewReader(existing))
		if decodeErr != nil {
			return decodeErr
		}
		restored := image.NewNRGBA(image.Rect(0, 0, w, h))
		if decoded.Bounds().Dx() != w || decoded.Bounds().Dy() != h {
			return fmt.Errorf("existing export dimensions changed: %s", p)
		}
		draw.Draw(restored, restored.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
		if !bytes.Equal(restored.Pix, rgba) {
			return fmt.Errorf("existing export pixels changed: %s", p)
		}
		e.Catalog.Images[key] = &Image{w, h, []Source{source}}
		return nil
	}
	if err != nil {
		return err
	}
	img := &image.NRGBA{Pix: rgba, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	err = png.Encode(file, img)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	e.Catalog.Images[key] = &Image{w, h, []Source{source}}
	return nil
}
func (e *Exporter) Add(name string, data []byte) error {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pup":
		if !e.Characters {
			return nil
		}
		pup, err := df.ReadPuppet(data)
		if err != nil {
			return err
		}
		palette := e.gamma.DisplayPalette(df.PaletteRGBA(pup.PaletteRaw, 256, binary.LittleEndian))
		frames := map[int]*df.Sprite{}
		readFrame := func(loc int) (*df.Sprite, error) {
			if f := frames[loc]; f != nil {
				return f, nil
			}
			f, err := df.DecodeSprite(pup.File.Data(loc))
			if err != nil {
				return nil, err
			}
			frames[loc] = &f
			return &f, nil
		}
		seen := map[string]bool{}
		for _, ident := range pup.DialogueOrder {
			line := pup.Dialogue[ident]
			for i, pose := range pup.AnimLogic(line.AnimLogicLocation) {
				key := engine.PuppetPoseKey(line.Stance, &pose)
				if seen[key] {
					continue
				}
				seen[key] = true
				art, err := engine.PuppetArtwork(pup, line.Stance, &pose, palette, readFrame)
				if err != nil {
					return err
				}
				if art != nil {
					if err = e.rgba(art.Pix, art.Rect.Dx(), art.Rect.Dy(), Source{name, "character", fmt.Sprintf("%s/frame%d", ident, i)}); err != nil {
						return err
					}
				}
			}
		}
	case ".set":
		set, err := df.ReadSet(data)
		if err != nil {
			return err
		}
		palette := e.gamma.DisplayPalette(df.PaletteRGBA(set.PaletteRaw, set.ColorCount, set.File.Order))
		ring := func(reg *df.FrameRegister, label string, sharpPairs map[int]bool) error {
			fb := new(df.FrameBuffer)
			for _, fi := range reg.Frames {
				if fi.FrameContainerLoc == 0 {
					continue
				}
				f, err := df.DecodeFrame(set.File.Data(fi.FrameContainerLoc), fb, set.File.Order)
				if err != nil {
					return err
				}
				if !e.Motion && (fi.MotionInfo == 0 || (fi.MotionInfo != 2 && sharpPairs[fi.FramePairID])) {
					continue
				}
				if err = e.image(fb.Pixels[:f.Width*f.Height], nil, palette, f.Width, f.Height, Source{name, "room", fmt.Sprintf("%s/%d", label, fi.FrameContainerLoc)}); err != nil {
					return err
				}
			}
			return nil
		}
		for i := range set.Scenes {
			// The player selects the sharp standpoint when a pair exists.
			// Preserve the sole available frame in scenes without a sharp twin.
			sharpPairs := map[int]bool{}
			for _, frame := range set.Scenes[i].Turns[engine.LeftTurns].Frames {
				if frame.MotionInfo == 2 && frame.FrameContainerLoc != 0 {
					sharpPairs[frame.FramePairID] = true
				}
			}
			for j := range set.Scenes[i].Turns {
				if err = ring(&set.Scenes[i].Turns[j], set.Scenes[i].SceneName, sharpPairs); err != nil {
					return err
				}
			}
		}
		if e.Motion {
			for i := range set.Transitions {
				for j := range set.Transitions[i].FrameRegisters {
					if err = ring(&set.Transitions[i].FrameRegisters[j], set.Transitions[i].TransitionName, nil); err != nil {
						return err
					}
				}
			}
		}
	case ".stg":
		stage, err := df.ReadStage(data)
		if err != nil {
			return err
		}
		palette := e.gamma.DisplayPalette(df.PaletteRGBA(stage.PaletteRaw, 256, nil))
		for _, flat := range stage.Flats {
			if flat.LocationFrame == 0 {
				continue
			}
			fb := new(df.FrameBuffer)
			f, err := df.DecodeFrame(stage.File.Data(flat.LocationFrame), fb, nil)
			if err != nil {
				return err
			}
			if err = e.image(fb.Pixels[:f.Width*f.Height], nil, palette, f.Width, f.Height, Source{name, "ui", flat.Name}); err != nil {
				return err
			}
		}
	case ".mov":
		movie, err := df.ReadMovie(data)
		if err != nil {
			return err
		}
		for j := range movie.Segments {
			segment := &movie.Segments[j]
			frames := engine.NewMovieFrames(segment)
			palette := e.gamma.DisplayPalette(df.PaletteRGBA(segment.PaletteRaw, 256, segment.File.Order))
			for i, meta := range segment.Frames {
				if len(meta.Regions) == 0 {
					continue
				} // interactive menu/puzzle artwork, not cinematics
				frame, err := frames.Get(i)
				if err != nil {
					return err
				}
				if frame == nil {
					continue
				}
				if err = e.image(frame.Pixels, nil, palette, frame.Width, frame.Height, Source{name, "ui", fmt.Sprintf("segment%d/frame%d", j, i)}); err != nil {
					return err
				}
			}
		}
	case ".shp":
		shop, err := df.ReadShop(data)
		if err != nil {
			return err
		}
		palettes := [][]byte{e.gamma.DisplayPalette(df.PaletteRGBA(shop.PaletteRaw, 256, nil))}
		stem := strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".shp")
		// Props use the active stage's palette, not necessarily the palette
		// saved in their shop. Include authored UI and puzzle-stage variants.
		if palette := e.StagePalettes[stem]; palette != nil {
			palettes = append(palettes, palette)
		}
		kind := "sprite"
		if stem == "house" || stem == "inven" {
			kind = "ui"
			if palette := e.StagePalettes["main"]; palette != nil {
				palettes = append(palettes, palette)
			}
		}
		for _, palette := range palettes {
			for _, group := range shop.Groups {
				for _, state := range group.States {
					for _, loc := range state.Frames {
						if loc == 0 {
							continue
						}
						f, err := df.DecodeSprite(shop.File.Data(loc))
						if err != nil {
							return err
						}
						if err = e.image(f.Indexed, f.Opaque, palette, f.Width, f.Height, Source{name, kind, group.Name + "/" + state.Identifier}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}
func (e *Exporter) Finish() error {
	b, err := json.MarshalIndent(e.Catalog, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.Root, "catalog.json"), append(b, '\n'), 0600)
}
func Export(game, manifest, out, mods string, motion, resume, characters bool) error {
	sources, err := personalbuild.GameSources(game, manifest)
	if err != nil {
		return err
	}
	var e *Exporter
	if resume {
		b, err := os.ReadFile(filepath.Join(out, "catalog.json"))
		if err != nil {
			return err
		}
		e = &Exporter{Root: out, Motion: motion, gamma: engine.NewScreenGamma()}
		if err = json.Unmarshal(b, &e.Catalog); err != nil {
			return err
		}
		if e.Catalog.Version != 1 || e.Catalog.Images == nil {
			return fmt.Errorf("invalid export catalog")
		}
		// Reuse existing image files, but rebuild the inventory for these options.
		e.Catalog.Images = map[string]*Image{}
	} else {
		e, err = New(out, motion)
		if err != nil {
			return err
		}
	}
	e.Characters = characters
	e.StagePalettes = map[string][]byte{}
	for _, source := range sources {
		if strings.EqualFold(filepath.Ext(source.Name), ".stg") {
			data, err := source.Read()
			if err != nil {
				return err
			}
			stage, err := df.ReadStage(data)
			if err != nil {
				return err
			}
			stem := strings.TrimSuffix(strings.ToLower(filepath.Base(source.Name)), ".stg")
			e.StagePalettes[stem] = e.gamma.DisplayPalette(df.PaletteRGBA(stage.PaletteRaw, 256, nil))
		}
	}
	for _, s := range sources {
		ext := strings.ToLower(filepath.Ext(s.Name))
		if ext != ".set" && ext != ".stg" && ext != ".shp" && ext != ".mov" && !(characters && ext == ".pup") {
			continue
		}
		fmt.Println("Exporting", s.Name)
		data, err := s.Read()
		if err != nil {
			return err
		}
		if err = e.Add(s.Name, data); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}
	if mods != "" {
		err = filepath.WalkDir(mods, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("mod source is a symlink: %s", p)
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".set" && ext != ".stg" && ext != ".shp" && ext != ".mov" && !(characters && ext == ".pup") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fmt.Println("Exporting mod", p)
			return e.Add("mods/"+filepath.Base(p), data)
		})
		if err != nil {
			return err
		}
	}
	fmt.Printf("Exported %d unique artwork images.\n", len(e.Catalog.Images))
	return e.Finish()
}
