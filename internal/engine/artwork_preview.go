package engine

import (
	"fmt"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/hdpack"
)

type ArtworkExample struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Pixels []byte `json:"pixels"`
}

// ArtworkExamples reads original controls without booting or advancing a game.
// Original pixels stay local; only the matching SVG artwork ships in the player.
func (p *Player) ArtworkExamples(index map[string]string) ([]ArtworkExample, error) {
	files := NewFiles(index, p.read)
	data, err := files.Provide("house.shp")
	if err != nil {
		return nil, err
	}
	shop, err := df.ReadShop(data)
	if err != nil {
		return nil, err
	}
	data, err = files.Provide("main.stg")
	if err != nil {
		return nil, err
	}
	stage, err := df.ReadStage(data)
	if err != nil {
		return nil, err
	}
	pack, err := hdpack.Open("res://artwork/ui", p.read, nil)
	if err != nil {
		return nil, err
	}
	palette := NewScreenGamma().DisplayPalette(df.PaletteRGBA(stage.PaletteRaw, 256, nil))
	examples := []ArtworkExample{}
	for _, spec := range [][2]string{{"life", "light"}, {"bag", "lightclosed"}, {"navarrow", "green"}} {
		found := false
		for _, group := range shop.Groups {
			if group.Name != spec[0] {
				continue
			}
			for _, state := range group.States {
				if state.Identifier != spec[1] {
					continue
				}
				for _, loc := range state.Frames {
					if loc == 0 {
						continue
					}
					frame, err := df.DecodeSprite(shop.File.Data(loc))
					if err != nil || frame.Width < 1 || frame.Height < 1 || frame.Width > 128 || frame.Height > 128 {
						continue
					}
					pixels := spriteRGBA(&frame, palette)
					key := hdpack.Key(frame.Width, frame.Height, pixels)
					if entry, ok := pack.Manifest.Images[key]; ok && entry.Format == "svg" && entry.Width == frame.Width*2 && entry.Height == frame.Height*2 {
						examples = append(examples, ArtworkExample{spec[0], key, frame.Width, frame.Height, pixels})
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}
	if len(examples) == 0 {
		return nil, fmt.Errorf("no matching interface examples in these game files")
	}
	return examples, nil
}
