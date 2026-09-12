#!/usr/bin/env python3
"""Fetch exactly the user-requested M3tox FULL pack; verify before installation."""
from pathlib import Path
import argparse, hashlib, json, shutil, tempfile, urllib.request, zipfile
p = argparse.ArgumentParser()
p.add_argument('--archive', type=Path, help='Use an already downloaded archive')
a = p.parse_args()
root = Path(__file__).resolve().parents[1]
manifest = json.loads((root/'godot/patches/manifest.json').read_text())
cache = root/'.build/downloads'
cache.mkdir(parents=True, exist_ok=True)
archive = a.archive or cache/'TAOOTpatch1.03.FULL.zip'
def digest(path):
    with path.open('rb') as f:
        return hashlib.file_digest(f, 'sha256').hexdigest()
if not archive.exists():
    temporary = archive.with_suffix('.download')
    with urllib.request.urlopen(manifest['url'], timeout=120) as src, temporary.open('wb') as dst:
        shutil.copyfileobj(src, dst)
    if digest(temporary) != manifest['sha256']:
        temporary.unlink()
        raise SystemExit('Patch archive checksum mismatch')
    temporary.replace(archive)
if digest(archive) != manifest['sha256']:
    raise SystemExit('Patch archive checksum mismatch')
target = root/'godot/patches/files'
with tempfile.TemporaryDirectory(dir=cache) as temporary:
    staging = Path(temporary)
    with zipfile.ZipFile(archive) as z:
        if set(z.namelist()) != set(manifest['files']):
            raise SystemExit('Unexpected patch archive contents')
        for name, expected in manifest['files'].items():
            data = z.read(name)
            if len(data) != expected['size'] or hashlib.sha256(data).hexdigest() != expected['sha256']:
                raise SystemExit('Patch file checksum mismatch: '+name)
            (staging/name).write_bytes(data)
    target.mkdir(parents=True, exist_ok=True)
    for path in staging.iterdir():
        shutil.copy2(path, target/path.name)
print('Verified and installed 56 M3tox v1.0.3 patch files for packaging.')
