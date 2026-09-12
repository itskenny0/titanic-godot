#!/usr/bin/env python3
"""Check the packaged APK's architecture and native page-size compatibility."""
from pathlib import Path
import argparse, re, subprocess, tempfile, zipfile
p=argparse.ArgumentParser();p.add_argument('apk');a=p.parse_args()
with zipfile.ZipFile(a.apk) as z, tempfile.TemporaryDirectory() as temporary:
    native=[n for n in z.namelist() if n.startswith('lib/') and n.endswith('.so')]
    if set(native) != {'lib/arm64-v8a/libgodot_android.so','lib/arm64-v8a/libtitanic_go.so'}:
        raise SystemExit('Unexpected APK native libraries: '+str(native))
    library=z.read('lib/arm64-v8a/libgodot_android.so')
    if b'__indexedRGBA' not in library:raise SystemExit('Missing native palette conversion')
    if b'__decodeFrame' not in library:raise SystemExit('Missing Go codec bridge')
    for name in native:
        path=Path(temporary)/Path(name).name;path.write_bytes(z.read(name))
        headers=subprocess.check_output(['readelf','-lW',str(path)],text=True)
        for line in headers.splitlines():
            if line.strip().startswith('LOAD') and int(line.split()[-1],16)<16384:
                raise SystemExit(name+' is not aligned for 16 KiB pages')
        deps=subprocess.check_output(['readelf','-d',str(path)],text=True)
        if 'libc++_shared' in deps:raise SystemExit('Android C++ runtime must be static')
        if name.endswith('/libgodot_android.so') and 'libtitanic_go.so' not in deps:raise SystemExit('Godot is not linked to Go codecs')
    for name in ['assets/engine.js','assets/patches/manifest.json','assets/notices/COPYING.txt']:
        if name not in z.namelist():raise SystemExit('Missing APK payload: '+name)
    if len([n for n in z.namelist() if n.startswith('assets/patches/files/') and n.endswith('.SET')]) != 56:
        raise SystemExit('Missing bundled patches')
print('Android APK: ARM64, 16 KiB native alignment, embedded engine, patches, and notices verified.')
