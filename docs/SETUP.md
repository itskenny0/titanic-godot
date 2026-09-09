# Set up Titanic for Mac

The public app supplies the Mac player. You supply your own original game files. Once both discs are imported, the game runs offline and keeps saved games outside the app.

You need macOS 14 or later and enough space for the app, your original downloads, and approximately 1.2 GB of imported game data. Replacing an existing import temporarily needs space for both copies. Apple Silicon and Intel binaries are included; hands-on verification has been performed on Apple Silicon.

## Get the original game

The exact game is *Titanic: Adventure Out of Time*, originally released in 1996. Official purchase pages are [GOG](https://www.gog.com/en/game/titanic_adventure_out_of_time) and [Steam](https://store.steampowered.com/app/785480/Titanic_Adventure_Out_Of_Time/).

Check the support table below before buying specifically for this player. The English PC discs and the known GOG digital data profile are tested inputs. The actual GOG offline installer still needs extraction verification, and current Steam packages have not been inspected. The store listings describe the Windows game; this project provides its own unofficial Mac player.

Purchase and sign in yourself. Download the game files from your own account, then give Codex the local path. [GOG's product page](https://www.gog.com/en/game/titanic_adventure_out_of_time) confirms that offline installers are available without Galaxy. Keep every installer part together; a small launcher or download stub is not the complete game. If you already own the discs or have a supported extracted copy, you do not need to buy the game again.

## Supported inputs

| Input | Current status | Next step |
| --- | --- | --- |
| Complete extracted English PC Disc 1 and Disc 2 | Verified: native import, gameplay, fullscreen, and save/load | Validate and import the folders. |
| Original disc images | The app does not import image files directly | Extract or mount your own images, retain both volumes, then validate the resulting files. |
| GOG English `1.0 tour fix` digital `LOCAL` data matching the known profile | Preparation and full scripted story route verified using files matching the official metadata | Use the helper to verify the checksums and prepare importable disc folders. |
| GOG offline Windows installer (`.exe` plus any `.bin` parts) | Extraction support is provided, but the actual purchased installer has not been tested | Use the optional `innoextract` route; proceed only if extraction and content validation succeed. |
| Steam Windows installation or downloaded depot | Current commercial package/layout unverified | The helper accepts it only if it contains separate complete discs or a `LOCAL` tree matching the known checksum profile. |
| Other languages, demos, incomplete downloads, or an unrecognized merged layout | Unsupported | Supply the tested English two-disc data, or report the exact layout for further compatibility work. |

Compatibility belongs to an actual package and its contents. Successfully opening an installer archive does not establish that it contains both complete discs. Report a new store version as tested only after extraction, full validation, native import, and real gameplay/save-load verification.

For the GOG profile, the test files reproduced all 440 required unique files advertised by build `50837422884815054`, version `1.0 tour fix`, with matching sizes and raw chunk checksums. They were assembled from existing owned media; no purchased installer was used. The helper produced all 536 required disc paths, and the real engine completed all 27 story segments to the good ending with no reported errors, 26 save checkpoints, and five saved-game loads. See the [profile and provenance](FILE-FORMATS.md#known-digital-local-profile) and [official GOG build metadata](https://content-system.gog.com/products/1792718486/os/windows/builds?generation=2).

## Let Codex handle local setup

Open this repository in Codex, or give it the repository URL, and paste:

```text
Set up Titanic for Mac from https://github.com/axx-archive/titanic-mac.
Read AGENTS.md and docs/SETUP.md. My game files, if any: [path or none].
If I don't have them yet, explain the official purchase options and current
compatibility before I buy. Once I download my copy, handle supported data
preparation, install the player, and get the game running. Preserve my files
and saves. Check fullscreen, controls, sound, and saving/loading after relaunch.
Ask only for an essential missing input or system action.
```

Codex should inspect the supplied files and complete the available local work. You handle purchases, account sign-in, and downloading the original game. The [agent instructions](../AGENTS.md) describe the boundaries and completion checks.

## Prepare and validate game files

Keep the original download or disc image intact. Work in a separate extraction folder. Do not merge Disc 1 and Disc 2: some identically named rooms contain different story states on each disc.

The supported folder shape is:

```text
Titanic game files/
├── cd1/
│   ├── DATA/          # includes BOOTFILE and BEDSIT1.SET
│   ├── MOVIES/
│   ├── PUPPETS2/
│   └── ...           # keep the other original runtime directories
└── cd2/
    ├── DATA/          # includes A14.SET and CARGO.SET
    ├── MOVIES/
    ├── PUPPETS1/
    └── ...
```

The native importer requires all 536 indexed runtime paths across both discs, checks that files are nonempty, and validates key container headers. Folder names and a few sample files are insufficient. Original installers, readme files, and obsolete Windows executables are not needed to run this Mac player.

From a clone of this repository, inspect the preparation tool and validate the local source:

```sh
python3 scripts/prepare-game-data.py --help
python3 scripts/prepare-game-data.py --source "/path/to/your/game files" --dry-run --json
```

For recognized data, prepare a separate folder for the app to import:

```sh
python3 scripts/prepare-game-data.py \
  --source "/path/to/your/game files" \
  --output "$HOME/Titanic Game Data" --json
```

If discovery reports ambiguous disc folders, specify the two original roots:

```sh
python3 scripts/prepare-game-data.py \
  --disc1 "/path/to/Disc 1" --disc2 "/path/to/Disc 2" \
  --output "$HOME/Titanic Game Data" --json
```

Choose an output folder that does not already exist, inside an existing parent folder. The helper preserves the source and refuses to replace an existing output. A successful result reports `validated` for a folder dry run or `prepared` after copying. Errors return exit code 2 and identify the missing or incompatible input.

The helper prepares files on disk; the app still performs its own validation and import. Read the helper's result before continuing. Select the generated **Titanic Game Data** folder in the app.

### GOG offline installers and digital installations

For an already extracted digital installation, pass its parent folder or its `LOCAL` directory to `--source`. The helper recognizes one specific English GOG profile and verifies all 440 required unique files against the published sizes and chunk checksums before mapping them into the app's 536 disc paths. An unknown or modified digital edition is rejected; folder naming alone cannot make it compatible.

If you have a GOG offline installer instead, keep the `.exe` and all accompanying `.bin` parts in one folder. The optional extraction route requires [innoextract](https://constexpr.org/innoextract/) already installed:

```sh
python3 scripts/prepare-game-data.py \
  --installer "/path/to/setup_titanic_adventure_out_of_time.exe" \
  --output "$HOME/Titanic Game Data" --json
```

This extracts the user-supplied package without running its Windows executable, then applies the same content checks. The actual purchased offline installer has not yet been exercised in this project's verification; a missing part or unsupported installer version may still block extraction. In installer mode, `--dry-run` reports an extraction plan only and does not validate the game.

The helper does not purchase games, authenticate to stores, or download game files. It can inspect a local Steam installation, but a current Steam package has not been verified. It accepts a digital `LOCAL` tree only when its bytes match the known profile.

For a package that the preparation tooling cannot recognize, stop extraction and keep the originals. Record the installer name/version, local directory layout, and missing required paths. Do not rename unrelated files to satisfy validation or copy one disc over the other.

## Install and import

1. Download **Titanic-Mac-v0.1.0.zip** from this project's [GitHub Releases](https://github.com/axx-archive/titanic-mac/releases), unzip it, and move **Titanic.app** to Applications. Preserve an existing app before replacing it.
2. Open Titanic. On first launch, choose **Choose Game Folder** and select the parent containing both extracted discs. Use **Choose Discs Separately** if the folders are in different locations.
3. Wait for validation and copying. The app starts the game automatically when the import is complete. Your source files remain in place.
4. Later launches use the imported local files. **Game → Import Game Files** can replace them; a cancelled or failed import preserves the previous valid data.

The release is locally signed and is not notarized by Apple. If macOS blocks it, follow [Apple's app-specific opening instructions](https://support.apple.com/en-us/102445). Keep Gatekeeper enabled. A [source build](../BUILDING.md) is also available:

```sh
git clone https://github.com/axx-archive/titanic-mac.git
cd titanic-mac
scripts/build-restored-app.sh --runtime-only
python3 scripts/verify-restored-app.py --runtime-only dist/public/Titanic.app
open dist/public/Titanic.app
```

Building requires Xcode Command Line Tools, Git, Python 3, and Node.js 22 or 24 with npm. The downloaded app does not require those tools, Wine, or a local server.

## Verify the result

Start or load a game, move around a room, interact with something, and check that music/dialogue and the pointer behave correctly. Toggle fullscreen with **Control–Command–F**, return to windowed mode, and switch away and back to check recovery.

While exploring, use **Command–S** to create a distinct setup-test save. Quit with **Command–Q**, reopen, and load it with **Command–O**. Confirm the room and progress are restored. Preserve existing saves; do not overwrite one to test setup. If Codex cannot hear audio or operate the screen, it should name that remaining check rather than report it as passed.

## Saves and troubleshooting

| Content | Location |
| --- | --- |
| Imported game files | `~/Library/Application Support/Titanic Adventure Out of Time/Game` |
| Saved games | `~/Library/Application Support/Titanic Adventure Out of Time/Saves` |
| Previous save versions | `~/Library/Application Support/Titanic Adventure Out of Time/Save Archives` |
| Local diagnostics | `~/Library/Application Support/Titanic Adventure Out of Time/Diagnostics` |

Moving or updating the app does not reset these folders. The game uses manual saves. Original Windows `.ti` files can be imported through the Game menu or opened with Titanic. New saves preserve this player's complete state and are intended for this player; original Windows-executable compatibility is not promised.

If import reports a missing file, check that the complete original disc was extracted and that the two volumes remain separate. If an installer is unsupported, retain it and report its precise version or layout. If the app starts but a gameplay check fails, record the room, action, macOS version, and observed result. Review diagnostics before sharing; they may contain local filenames or paths. Never publish original game files, installers, or personal saved games in an issue.
