#!/usr/bin/env python3
"""Package compiled runtimes. Commercial data and personal saves are never inputs."""
from pathlib import Path
import argparse,shutil,subprocess,tarfile,zipfile,plistlib,os
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('kind',choices=['linux','mac','windows','portmaster']);p.add_argument('--binary');p.add_argument('--arch',default='x86_64');p.add_argument('--pack',default='dist/titanic.pck');p.add_argument('--version',default='0.1.0');p.add_argument('--static',action='store_true');a=p.parse_args()
name=f'titanic-{a.kind}-{a.arch}'+('-static' if a.static else '')
stage=ROOT/'.build/packages'/name
if stage.exists():shutil.rmtree(stage)
stage.mkdir(parents=True)
dist=ROOT/'dist';dist.mkdir(exist_ok=True)
def licenses(dest):
 dest.mkdir(parents=True,exist_ok=True)
 for name,src in {'COPYING':ROOT/'LICENSE','CREDITS.md':ROOT/'CREDITS.md','dreamREfactory.txt':ROOT/'vendor/dreamrefactory/LICENSE','QuickJS.txt':ROOT/'vendor/quickjs/LICENSE','Godot-headers.txt':ROOT/'vendor/godot-headers/LICENSE.md','Liberation-fonts.txt':ROOT/'godot/fonts/LICENSE.txt'}.items():shutil.copy2(src,dest/name)
 major='4' if a.kind=='windows' and a.arch=='arm64' else '3'
 godot=ROOT/'.build'/f'godot{major}-{a.kind}-{a.arch}'
 if not godot.exists():godot=ROOT/'.build'/('godot'+major)
 for name in ['LICENSE.txt','COPYRIGHT.txt','AUTHORS.md']:
  if (godot/name).exists():shutil.copy2(godot/name,dest/('Godot-'+name))
 shutil.copy2(ROOT/'README.md',dest.parent/'README.txt')
def zipped(path):
 with zipfile.ZipFile(path,'w',zipfile.ZIP_DEFLATED) as z:
  for f in sorted(stage.rglob('*')):
   if f.is_file():z.write(f,f.relative_to(stage))
if a.kind=='portmaster':
 game=stage/'titanic';game.mkdir()
 shutil.copy2(a.pack,game/'titanic.pck')
 shutil.copy2(ROOT/'packaging/portmaster/Titanic.sh',stage/'Titanic.sh');(stage/'Titanic.sh').chmod(0o755)
 shutil.copy2(ROOT/'packaging/portmaster/titanic.gptk',game/'titanic.gptk')
 shutil.copy2(ROOT/'packaging/portmaster/port.json',game/'port.json')
 (game/'native').mkdir();(game/'gamedata').mkdir();(game/'mods').mkdir()
 for arch in ['aarch64','armhf']:shutil.copy2(ROOT/f'godot/native/libtitanic.{arch}.so',game/'native'/f'libtitanic.{arch}.so')
 licenses(game/'licenses')
 (game/'gamedata/PUT_GAME_FILES_HERE.txt').write_text('Copy prepared cd1 and cd2 folders here. See README.txt.\n')
 shutil.copy2(ROOT/'docs/PORTMASTER.md',game/'README.txt')
 zipped(dist/'titanic-portmaster.zip')
elif a.kind=='mac':
 app=stage/'Titanic.app/Contents';(app/'MacOS').mkdir(parents=True);(app/'Resources').mkdir()
 shutil.copy2(a.binary,app/'MacOS/titanic');(app/'MacOS/titanic').chmod(0o755)
 shutil.copy2(a.pack,app/'Resources/titanic.pck');licenses(app/'Resources/licenses')
 (app/'Info.plist').write_bytes(plistlib.dumps({'CFBundleExecutable':'titanic','CFBundleIdentifier':'io.github.itskenny0.Titanic','CFBundleName':'Titanic','CFBundlePackageType':'APPL','CFBundleShortVersionString':a.version,'NSHighResolutionCapable':True}))
 subprocess.run(['codesign','--force','--deep','--sign','-',str(app.parent)],check=True)
 zipped(dist/f'{name}.zip')
else:
 exe=stage/('titanic.exe' if a.kind=='windows' else 'titanic');shutil.copy2(a.binary,exe);exe.chmod(0o755)
 shutil.copy2(a.pack,stage/'titanic.pck');licenses(stage/'licenses')
 if a.kind=='windows':zipped(dist/f'{name}-portable.zip')
 else:
  if a.static:
   info=subprocess.check_output(['readelf','-l',str(exe)],text=True)
   deps=subprocess.check_output(['readelf','-d',str(exe)],text=True)
   if any(lib in deps for lib in ['libstdc++','libgcc_s','libquickjs','libtitanic']):raise SystemExit('The engine dependencies were not statically linked')
  with tarfile.open(dist/f'{name}.tar.gz','w:gz') as t:t.add(stage,arcname='titanic')
  if not a.static:
   deb=ROOT/'.build/packages'/f'deb-{a.arch}';shutil.rmtree(deb,ignore_errors=True)
   payload=deb/'usr/lib/titanic';shutil.copytree(stage,payload)
   bindir=deb/'usr/bin';bindir.mkdir();launcher=bindir/'titanic';launcher.write_text('#!/bin/sh\nexec /usr/lib/titanic/titanic --main-pack /usr/lib/titanic/titanic.pck "$@"\n');launcher.chmod(0o755)
   debarch={'x86_64':'amd64','arm64':'arm64'}[a.arch];(deb/'DEBIAN').mkdir()
   (deb/'DEBIAN/control').write_text(f'Package: titanic-godot\nVersion: {a.version}\nArchitecture: {debarch}\nMaintainer: itskenny0\nDepends: libc6, libx11-6, libxcursor1, libxinerama1, libxrandr2, libxi6, libgl1, libasound2\nSection: games\nPriority: optional\nDescription: Godot player for Titanic Adventure Out of Time\n Original game files required.\n')
   for src,dest in [('io.github.itskenny0.Titanic.desktop','usr/share/applications'),('io.github.itskenny0.Titanic.svg','usr/share/icons/hicolor/scalable/apps'),('io.github.itskenny0.Titanic.metainfo.xml','usr/share/metainfo')]:
    d=deb/dest;d.mkdir(parents=True,exist_ok=True);shutil.copy2(ROOT/'packaging/linux'/src,d/src)
   subprocess.run(['dpkg-deb','--root-owner-group','--build',str(deb),str(dist/f'{name}.deb')],check=True)
print(stage)
