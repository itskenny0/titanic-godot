# Building

Use Go 1.26.4, Python 3.11+, CMake, SCons 4.8.1, and a C/C++ compiler. Go handles gameplay, scripts, media decoding, and frame composition. Godot handles display, input, dialogs, and audio output. The port follows the pinned dreamREfactory revision recorded in `vendor/REVISIONS.json`. Godot 3.5.2 is the desktop and PortMaster baseline. Windows ARM64 and Android use the generated Godot 4.3 frontend.

```
python3 tools/prepare-notices.py
go test -race ./internal/... ./cmd/native-codecs
python3 tools/build-go.py --platform linux --arch x86_64
cmake -S native -B .build/native -DCMAKE_BUILD_TYPE=Release -DTITANIC_GO_LIBRARY="$PWD/.build/go/linux-x86_64/libtitanic_go.a"
cmake --build .build/native --target titanic -j2
cp .build/native/libtitanic.so godot/native/libtitanic.x86_64.so
```

Open `godot/project.godot` with Godot 3.5.2. On Linux without a display, use `xvfb-run -a` and `--audio-driver Dummy`. The editor uses the native bridge above; packaged desktop players compile it directly into Godot.

```
python3 tools/build-runtime.py --platform linux --arch x86_64
xvfb-run -a python3 tools/export-pack.py --godot /path/to/godot3 --output dist/titanic.pck
python3 tools/package.py linux --arch x86_64 --binary dist/runtime-linux-x86_64
```

Each desktop target gets its own Godot source directory. Do not build different platforms concurrently in the same Godot tree: generated headers are shared.

Linux needs the Godot X11, OpenGL, ALSA, PulseAudio, and udev development packages. Windows cross builds use LLVM MinGW 20240619. macOS builds require Xcode command line tools; the player requires macOS 12 or later. See `.github/workflows/build.yml` for complete commands for every package, including Android and PortMaster.

Publishing a GitHub release builds its tagged source and attaches all packages, SHA-256 checksums, and build details after every target succeeds. To fill in downloads for an existing release, run **Build Godot players** from Actions on `main` and enter its tag in `release_tag`. Leave that field blank for a build without publishing. Reruns replace the downloads for the same tag.

If only Linux packaging failed, **Finish release packaging** can reuse the compiled packages from that build run. Enter the release tag and Actions run ID. It checks the archived source against the tag before attaching the downloads.

The `--static` Linux tarball statically links the Go gameplay library and C++ runtime. It still uses system C, display, graphics, and audio libraries. It is not a fully static ELF executable. macOS apps are ad hoc signed and Windows packages are unsigned. Android uses the committed `packaging/android/debug.keystore` with alias `androiddebugkey` and password `android`, matching the test-device installs. Production signing needs your own private keys.

Run the portable Godot tests by copying `tests/runtime.gd` to `godot/runtime-test.gd` and launching Godot with `--path godot -s res://runtime-test.gd`. `tests/integration.gd` additionally needs owned game files and `--game-data=/path/to/gamedata --integration-test`. Do not include game data or saves in Git. Public packages contain the patch chooser and checksums. The player downloads or imports the M3tox FULL ZIP on first start. Use `--bundle-patches` with either exporter if you have prepared the files using `python3 tools/fetch-patches.py`.

CI uses synthetic fixtures, including tiny generated ISOs; it needs no commercial game files. Set `TAOOT_ISO_DIR` locally to check your own disc images. The gameplay fixtures follow the pinned reference engine. With owned game files, run `TAOOT_GAME_DATA=/path/to/gamedata go test ./internal/...` for startup, navigation, and save/load integration. Set `TAOOT_MOD_DATA` to the extracted Extended Mod directory to check its startup too. The Godot integration test uses actual game data and the UI bridge; `Dummy` audio checks do not verify sound on a device.

## Personal packages with game files

Run these commands from the repository with Go 1.26.4. Start with an unbundled native Go APK or PortMaster ZIP from a release or your own build. The local Go tool copies only the files in `godot/required_files.json`; it leaves saves, installers, and extra mods out. The builder also downloads and bundles the checksum-pinned patches. Pass `--patch-archive /path/to/TAOOTpatch1.03.FULL.zip` to use a local copy. The chooser stays available offline. Existing outputs are never overwritten.

Pass a folder containing both CD ISOs, the installed GOG/Steam game folder, its `LOCAL` folder, or a parent containing extracted `cd1` and `cd2` folders as `--game-data`. ISO filenames must contain `cd1` and `cd2` (case-insensitive). The builder copies only required files out of the ISOs into the package; it leaves the images untouched. Digital packages keep one copy of each required file in `LOCAL`; original disc packages retain separate disc files. For digital input, use a base player built with `LOCAL` support.

```
go run ./cmd/personal-build --target portmaster --base dist/titanic-portmaster.zip --game-data gamedata --output titanic-portmaster-personal.zip

go run ./cmd/personal-build --target android --base dist/titanic-android-arm64-release.apk --game-data gamedata --output titanic-android-arm64-personal.apk --android-build-tools /path/to/android-sdk/build-tools/35.0.0
```

Android needs Java 17 and Android SDK Build Tools 35 or newer. The tool aligns and signs the APK with the shared debug key, preserving `cat.kenny.taoot`. Release code and debug-key signing are separate choices. An existing `titanic-android-arm64-debug.apk` also works as a base. Add `--strip-tool /path/to/llvm-strip` to either command to remove native debug symbols while keeping the exports needed by Godot and Go. The NDK includes this tool.

Install the APK normally, or extract the ZIP into the handheld's ports folder. Android reads its bundled assets directly without an import or a second extracted copy. PortMaster reads the included game folder with its usual FRT 3.5.2 runtime. Saves still live outside the game assets. Keep these packages private; the filenames above are ignored by Git, and this mode is not part of any GitHub workflow.

## Smaller Android release builds

Prepare notices as above, put SCons on `PATH`, and set `ANDROID_HOME` and `JAVA_HOME`. Install Android platform 34, Build Tools 34.0.0, and NDK 26.1.10909125 for the template; the personal packager uses Build Tools 35 or newer. Then run:

```
python3 tools/fetch-godot.py 4 .build/godot4
python3 tools/build-go.py --platform android --arch arm64
python3 tools/install-module.py .build/godot4 --go-library .build/go/android-arm64/libtitanic_go.so
python3 tools/prepare-android.py
python3 tools/build-android.py
python3 tools/prepare-godot4.py --godot /path/to/godot4
XDG_CONFIG_HOME="$PWD/.build/android-editor-settings" python3 tools/export-android.py --release --godot /path/to/godot4 --sdk "$ANDROID_HOME"
```

The release template keeps GDScript, fonts, text shaping, and the Titanic bridge. It disables other optional modules, 3D, and Vulkan, uses size optimization and ThinLTO, and runs R8 on Java/Kotlin with keep rules for JNI and the document picker. Go builds use their normal compiler optimizations plus `-s -w` to remove symbol and debug tables. Game and patch assets make up most of the package size.
