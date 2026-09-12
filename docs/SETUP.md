# Setup and mods

Use your own English PC game files. The player reads two extracted discs, each with its original DATA and MOVIES folders. It also accepts the known GOG `1.0 tour fix` LOCAL layout after preparation:

```sh
python3 tools/prepare-game-data.py --source /path/to/game --output /path/to/prepared
```

The helper checks all 440 digital files against the Mac port's checksum profile, then creates separate `cd1` and `cd2` folders. Original disc data uses a 536-path inventory and container checks. It never overwrites an existing output directory. Select that prepared folder in the player, or use `--game-data=/path/to/prepared`. Game files remain where you selected them; keep that folder available.

Opening an unrecognized or already modified LOCAL install is rejected. Prepare clean files first, then enable mods. This prevents an unknown installation from silently receiving the wrong disc mapping.

## Mods

Download [M3tox's TAOOT repository](https://github.com/M3tox/TAOOT) separately. Select the extracted `fixes` folder, `mods` folder, or a folder containing your chosen combination from Game files / mods. Extract `extendMod.zip` first; its BOOTFILE and LOCAL directory must be under the selected root. ZIP files are not read directly.

The loader supports replacement `.SET`, `.SHP`, `.STG`, `.CST`, `.PUP`, `.MOV`, `.TRK`, `.SFX`, `.11K`, and BOOTFILE files. It checks each container, rejects conflicting duplicate basenames, and applies the selected files to both disc namespaces. Original files are untouched. Disabling mods restores the original index.

The LNGHALL lounge unlock needs LOUNGE1C.SET on both discs. The loader maps the original Disc 2 copy to Disc 1 when necessary. The extended pack adds ROGATION.TRK and replaces BOOTFILE, VLAD1.PUP, TURK.SET, INSIDE.TRK, and GANG.CST. Keep a mod enabled while continuing saves made with it. Mod selection restarts the game; save first.

## Controls and saves

Arrow keys move. The mouse interacts and chooses dialogue; hold the left button to drag. Space operates the original door shortcut. Escape skips animations. F10 opens the player menu, F11 toggles fullscreen, and F12 opens the on-screen keyboard. Ctrl/Cmd+S saves, Ctrl/Cmd+O loads. F1-F9 retain the Mac player's brightness/color shortcuts.

The original life-preserver menu remains available. Save while exploring, after conversations and animations finish. The player pauses when it loses focus. Save before quitting; there is no autosave.

Saves use Godot's `user://Saves` under the custom `Retanic` user directory. `Save Archives` sits beside it. The menu can import and export `.ti` files and open the save folder. Original Windows saves and Mac restoration saves are accepted. New saves retain the Mac restoration's complete-state trailer; compatibility with the 1996 Windows executable is not promised.

## Bundled patches

The first launch lists M3tox's v1.0.3 FULL patches. Choose any combination, then Continue. They are intended for the Steam/GOG data, as the upstream README specifies. Nothing is applied until you choose. Reopen the chooser under Game files / mods to change your selection; save before applying changes because the player restarts.

- A-deck: stops repeated Tour-mode script errors.
- B-deck: restores Jay walking in the corridors.
- Starboard vestibule: fixes the Boat Deck door's wrong destination.
- Port vestibule: fixes the matching door on the port side.
- Grand Staircase: stops Trask clipping through the stairs.
- Other locations: sharper images during navigation across the remaining 51 SET files.

Each of the first five files also includes sharper navigation images for its location. Those changes cannot be split without editing the patch itself. The other-locations choice does not enable the separate lounge-unlock mod. Descriptions follow [M3tox's README](https://github.com/M3tox/TAOOT/blob/6019b87c9f5dacf83f9257770ea54a2822cb46c7/README.md).

Selected patches are read from the application package, leaving your game files intact. An explicitly selected external mod folder takes precedence over bundled patches when filenames overlap. Deselect bundled patches to play the unpatched data. Builds verify the archive SHA-256 and all 56 files before packaging; the game works offline afterward.
