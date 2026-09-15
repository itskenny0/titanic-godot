#!/usr/bin/env python3
from pathlib import Path
import argparse, subprocess
p=argparse.ArgumentParser();p.add_argument('--godot',required=True);p.add_argument('--project',default='godot');p.add_argument('--output',required=True);p.add_argument('--major',type=int,default=3);p.add_argument('--bundle-patches',action='store_true');a=p.parse_args()
project=Path(a.project).resolve();out=Path(a.output).resolve();out.parent.mkdir(parents=True,exist_ok=True)
preset='''[preset.0]
name="Pack"
platform="%s"
runnable=true
export_filter="all_resources"
include_filter="required_files.json,hdpack-support.json,fonts/LICENSE.txt,patches/*,patches/files/*,notices/*"
exclude_filter="hdpack/*,native/*,integration.gd,*-test.gd%s"
export_path=""
script_export_mode=0
[preset.0.options]
binary_format/64_bits=true
binary_format/embed_pck=false
''' % ('Linux/X11' if a.major==3 else 'Linux', '' if a.bundle_patches else ',patches/files/*')
subprocess.run(['python3',str(Path(__file__).resolve().parent/'prepare-notices.py'),'--output',str(project/'notices')],check=True)
(project/'export_presets.cfg').write_text(preset)
if a.major==4:
 subprocess.run([a.godot,'--headless','--path',str(project),'--editor','--import'],check=True)
 command=[a.godot,'--headless','--path',str(project),'--export-pack','Pack',str(out)]
else:
 command=[a.godot,'--path',str(project),'--no-window','--audio-driver','Dummy','--export-pack','Pack',str(out)]
subprocess.run(command,check=True)
if not out.is_file() or out.stat().st_size<100000:raise SystemExit('Missing or empty project pack')
