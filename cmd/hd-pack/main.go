// The HD artwork exporter runs locally, never in release CI.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/itskenny0/titanic-godot/internal/hdexport"
)

func main() {
	game := flag.String("game-data", "", "owned ISO, GOG/Steam or extracted-disc folder")
	manifest := flag.String("manifest", "godot/required_files.json", "game-file manifest")
	out := flag.String("output", ".build/hd-personal", "new work folder")
	mods := flag.String("mods", "", "optional extracted patch/mod folder; exports both original and patched art")
	motion := flag.Bool("motion", false, "also export all navigation animation frames (much larger)")
	characters := flag.Bool("characters", false, "also export complete dialogue character poses (larger pack)")
	resume := flag.Bool("resume", false, "extend an existing completed export with menus, mods or motion frames")
	flag.Parse()
	if *game == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := hdexport.Export(*game, *manifest, *out, *mods, *motion, *resume, *characters); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
