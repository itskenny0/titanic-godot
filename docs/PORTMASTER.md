# PortMaster

Extract `titanic-portmaster.zip` into your ports folder. Put prepared `cd1` and `cd2` folders in `ports/titanic/gamedata`. The ZIP contains the Godot pack and ARM libraries, including the M3tox 1.0.3 FULL patches and their first-start chooser. The full game is not included. PortMaster downloads the shared `frt_3.5.2.squashfs` runtime.

| Control | Action |
| --- | --- |
| Left stick / D-pad | Move and turn |
| Right stick | Move the mouse pointer |
| A / R3 | Click, hold to drag |
| B | Skip animation |
| X | Open door / Space |
| Y | On-screen keyboard |
| L1 | Slow pointer |
| R1 | Enter / confirm focused control |
| Start | Player menu |

FRT's PortMaster build disables native joypad input. `titanic.gptk` therefore maps both sticks through gptokeyb. Desktop Godot uses native controller input with the same layout.

Saves stay in the port's `saves/Retanic` directory. Keep that directory when upgrading. To use mods, extract them into a directory on the handheld and select it from Game files / mods. Prepare clean game data before adding mods.

ARM64 (aarch64) is the main handheld target. An optional armhf bridge is included for older devices. Both are built against an older system library baseline. The embedded JavaScript heap is limited to 384 MiB, in addition to Godot, decoded audio, and graphics memory. Start with a device with at least 1 GiB RAM. Handheld performance and sound still need device testing.

PortMaster's [porting](https://portmaster.games/porting.html), [packaging](https://portmaster.games/packaging.html), and [gptokeyb](https://portmaster.games/gptokeyb-documentation.html) guides define the launcher and mapping conventions. This package has not been submitted to PortMaster.
