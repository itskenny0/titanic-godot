package engine

import (
	"fmt"
	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
	"os"
	"strings"
	"testing"
)

func adaptiveTestPlayer(t *testing.T) *Player {
	t.Helper()
	p := NewPlayer(&testPlayerBridge{})
	if err := p.Boot(PlayerConfig{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	s, d := p.Host.Session, p.Host.Director
	p.Ready = true
	s.Fade = FadeState{}
	pal := make([]byte, 1024)
	pal[4], pal[5], pal[6] = 91, 122, 153
	d.SetRoom(&directorTestRoom{frame: &CachedFrame{Width: 512, Height: 264, Pixels: make([]byte, 512*264)}, palette: pal})
	s.SetName, s.SetVisible = "test", true
	s.StageCtrl.Name, s.StageCtrl.CurrentFlat = "main.stg", "main 1"
	s.StageCtrl.File = &df.Stage{}
	s.StageCtrl.images["main.stg:main 1"] = &FlatImage{Width: 512, Height: 384, Pixels: make([]byte, 512*384), Palette: pal}
	groups := []df.PropGroup{}
	for _, name := range []string{"life", "bag", "watch", "map", "navarrow"} {
		groups = append(groups, df.PropGroup{Name: name, States: []df.PropState{{Identifier: "lightclosed", Frames: []int{1}, RefScales: []int{96}}}})
	}
	shop := s.Props.AddShop("house.shp", &df.Shop{Groups: groups})
	seedSprites(shop)
	shop.Persistent = true
	for i, g := range groups {
		prop := s.Props.Get(g.Name)
		prop.Visible = true
		prop.ScreenPlaced = true
		prop.StateName = "lightclosed"
		prop.AnchorX = float64(100 + i*30)
		prop.AnchorY = 300
	}
	return p
}

func TestAdaptiveControlsKeepOriginalHitTesting(t *testing.T) {
	p := adaptiveTestPlayer(t)
	layout := p.AdaptiveLayout()
	if !layout.Eligible || len(layout.Controls) != 5 {
		t.Fatalf("layout: %+v", layout)
	}
	atlas := p.AdaptiveAtlas()
	if len(atlas) != 640*128*4 {
		t.Fatal("atlas size", len(atlas))
	}
	for _, c := range layout.Controls {
		hit, err := p.Host.Director.HitTestAt(c.AimX, c.AimY)
		if err != nil || hit.Name != c.Name {
			t.Fatalf("%s aims at %v (%v)", c.Name, hit, err)
		}
		dst := c.Slot * 128 * 4
		if atlas[dst] != 91 || atlas[dst+1] != 122 || atlas[dst+2] != 153 || atlas[dst+3] != 255 || atlas[dst+7] != 0 {
			t.Fatal("sprite palette/alpha lost", c.Name)
		}
	}
}

func TestAdaptiveUnsafeScreensFallBack(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*Player)
	}{
		{"held item", func(p *Player) { p.Host.Session.Interp.Globals.Set("handitem", script.Str("key")) }},
		{"inventory", func(p *Player) { p.Host.Session.StageCtrl.Name = "inven.stg" }},
		{"other flat", func(p *Player) { p.Host.Session.StageCtrl.CurrentFlat = "puzzle" }},
		{"dialogue", func(p *Player) { p.Host.Session.PuppetCtrl.Puppet = &PuppetState{Visible: true, Pup: &df.Puppet{}} }},

		{"text", func(p *Player) { p.Host.Session.TextOverlay = []TextOverlay{{Text: "story", Y: 300}} }},
		{"open bag", func(p *Player) { p.Host.Session.Props.Get("bag").StateName = "lightopen" }},
		{"animation", func(p *Player) { p.Host.Session.Props.Get("watch").Animating = true }},
		{"unexpected prop", func(p *Player) { p.Host.Session.Props.Instance("life", "new ui") }},
		{"missing control", func(p *Player) { p.Host.Session.Props.Get("map").Visible = false }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			p := adaptiveTestPlayer(t)
			scenario.change(p)
			if p.AdaptiveLayout().Eligible || p.AdaptiveAtlas() != nil {
				t.Fatal("unsafe layout rearranged")
			}
		})
	}
}

func TestAdaptiveDoorsAndSignsStayInWorld(t *testing.T) {
	for _, name := range []string{"door", "signs"} {
		t.Run(name, func(t *testing.T) {
			p := adaptiveTestPlayer(t)
			p.Host.Session.Props.Instance("life", name)
			prop := p.Host.Session.Props.Get(name)
			prop.AnchorX, prop.AnchorY = 256, 100
			prop.Visible = true
			if !p.AdaptiveLayout().Eligible {
				t.Fatal("world sprite restored Classic", p.AdaptiveLayout())
			}
			prop.AnchorY = 300
			if p.AdaptiveLayout().Eligible {
				t.Fatal("sprite extending into the toolbar must retain Classic")
			}
		})
	}
}

func checkOwnedAdaptiveControls(t *testing.T, p *Player) {
	t.Helper()
	if a := p.AdaptiveLayout(); !a.Eligible {
		t.Fatalf("owned exploration should support adaptive controls: %+v", a)
	}
	for _, c := range p.AdaptiveLayout().Controls {
		hit, err := p.Host.Director.HitTestAt(c.AimX, c.AimY)
		if err != nil || hit.Name != c.Name {
			t.Fatalf("original control %s not reachable", c.Name)
		}
	}
	s := p.Host.Session
	hand, exists := s.Interp.Globals.Get("handitem")
	s.Interp.Globals.Set("handitem", script.Str("newly acquired item"))
	if p.AdaptiveLayout().Eligible {
		t.Fatal("item acquisition did not restore original toolbar")
	}
	if !exists {
		hand = script.Str("")
	}
	s.Interp.Globals.Set("handitem", hand)
	if !p.AdaptiveLayout().Eligible {
		t.Fatal("stowing item did not restore adaptive layout")
	}
}

func TestNativeAdaptiveSavedGameIntegration(t *testing.T) {
	path := os.Getenv("TAOOT_ADAPTIVE_SAVE")
	if path == "" {
		t.Skip("set TAOOT_ADAPTIVE_SAVE for an owned normal exploration save")
	}
	files := ownedFiles(t)
	p := NewPlayer(&testPlayerBridge{read: os.ReadFile})
	defer p.Close()
	if err := p.Boot(PlayerConfig{Index: files.Index, Save: path, DisableAutosave: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1200; i++ {
		if err := p.Tick(50); err != nil {
			t.Fatal(err)
		}
		for _, event := range p.Events() {
			if a, ok := event.(AudioEvent); ok && a.Type == "audio_play" {
				p.TakeAudio(a.ID)
				if !a.Loop {
					p.Command(PlayerCommand{Action: "audio_done", ID: a.ID})
				}
			}
		}
		if i >= 20 && p.AdaptiveLayout().Eligible {
			checkOwnedAdaptiveControls(t, p)
			checkOwnedAdaptiveDoorsAndSigns(t, p)
			checkOwnedAdaptiveBag(t, p)
			return
		}
	}
	t.Fatal("saved exploration never supports adaptive controls", p.State(), p.AdaptiveLayout())
}

func checkOwnedAdaptiveDoorsAndSigns(t *testing.T, p *Player) {
	t.Helper()
	for _, name := range []string{"door", "signs"} {
		prop := p.Host.Session.Props.Get(name)
		if prop == nil {
			t.Fatal("missing owned world sprite", name)
		}
		original := *prop
		prop.Visible, prop.Hidden = true, false
		prop.AnchorX, prop.AnchorY = 256, 192
		for _, state := range prop.Group.States {
			prop.StateName = state.Identifier
			for frame := range state.Frames {
				prop.FrameIdx, prop.FrameOrder = 0, []int{frame}
				if !p.AdaptiveLayout().Eligible {
					t.Fatalf("%s %s frame %d restored Classic: %v", name, state.Identifier, frame, p.AdaptiveLayout())
				}
			}
		}
		*prop = original
	}
}

func checkOwnedAdaptiveBag(t *testing.T, p *Player) {
	t.Helper()
	s := p.Host.Session
	step := func() {
		if err := p.Tick(50); err != nil {
			t.Fatal(err)
		}
		for _, raw := range p.Events() {
			if a, ok := raw.(AudioEvent); ok && a.Type == "audio_play" {
				p.TakeAudio(a.ID)
				if !a.Loop {
					p.Command(PlayerCommand{Action: "audio_done", ID: a.ID})
				}
			}
		}
	}
	wait := func(label string, done func() bool) {
		t.Helper()
		for i := 0; i < 300; i++ {
			step()
			if done() {
				return
			}
		}
		hand, _ := s.Interp.Globals.Get("handitem")
		t.Fatal(label, p.State(), p.AdaptiveLayout(), "hand", hand, "targets", p.ControllerSurface(true))
	}
	click := func(x, y float64) {
		p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: x, Y: y})
		p.Command(PlayerCommand{Action: "pointer", Kind: "release", X: x, Y: y})
	}
	for _, c := range p.AdaptiveLayout().Controls {
		if c.Name == "bag" {
			click(c.AimX, c.AimY)
			break
		}
	}
	wait("bag opens", func() bool { return strings.HasPrefix(s.StageCtrl.Name, "inven") && !p.Host.Director.InputLocked() })
	if p.AdaptiveLayout().Eligible {
		t.Fatal("inventory must use Classic")
	}
	var item string
	for _, c := range p.ControllerSurface(true).Targets {
		if strings.HasPrefix(c.ID, "prop:") {
			name := strings.TrimPrefix(c.ID, "prop:")
			prop := s.Props.Get(name)
			if prop != nil && prop.Shop.Name == "inven.shp" {
				item = name
				click(c.AimX, c.AimY)
				break
			}
		}
	}
	if item == "" {
		t.Fatal("no inventory item in owned save")
	}
	wait("item selected", func() bool {
		hand, _ := s.Interp.Globals.Get("handitem")
		return hand.Text == item && !p.Host.Director.InputLocked()
	})
	if s.StageCtrl.Name != "main.stg" {
		found := false
		for _, c := range p.ControllerSurface(true).Targets {
			if strings.HasSuffix(c.ID, ":ok") {
				click(c.AimX, c.AimY)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("inventory OK control missing", p.ControllerSurface(true))
		}
		wait("leave inventory", func() bool { return s.StageCtrl.Name == "main.stg" && !p.Host.Director.InputLocked() })
	}
	if p.AdaptiveLayout().Eligible {
		t.Fatal("selected item must keep original bottom bar")
	}
	// Drag the actual item back onto the original bag using script coordinates.
	r, err := s.Props.Get(item).ScreenRect()
	if err != nil || r == nil {
		t.Fatal("held item missing", err)
	}
	x, y := r.X+float64(r.W)/2, r.Y+float64(r.H)/2
	for _, c := range p.ControllerSurface(true).Targets {
		if c.ID == "prop:"+item {
			x, y = c.AimX, c.AimY
		}
	}
	p.Command(PlayerCommand{Action: "pointer", Kind: "press", X: x, Y: y})
	step()
	p.Command(PlayerCommand{Action: "pointer", Kind: "move", X: 199, Y: 325})
	step()
	p.Command(PlayerCommand{Action: "pointer", Kind: "release", X: 199, Y: 325})
	wait("item returned to bag", func() bool {
		hand, _ := s.Interp.Globals.Get("handitem")
		return hand.Text == "" && p.AdaptiveLayout().Eligible
	})
	if err := p.Host.Director.Movies.Play(nil, "stackup.mov", 0); err != nil {
		t.Fatal(err)
	}
	if !p.Host.Director.Movies.Playing() {
		t.Fatal("owned climbing movie unavailable")
	}
	for i := 0; i < 20 && p.Host.Director.Movies.Playing(); i++ {
		step()
		if p.Host.Director.Movies.Playing() && !p.AdaptiveLayout().Eligible {
			t.Fatal("climbing movie changed layout", p.AdaptiveLayout())
		}
	}

}

func TestAdaptiveWorldLabelsAndNavigationFades(t *testing.T) {
	p := adaptiveTestPlayer(t)
	p.Host.Session.TextOverlay = []TextOverlay{{Text: "A Deck", X: 24, Y: 30, Size: 12}}
	p.Host.Session.Fade.Level = 0.5
	p.Host.Session.Fade.PendingReveal = true
	if !p.AdaptiveLayout().Eligible || len(p.AdaptiveAtlas()) == 0 {
		t.Fatal("world label or navigation fade restored the bottom bar")
	}
}

func TestAdaptiveWorldSizedMoviesKeepLayout(t *testing.T) {
	for _, height := range []int{264, 384} {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			p := adaptiveTestPlayer(t)
			d := p.Host.Director
			seg := &df.MovieSegment{Frames: []df.MovieFrame{{}}, Width: 512, Height: height}
			frames := NewMovieFrames(seg)
			frames.imageIndex[0] = frames.images.PushBack(movieImageEntry{0, &MovieImage{Width: 512, Height: height, Pixels: make([]byte, 512*height)}})
			d.Movies.active = &activeMovie{name: "navigation.mov", seg: seg, frames: frames, paletteGeneration: d.Gamma.Generation}
			defer func() { d.Movies.active = nil }()
			if p.AdaptiveLayout().Eligible != (height == 264) {
				t.Fatal("incorrect layout for movie", height, p.AdaptiveLayout())
			}
			if height == 264 {
				p.Host.Session.SetVisible = false
				d.SetRoom(nil)
				for _, prop := range p.Host.Session.Props.Props.All() {
					prop.Visible = false
				}
				if !p.AdaptiveLayout().Eligible || len(p.AdaptiveAtlas()) == 0 {
					t.Fatal("navigation movie hiding the old toolbar changed layout")
				}
			}
		})
	}
}
