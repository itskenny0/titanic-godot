# PortMaster

Extract `titanic-portmaster.zip` into your ports folder. Put both CD ISOs inside `ports/titanic`, in its `gamedata` folder, or beside `Titanic.sh`. Their filenames must contain `cd1` and `cd2` (case-insensitive), with an `.iso` extension. FRT reads them directly without mounting or extraction. GOG/Steam game files with their `LOCAL` folder and extracted `cd1` and `cd2` folders also work in `gamedata`. The ZIP contains the Godot pack, ARM libraries, and a first-start patch chooser. Download or import the M3tox 1.0.3 FULL ZIP there. Personal packages include the patches. The full game is not included. PortMaster downloads the shared `frt_3.5.2.squashfs` runtime.

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

Dialogue replies get a visible selection border. Use directions and A to answer without aiming the pointer. The same controls work in the game menus, save lists and on-screen keyboard. In rooms, L2 and R2 select visible targets; the right stick remains available for puzzles and dragging. The pointer hides during button navigation and returns when you move the right stick or a mouse. Open Controller settings from the voyage menu to remap actions or view the controls. Changes are saved automatically; Reset defaults restores the original layout. PortMaster remaps the keys emitted by gptokeyb, so A/R1 and B/Select change together. D-pad changes also affect the left stick. R3 clicking and L1 pointer speed remain in `titanic.gptk`. The UI fits a 640x480 handheld screen, including devices such as the R36S.

Saves stay in the port's `saves/Retanic` directory. Keep that directory when upgrading. To use mods, extract them into a directory on the handheld and select it from Game files / mods. Keep clean game data and select mods separately.

The launcher uses the screen resolution reported by PortMaster, with a 640x480 fallback. The original game picture is 512x384 and scales to fit. Game images are not recompressed for handhelds. Personal packages include the M3tox patches, but you still choose which ones to enable in Game files / mods. Select All to apply sharper navigation in every patched area. Patch choices are saved separately on each device.

ARM64 (aarch64) is the main handheld target. An optional armhf bridge is included for older devices. Both are built against an older system library baseline. Handheld performance and sound still need device testing.

PortMaster's [porting](https://portmaster.games/porting.html), [packaging](https://portmaster.games/packaging.html), and [gptokeyb](https://portmaster.games/gptokeyb-documentation.html) guides define the launcher and mapping conventions. This package has not been submitted to PortMaster.
