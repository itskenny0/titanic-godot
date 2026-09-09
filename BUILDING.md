# Building the Mac app

The public app contains the native host and the open-source engine. It contains no original game discs, saved games, manuals, box artwork, or extracted game cursors. On first launch, supply your own extracted English PC game discs. The app runs offline after import.

## Requirements

- macOS 14 or newer, on Apple silicon or Intel.
- Xcode Command Line Tools (`xcode-select --install`), including Swift and the macOS SDK.
- Node.js 22 or 24, npm, Python 3, and Git.
- About 1 GB for the source and development dependencies; imported game data requires additional disk space.

## Public build

From the repository root:

```sh
python3 scripts/build-restored-app.py --runtime-only
python3 scripts/verify-restored-app.py --runtime-only dist/public/Titanic.app
```

The builder obtains dreamREfactory at the pinned revision `b43a02668f3db36519bd5b44a5892fdefd292208`, installs its locked npm dependencies, builds the Web runtime, and compiles a universal native executable for `arm64` and `x86_64`. It rejects a different engine revision or tracked engine modifications.

The result is `dist/public/Titanic.app`. This command does not install or replace an app in Applications. The signature is ad hoc for local integrity; it is not a Developer ID signature or Apple notarization.

Open the app and select a folder containing both extracted discs (`cd1` and `cd2`, or `titanic1` and `titanic2`), or select each disc when prompted. Disc image and installer extraction is outside this app. The import copies needed runtime files into Application Support; it leaves the originals intact.

## Tests without game files

After the builder has prepared the pinned engine dependencies:

```sh
vendor/dreamrefactory/node_modules/.bin/vitest run --config native/restoration/Web/vitest.config.ts
vendor/dreamrefactory/node_modules/.bin/tsc -p native/restoration/Web/tsconfig.json
```

These tests generate their own neutral save envelope from the documented format schema. They verify complete metadata, corrupt-file rejection, Finder/startup error recovery, and the original engine's save/load dialog recovery. They require neither commercial game data nor a personal saved game.

New saves use an authored format envelope plus a validated, versioned state extension. The app can read original Windows `.ti` saves. Saves created by this Mac app are intended for this app; compatibility with the original Windows executable is not promised.

## Rebuilding from the bundled corresponding source

Each app includes these archives under `Contents/Resources/Licenses`:

- `native-restoration-source.tar.gz`: native/Web integration, authored icon, build scripts, tests, and this document.
- `dreamREfactory-source.tar.gz`: required engine code, original package manifests and lockfile, revision marker, and per-file hashes. Commercial artwork and game-data directories are excluded.

Extract the restoration archive into a new working folder. Extract the engine archive into `vendor/dreamrefactory` inside that folder. Run the public build command above. The builder validates the included source hashes when no Git checkout is present. npm may download the locked development dependencies; the resulting app does not need npm, Node, Wine, or an internet connection to play.

The original icon can be regenerated with:

```sh
swift native/restoration/Resources/generate-icon.swift
```

## Optional private bundle

For a personal app that embeds both discs, place your extracted discs in `discs/cd1` and `discs/cd2`, then omit `--runtime-only`. Local `.ti` files under `saves` can be included as import seeds. Private builds go to `dist/Titanic.app`. The optional `--install` flag installs a private build and archives an existing app before replacement. Never publish that private bundle or its disc/save contents.

The GPL-3.0 license covers this integration and the engine. It does not grant rights to distribute the commercial game's data.
