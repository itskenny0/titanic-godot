# Release validation

Version 0.1.0 was checked on 9 September 2026 on an Apple Silicon Mac running macOS 26.5. The app targets macOS 14 and includes `arm64` and `x86_64` binaries. Intel hardware and older supported macOS versions were not tested directly.

## Public player

- Started the game-free public app with no imported game directory. Its native setup screen appeared.
- Selected an English PC CD parent folder and imported both discs: 536 required files, approximately 1.2 GB. The game reached the original opening sequence and London apartment.
- Moved around, created a distinct new save, quit, and launched the final release build from another folder. Loaded that save and continued moving in the restored room.
- Loaded a story checkpoint on Disc 2, entered the gymnasium, and spoke to Penny Pringle. The original portrait and dialogue choices rendered correctly.
- Earlier gameplay checks covered the Grand Staircase, deck, smoking room, Trask's conversation, engine and smokestack ladders, fullscreen transitions, and focus recovery.
- Original Windows saves loaded. Save replacement archives, damaged-save handling, and the new complete-state save format were checked separately.

## Source and package

- Clean build without original game files or saved games passed.
- An isolated rebuild from the app's corresponding-source archives passed without a Git checkout or commercial assets.
- Seven portable save/recovery tests and the TypeScript check passed.
- The pinned engine's 30-test story suite passed across 27 segments with English PC CD data. This is automated route coverage, not audiovisual testing of every branch.
- The public bundle passed signature, architecture, system-dependency, source-archive, and game-data exclusion checks. Its ZIP passed integrity and content checks.

## GOG data preparation

The known English GOG profile is build `50837422884815054`, version `1.0 tour fix`. All 440 unique required `LOCAL` files were reconstructed from owned original discs and checked against the sizes and raw chunk checksums in GOG's published depot metadata. No commercial files were downloaded for this test.

The actual preparation helper converted those verified inputs into 536 runtime paths. Against that output, all 27 automated story segments reached the good ending with no engine errors. All 26 save checkpoints retained the complete globals, and five mission-boundary saves were loaded by the real game host. The native Mac app also imported that prepared folder, loaded London and gymnasium saves, navigated, switched fullscreen, and wrote a fresh save.

This verifies the advertised runtime contents and their conversion. It does **not** verify extraction of a currently purchased offline installer, every optional GOG tour patch movie, or current Steam downloads. The helper rejects digital files that do not match the known content profile. See [file-format details](FILE-FORMATS.md).

## Remaining limits

This is an unofficial prerelease. The app is ad hoc signed and has not been notarized by Apple. Occasional reported audio clipping has not been conclusively isolated. Current store-package preparation status is documented in [SETUP.md](SETUP.md); extraction alone is not proof that a game works.

The restored player imports original Windows saves. New extended saves are intended for this player; compatibility with the original Windows executable is not promised.
