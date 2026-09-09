# Game data formats and preparation

The app imports two extracted English disc trees. The preparation helper also
recognizes the known digital `LOCAL` layout and converts it into that structure
after verifying every required file against a checked-in GOG metadata profile.
It needs files you supply locally; it does not buy, download, log in, or run a
Windows installer.

## Commands

Run these from a checkout of this repository with Python 3. First inspect the
files without changing them:

```sh
python3 scripts/prepare-game-data.py \
  --source "/path/to/extracted game" --dry-run --json
```

Then choose a **new** output directory whose parent already exists:

```sh
python3 scripts/prepare-game-data.py \
  --source "/path/to/extracted game" \
  --output "/path/to/Titanic Game Data" --json
```

Open Titanic.app and select `Titanic Game Data`, which will contain `cd1` and
`cd2`. The app validates and copies these into its own Application Support
folder. The preparation output can remain outside the application bundle.

If automatic discovery finds multiple disc copies, choose exact roots:

```sh
python3 scripts/prepare-game-data.py \
  --disc1 "/path/to/first disc" --disc2 "/path/to/second disc" \
  --output "/path/to/Titanic Game Data" --json
```

`--source` can also be repeated for two roots. Discovery searches up to six
directory levels below the supplied folder, ignoring installation, support,
screenshots, and demo folders. Select a narrower folder if discovery is
ambiguous. Names are case-insensitive; conflicting names and symbolic links in
required files are rejected.

## Original two-disc edition

The expected tree is:

```text
Titanic Game Data/
  cd1/
    data/BOOTFILE
    data/BEDSIT1.SET
    data/MAIN.STG
    data/CTL.STG
    movies/...
    puppets2/...
    [other original runtime folders]
  cd2/
    DATA/A14.SET
    DATA/DECKBD.SET
    DATA/CARGO.SET
    MOVIES/...
    PUPPETS1/...
    [other original runtime folders]
```

The complete names-only index is
[`GameFileIndex.swift`](../native/restoration/Sources/GameFileIndex.swift):
213 paths on Disc 1 and 323 on Disc 2. A few sample files are insufficient.
Preparation checks all 536 paths for nonempty regular files and verifies the
seven main DreamFactory container headers and declared lengths. It preserves
the original file bytes in their respective discs while normalizing path case.

Keep the discs separate. The engine selects the correct disc from original game
scripts, and same-name files can contain different data. A single merged `DATA`
folder does not establish which original disc variants it contains. See the
[upstream game-data documentation](https://github.com/dhobi/dreamrefactory/blob/b43a02668f3db36519bd5b44a5892fdefd292208/taoot/README.md#game-data).

ISO, BIN/CUE, and other disc-image containers must first be mounted or extracted
to ordinary folders containing the Windows game data. This helper does not
decode disc images or support the original Macintosh edition's data format.

## Known digital LOCAL profile

Digital data preparation is content-based. The helper recognizes a folder named
`LOCAL`, requires all 440 unique runtime basenames, and verifies each file's size
and every raw MD5 chunk against
[`gog-local-profile.json`](../scripts/gog-local-profile.json). It then maps these
known digital files into all 536 paths expected by the engine. Shared files are
copied to each required destination according to this digital edition's layout;
this does not infer that arbitrary merged original discs are equivalent.

The profile records official public GOG metadata for product `1792718486`, build
`50837422884815054`, version `1.0 tour fix`, depot
`242b2b5e81c10c17ac8826e9fca30fc3`, inspected on September 9, 2026. It contains
filenames, sizes, checksums, and source URLs, with no game media. The
[official build list](https://content-system.gog.com/products/1792718486/os/windows/builds?generation=2)
and [depot metadata](https://gog-cdn-fastly.gog.com/content-system/v2/meta/24/2b/242b2b5e81c10c17ac8826e9fca30fc3)
identify the source. Bonus files outside the engine's required runtime list are
not imported by this helper.

The same recognition works on an extracted installer folder or an installed
game folder containing `LOCAL`. An installed Steam copy is accepted only if its
required files match this profile; the current Steam package has not been
independently inspected. Future releases or modified files may differ and will
be rejected with the filename and mismatch type. Keep the original files and
report the store, language, version, and helper result before extending a
profile.

For verification, all 440 required digital files were reconstructed locally
from owned original discs only where the complete bytes matched the official
GOG chunk checksums and sizes. The helper's actual prepared output completed
all 27 scripted story segments through the good ending without engine errors,
including 26 complete save snapshots and five actual save loads. This verifies
the known digital content mapping; it does not substitute for testing a newly
purchased installer package or a different store build.

## Optional GOG offline installer extraction

GOG offers the original Windows game and offline installers on its
[official product page](https://www.gog.com/en/game/titanic_adventure_out_of_time).
After purchasing and downloading through your own account, keep the setup
`.exe` and every corresponding `.bin` part together. With
[innoextract](https://constexpr.org/innoextract/) installed separately:

```sh
python3 scripts/prepare-game-data.py \
  --installer "/path/to/setup_titanic_example.exe" \
  --output "/path/to/Titanic Game Data" --json
```

The helper invokes the external extractor with:

```text
innoextract --extract --test --no-extract-unknown --gog \
  --output-dir <temporary-directory> -- <setup.exe>
```

It never executes `setup.exe`. It extracts into a temporary directory, discovers
the resulting layout, validates the content profile, and publishes the output
only after those checks pass. It removes its own temporary extraction when done.
The [official innoextract manual](https://constexpr.org/innoextract/innoextract.1)
documents multipart files, checksum testing, unsupported installer versions,
and additional extraction tools that some GOG packages require. This helper
does not install those tools automatically.

**The current purchased Titanic offline installer has not been exercised
end-to-end.** Innoextract supports specific installer formats and versions;
successful content-profile validation is separate from installer extraction
compatibility. An installer dry run prints the extraction plan and returns
`extraction-not-run`; it does not claim the installer or game data was validated.

## Results and failures

`--json` emits one result object. `validated` means all required local files
passed the selected layout checks and nothing was written. `prepared` also
means a complete new output folder was published. `error` returns exit code 2
and explains the missing file, checksum mismatch, unsupported layout, or output
problem. Extractor progress appears on stderr, leaving stdout available for JSON.

The helper never overwrites an existing destination. It creates a staging
directory beside the destination, checks the copied trees (including the
digital checksums again after copying), and renames it into
place only after completion. Keep source folders unchanged during preparation.
It does not copy executables, personal saves, installer extras, or unrelated
files into the prepared game data.

Portable tests use generated bytes rather than commercial game files:

```sh
python3 -m unittest discover -s scripts/tests -v
```
