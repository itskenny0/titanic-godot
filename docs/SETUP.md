# Setup and mods

Use your own English PC game files. You can place both CD ISO images beside the player or select their folder in Game files / mods. Filenames are matched case-insensitively: one must contain `cd1`, the other `cd2`, with an `.iso` extension. Keep one matching image per disc in that folder. The player reads individual files inside the images without mounting or extracting them. Select the installed GOG/Steam game folder or its `LOCAL` folder directly. The player reads those files in place, without conversion or duplicate disc folders. It also accepts original discs extracted into separate `cd1` and `cd2` folders, retaining each disc's DATA and MOVIES directories.

You can select the folder in Game files / mods or pass `--game-data=/path/to/game`. Keep it available while playing. Android's document picker imports the files into app storage. The player does not run installers.

The player checks the required files before starting. Enable patches or external mods through the player to keep your originals intact.

## Mods

Download [M3tox's TAOOT repository](https://github.com/M3tox/TAOOT) separately. Select the extracted `fixes` folder, `mods` folder, or a folder containing your chosen combination from Game files / mods. Extract `extendMod.zip` first; its BOOTFILE and LOCAL directory must be under the selected root. ZIP files are not read directly.

The loader supports replacement `.SET`, `.SHP`, `.STG`, `.CST`, `.PUP`, `.MOV`, `.TRK`, `.SFX`, `.11K`, and BOOTFILE files. It checks each container, rejects conflicting duplicate basenames, and applies the selected files to both disc namespaces. Original files are untouched. Disabling mods restores the original index.

The LNGHALL lounge unlock needs LOUNGE1C.SET on both discs. The loader maps the original Disc 2 copy to Disc 1 when necessary. The extended pack adds ROGATION.TRK and replaces BOOTFILE, VLAD1.PUP, TURK.SET, INSIDE.TRK, and GANG.CST. Keep a mod enabled while continuing saves made with it. Mod selection restarts the game; save first.

## Controls and saves

Arrow keys move. The mouse interacts and chooses dialogue; hold the left button to drag. Space operates the original door shortcut. Escape skips animations. F10 opens the player menu, F11 toggles fullscreen, and F12 opens the on-screen keyboard. Ctrl/Cmd+S saves, Ctrl/Cmd+O loads. F1-F9 retain the Mac player's brightness/color shortcuts. Closing the window opens the engine menu; a second click leaves it open. Four or more close requests spanning at least 2.5 seconds force an exit, provided the clicks stay within 2.5 seconds of each other.

The original life-preserver menu remains available. Save while exploring, after conversations and animations finish. The player pauses when it loses focus. Cutscenes during play queue an autosave once exploration resumes and manual saving is allowed. The newest 25 checkpoints are labelled **Autosave** in blue in the load list and live in `Saves/Autosaves`, separate from manual saves. Autosave creation and cleanup never overwrite or delete manual saves. Older checkpoints are deleted three seconds after the new file is written, giving slow storage a quiet interval; extra files left by a shutdown are cleaned up on the next launch. **Game files / mods** lets you turn checkpoints off. Save manually between cutscenes and before quitting if you want to keep that progress.

Saves use Godot's `user://Saves` under the custom `Retanic` user directory. `Save Archives` sits beside it. The menu can import and export `.ti` files and open the save folder. Original Windows saves and Mac restoration saves are accepted. New saves retain the Mac restoration's complete-state trailer; compatibility with the 1996 Windows executable is not promised.

## Patches

The first launch lists M3tox's v1.0.3 FULL patches. Download the FULL ZIP from GitHub or choose a copy from your device, select any combination, then Continue. You can also select None and play without patches. They are intended for the Steam/GOG data, as the upstream README specifies. Nothing is applied until you choose. Reopen the chooser under Game files / mods to change your selection; save before applying changes because the player restarts.

- A-deck: stops repeated Tour-mode script errors.
- B-deck: restores Jay walking in the corridors.
- Starboard vestibule: fixes the Boat Deck door's wrong destination.
- Port vestibule: fixes the matching door on the port side.
- Grand Staircase: stops Trask clipping through the stairs.
- Other locations: sharper images during navigation across the remaining 51 SET files.

Each of the first five files also includes sharper navigation images for its location. Those changes cannot be split without editing the patch itself. The other-locations choice does not enable the separate lounge-unlock mod. Descriptions follow [M3tox's README](https://github.com/M3tox/TAOOT/blob/6019b87c9f5dacf83f9257770ea54a2822cb46c7/README.md).

The player verifies the archive SHA-256 and all 56 files before installing them in its private cache. Your original game files stay intact, and patches work offline afterward. An external mod folder takes precedence when filenames overlap. Personal builds include the patches and need no download.

Touch taps and controller clicks can reach small room hotspots and dialogue choices from slightly outside their edges. Exact hits take priority; puzzle screens and mouse input retain their original coordinates.

Switching apps, losing window focus or suspending Android pauses the game and audio. Returning resumes from the same point, unless you had already paused in a menu. Release the sticks before moving again after an interruption.
