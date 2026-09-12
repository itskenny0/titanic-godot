#!/usr/bin/env python3
from pathlib import Path
import argparse, shutil
p=argparse.ArgumentParser();p.add_argument('--output',default='godot/notices');p.add_argument('--godot-source');a=p.parse_args()
root=Path(__file__).resolve().parents[1];out=Path(a.output);out.mkdir(parents=True,exist_ok=True)
for name,source in {'COPYING.txt':'LICENSE','CREDITS.txt':'CREDITS.md','dreamREfactory.txt':'vendor/dreamrefactory/LICENSE','QuickJS.txt':'vendor/quickjs/LICENSE','Godot-headers.txt':'vendor/godot-headers/LICENSE.md','Liberation-fonts.txt':'godot/fonts/LICENSE.txt'}.items():shutil.copy2(root/source,out/name)
if a.godot_source:
 for name in ['LICENSE.txt','COPYRIGHT.txt','AUTHORS.md']:
  shutil.copy2(Path(a.godot_source)/name,out/('Godot-'+name+'.txt'))
