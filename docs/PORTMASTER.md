# PortMaster

Extract `titanic-portmaster.zip` into your ports folder. Put both CD ISOs inside `ports/titanic`, in its `gamedata` folder, or beside `Titanic.sh`. Their filenames must contain `cd1` and `cd2` (case-insensitive), with an `.iso` extension. FRT reads them directly without mounting or extraction. GOG/Steam game files with their `LOCAL` folder and extracted `cd1` and `cd2` folders also work in `gamedata`. The ZIP contains the Godot pack, ARM libraries, and a first-start patch chooser. Download or import the M3tox 1.0.3 FULL ZIP there. Personal packages include the patches. The full game is not included. The ZIP includes Godot/FRT 3.5.2 for ARM64 and armhf. The console does not need to download a runtime.

## Installing patches without internet

1. On a computer, download M3tox's [TAOOT 1.0.3 FULL patch ZIP](https://github.com/M3tox/TAOOT/releases/download/v1.0.3/TAOOTpatch1.03.FULL.zip).
2. Copy `TAOOTpatch1.03.FULL.zip` to `ports/titanic` on the console's SD card. Keep it zipped.
3. Start Titanic. In the first-start patch selector, choose **Choose patch ZIP** and select that file. For an existing setup, press Start, open **Game files / mods**, then **M3tox patches**.
4. Wait for the import to finish, then choose the fixes you want. **All** includes the sharper navigation patches. Choose **Continue** on first start, or **Apply and restart** in an existing setup. Save your game before changing patches during play.

The player checks the archive and installs the patches locally. After a successful import, you can delete the copied ZIP to recover space. Your choices and installed patches remain on the console. A personal package already includes these patches, so it only needs the selection step. PortMaster itself must already be installed.


For Steam/GOG, the expected path is `ports/titanic/gamedata/LOCAL/BOOTFILE`. The launcher prefers lowercase `titanic` and `gamedata`, but accepts other capitalization when those names are absent.

If the port returns to the menu, share `ports/titanic/log.txt`. Verbose logging is always enabled. It records firmware, architecture, memory, free space, folder permissions, library dependencies, controller setup, the launch command, full Godot output, and exit status. Game and save contents are not logged. The previous launch is kept in `log.previous.txt`. If the game folder is missing, look for `Titanic-log.txt` beside `Titanic.sh` instead. Replace just `Titanic.sh` to try an updated launcher; keep your game files and saves.

| Control | Action |
| --- | --- |
| Left stick / D-pad | Move and turn, or select dialogue replies and menus |
| Right stick | Move the mouse pointer |
| A / R1 | Confirm selected reply or menu button; otherwise click, hold to drag |
| R3 | Click at the pointer, hold to drag |
| B | Back in menus; skip speech or animation in the game |
| X | Open door / Space |
| Y | On-screen keyboard |
| L1 | Slow pointer |
| L2 / R2 | Previous / next clickable target |
| Start | Player menu |

FRT's PortMaster build disables native joypad input. `titanic.gptk` therefore maps both sticks through gptokeyb. Desktop Godot uses native controller input with the same layout.

Selected targets get a thin, half-transparent brass circle. Use directions and A to answer without aiming the pointer. In inventory and puzzle screens, directions select the nearest item in that direction. Shoulder buttons cycle across the visible rows. The same controls work in the game menus, save lists and on-screen keyboard. In rooms, L2 and R2 select visible targets; the right stick remains available for puzzles and dragging. The pointer hides during button navigation and returns when you move the right stick or a mouse. Open Controls / display > Controller bindings from the voyage menu to remap actions or view the controls. Changes are saved automatically; Reset defaults restores the original layout. PortMaster remaps the keys emitted by gptokeyb, so A/R1 and B/Select change together. D-pad changes also affect the left stick. R3 clicking and L1 pointer speed remain in `titanic.gptk`. The UI fits a 640x480 handheld screen, including devices such as the R36S. These devices always use the original layout and skip the layout chooser. Wide devices can opt into side panels; L2/R2 can select the Classic / Side panels switch during exploration.

Saves stay in the port's `saves/Retanic` directory. Keep that directory when upgrading. To use mods, extract them into a directory on the handheld and select it from Game files / mods. Keep clean game data and select mods separately.

The launcher uses the screen resolution reported by PortMaster, with a 640x480 fallback. The original game picture is 512x384 and scales to fit. Game images are not recompressed for handhelds. Personal packages include the M3tox patches, but you still choose which ones to enable in Game files / mods. Select All to apply sharper navigation in every patched area. Patch choices are saved separately on each device.

ARM64 (aarch64) is the main handheld target. An optional armhf bridge is included for older devices. Both are built against an older system library baseline. Handheld performance and sound still need device testing.

PortMaster's [porting](https://portmaster.games/porting.html), [packaging](https://portmaster.games/packaging.html), and [gptokeyb](https://portmaster.games/gptokeyb-documentation.html) guides define the launcher and mapping conventions. This package has not been submitted to PortMaster.
