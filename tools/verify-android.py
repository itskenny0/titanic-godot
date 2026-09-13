#!/usr/bin/env python3
"""Check the packaged APK's architecture and native page-size compatibility."""
from pathlib import Path
import argparse, re, subprocess, tempfile, zipfile
p=argparse.ArgumentParser();p.add_argument('apk');p.add_argument('--require-bundled-patches',action='store_true');a=p.parse_args()
with zipfile.ZipFile(a.apk) as z, tempfile.TemporaryDirectory() as temporary:
    native=[n for n in z.namelist() if n.startswith('lib/') and n.endswith('.so')]
    if set(native) != {'lib/arm64-v8a/libgodot_android.so','lib/arm64-v8a/libtitanic_go.so'}:
        raise SystemExit('Unexpected APK native libraries: '+str(native))
    library=z.read('lib/arm64-v8a/libgodot_android.so')
    if b'taoot_player_call' not in library:raise SystemExit('Missing Go gameplay bridge')
    for name in native:
        path=Path(temporary)/Path(name).name;path.write_bytes(z.read(name))
        headers=subprocess.check_output(['readelf','-lW',str(path)],text=True)
        for line in headers.splitlines():
            if line.strip().startswith('LOAD') and int(line.split()[-1],16)<16384:
                raise SystemExit(name+' is not aligned for 16 KiB pages')
        deps=subprocess.check_output(['readelf','-d',str(path)],text=True)
        if 'libc++_shared' in deps:raise SystemExit('Android C++ runtime must be static')
        if name.endswith('/libgodot_android.so') and 'libtitanic_go.so' not in deps:raise SystemExit('Godot is not linked to Go gameplay')
    if 'assets/engine.js' in z.namelist():raise SystemExit('Obsolete JavaScript engine in APK')
    for name in ['assets/required_files.json','assets/patches/manifest.json','assets/notices/COPYING.txt','assets/notices/Go.txt']:
        if name not in z.namelist():raise SystemExit('Missing APK payload: '+name)
    patches = len([n for n in z.namelist() if n.startswith('assets/patches/files/') and n.endswith('.SET')])
    if patches not in (0, 56) or (a.require_bundled_patches and patches != 56):
        raise SystemExit('Missing or incomplete bundled patches')
print('Android APK: ARM64, 16 KiB native alignment, embedded engine, patches, and notices verified.')
