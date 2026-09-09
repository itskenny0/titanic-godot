# Agent setup guide

This repository makes *Titanic: Adventure Out of Time* playable on macOS using the open-source dreamREfactory engine and the user's original game files. A request to "set this up" means carry the supported local setup through launch and verification. Read [docs/SETUP.md](docs/SETUP.md) first; use [BUILDING.md](BUILDING.md) when a source build is needed.

## Determine the available route

1. Inspect the Mac version, architecture, available disk space, an existing Titanic app, and the game-data paths the user supplied. Use targeted filename searches in those paths. Identify whether the input is extracted discs, disc images, a store installer, or an installed Windows game. Do not infer compatibility from a store name or a filename alone.
   If the user has no game files yet, explain the official purchase options and the current compatibility evidence before they buy. Prepare the Mac player while they obtain their licensed files, then continue setup when the local download is available.
2. Check existing data under `~/Library/Application Support/Titanic Adventure Out of Time`. Preserve `Saves`, `Save Archives`, and a valid `Game` directory. Importing game data must not replace saved games. Keep the user's source downloads and original disc files intact.
3. Prefer the public release for playing; it needs macOS 14 or later and contains both Mac architectures. Building is useful when the release is unavailable, a local change needs verification, or the user prefers source. Node/npm, Git, Python, and Xcode tools are build dependencies, not requirements for running the downloaded app.
4. Match the input to the support matrix in [docs/SETUP.md](docs/SETUP.md#supported-inputs). Run `python3 scripts/prepare-game-data.py --help`, then validate a candidate with `--source PATH --dry-run --json`. Prepare recognized data into a new `--output` folder. The tool accepts separate discs or a digital `LOCAL` tree matching its known checksum profile. Installer mode requires installed `innoextract`; its dry run is only a plan. A successful archive extraction alone does not establish game compatibility.
5. Continue with all available authorized local work. Ask one focused question only for an essential missing path, missing owned file, unsupported package decision, or required system interaction. Do not ask for confirmation at every inspection, copy, validation, or launch.

## Accounts and original game data

The user purchases the game, signs into its store, and downloads the licensed files themselves. Provide the official [GOG](https://www.gog.com/en/game/titanic_adventure_out_of_time) or [Steam](https://store.steampowered.com/app/785480/Titanic_Adventure_Out_Of_Time/) link when useful. Do not buy the game, access store credentials, download commercial game data from mirrors, or bypass DRM. Do not describe a current store package as compatible until that actual package has been recognized, validated, imported, and played.

The known GOG `1.0 tour fix` content profile has passed preparation and the full scripted story route using checksum-matched local files. The actual purchased offline-installer extraction step remains untested. GOG's product page offers offline installers; check that a user's particular package can be extracted and matches the profile. Steam package compatibility is unverified. Document the actual package version or layout you observed and any remaining step.

## Install, import, and verify

- Download the public runtime only from this repository's GitHub Releases, or build with `scripts/build-restored-app.sh --runtime-only`. Never use a private bundle containing someone else's game data or saves.
- If an app already exists, inspect its identity and preserve it before replacing it. Never reset Application Support to make setup look fresh.
- Keep Gatekeeper enabled. Do not clear quarantine attributes or disable macOS security checks. If macOS blocks a release, explain the app-specific action in [Apple's instructions](https://support.apple.com/en-us/102445), or use the source-build route when appropriate.
- Open Titanic and use **Choose Game Folder** or **Choose Discs Separately**. The native importer validates the required files and publishes a complete import atomically. Replacing game files is available through **Game → Import Game Files**. It preserves the last valid data until the replacement succeeds.
- Play the installed app: start or load a game, move and interact, check visible rendering and sound, toggle Control–Command–F out of and back into fullscreen, and check focus recovery. Make a clearly named setup-test save while exploring, quit, reopen, and load it. Never overwrite a user's existing save for verification.
- If tools cannot inspect the running game or hear audio, report precisely what remains unverified and request only that remaining observation. Do not equate a successful build, imported files, or a main-menu screenshot with completed gameplay verification.

Finish with the installed app path, the game-data source format, the saves location, what was actually verified, and any remaining limitation. Keep the response brief.

## Repository boundaries

The public repository and release contain the engine, native host, authored assets, and corresponding source. They must not contain commercial game data, installers, disc images, or personal `.ti` saves. Keep local extraction and test artifacts in ignored locations outside the tracked source. Diagnostics can contain local paths and filenames; inspect them before sharing.

The pinned engine is authoritative for parsing and execution. Preserve its revision unless the task explicitly includes an engine upgrade. Keep changes focused, preserve unrelated work, and run the smallest decisive checks. Local setup does not authorize publishing a release, pushing code, sending messages, or deleting unrelated files.
