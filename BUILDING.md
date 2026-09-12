# Building

Use Python 3.11+, Node.js 22, CMake, SCons 4.8.1, and a C/C++ compiler. The build scripts pin Godot and vendor the gameplay engine, QuickJS-NG, and Godot headers. Godot 3.5.2 is the desktop and PortMaster baseline. Windows ARM64 and Android use the generated Godot 4.3 frontend.

```
npm ci
npm run build:engine
python3 tools/fetch-patches.py
npm test
cmake -S native -B .build/native -DCMAKE_BUILD_TYPE=Release
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

Linux needs the Godot X11, OpenGL, ALSA, PulseAudio, and udev development packages. Windows cross builds use LLVM MinGW 20240619. macOS builds require Xcode command line tools. See `.github/workflows/build.yml` for complete commands for every package, including Android and PortMaster.

The `--static` Linux tarball statically links the gameplay engine, QuickJS, and C++ runtime. It still uses system C, display, graphics, and audio libraries. It is not a fully static ELF executable. macOS apps are ad hoc signed and Windows packages are unsigned. Android uses the committed `packaging/android/debug.keystore` with alias `androiddebugkey` and password `android`, matching the test-device installs. Production signing needs your own private keys.

Run the portable Godot tests by copying `tests/runtime.gd` to `godot/runtime-test.gd` and launching Godot with `--path godot -s res://runtime-test.gd`. `tests/integration.gd` additionally needs owned game files and `--game-data=/path/to/gamedata --integration-test`. Do not include game data or saves in Git. Release builds fetch the requested, checksum-pinned M3tox patch pack; those downloaded files stay outside Git.
