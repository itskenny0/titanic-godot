#!/usr/bin/env python3
"""Archive the final worktree source, excluding user data, binaries and local caches."""
from pathlib import Path
import tarfile,subprocess
root=Path(__file__).resolve().parents[1]
# ls-files --cached --others honors .gitignore, including uncommitted authored changes.
paths=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','-z'],cwd=root).split(b'\0')
out=root/'dist/titanic-source.tar.gz';out.parent.mkdir(exist_ok=True)
with tarfile.open(out,'w:gz') as archive:
 for raw in sorted(set(paths)):
  if not raw:continue
  relative=raw.decode();p=root/relative
  if not p.is_file() or p.is_symlink():continue
  if any(x in p.parts for x in ['.git','.build','.tools','originalgame','gamedata','dist','node_modules']):continue
  if p.suffix.lower() in ['.ti','.set','.shp','.pup','.mov','.trk','.sfx','.exe','.dll','.so','.dylib']:raise SystemExit('Unexpected binary/game data in source: '+relative)
  archive.add(p,arcname='titanic-godot/'+relative)
print(out)
