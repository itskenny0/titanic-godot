# Titanic for Godot

A Godot player for *Titanic: Adventure Out of Time* (1996), based on [titanic-mac](https://github.com/axx-archive/titanic-mac) and [dreamREfactory](https://github.com/dhobi/dreamrefactory). Bring your own game files.

Build with the commands in [BUILDING.md](BUILDING.md), then choose your prepared `cd1` and `cd2` folders. [Setup and mods](docs/SETUP.md) covers digital installs and M3tox's fixes, lounge unlock, and extended-content mod. The bundled M3tox 1.0.3 FULL patches have a first-start chooser. Saves live outside the app; replaced saves are archived.

Use the arrow keys to move and the mouse to interact. On a handheld, the left stick moves, the right stick controls the pointer, and A clicks or drags. L1 slows the pointer. Start opens the menu. See [PortMaster setup](docs/PORTMASTER.md). On touchscreens, tap and drag directly, and use the virtual joystick to move. Portrait puts the game and controls at the bottom for easy reach; landscape puts controls beside the picture. An onscreen keyboard handles text entry.

CI builds Windows x64 and ARM64 portable packages, macOS Intel and Apple Silicon apps, Linux packages, a PortMaster ZIP, and an [Android ARM64 APK](docs/ANDROID.md).

Thank you to Daniel Hobi and the dreamREfactory contributors; axx-archive for the Mac port; M3tox for DFET and TAOOT mods; MRXstudios for reverse engineering; the Godot, QuickJS, QuickJS-NG, FRT, PortMaster, SDL, and gptokeyb teams; the Liberation Fonts authors; Evan Wallace for esbuild; and the original CyberFlix team. [Credits and licenses](CREDITS.md) lists the projects and authors behind this work.

Source: GPL-3.0. The full game is not included. Bundled patch files remain credited to M3tox and their respective rights holders.
