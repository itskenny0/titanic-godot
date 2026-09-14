// Personal packages are built locally and are never release inputs.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/itskenny0/titanic-godot/internal/personalbuild"
)

func main() {
	var options personalbuild.Options
	flag.StringVar(&options.Target, "target", "", "android or portmaster")
	flag.StringVar(&options.Base, "base", "", "unbundled native Go APK or PortMaster ZIP")
	flag.StringVar(&options.GameData, "game-data", "", "GOG/Steam game folder, LOCAL folder, or parent containing cd1 and cd2")
	flag.StringVar(&options.Output, "output", "", "new personal APK or ZIP (never overwritten)")
	flag.StringVar(&options.Manifest, "manifest", "godot/required_files.json", "required game-file manifest")
	flag.StringVar(&options.BuildTools, "android-build-tools", "", "Android SDK Build Tools 35+ directory (or use PATH)")
	flag.StringVar(&options.Keystore, "keystore", "packaging/android/debug.keystore", "shared Android debug keystore")
	flag.StringVar(&options.StripTool, "strip-tool", "", "optional llvm-strip path to remove native debug symbols")
	flag.StringVar(&options.PatchArchive, "patch-archive", "", "M3tox 1.0.3 FULL ZIP (downloads the pinned archive when omitted)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if err := personalbuild.Build(options); err != nil {
		fmt.Fprintln(os.Stderr, "Personal build:", err)
		os.Exit(1)
	}
}
