#!/usr/bin/env python3
from pathlib import Path
import argparse,subprocess,os,shutil
p=argparse.ArgumentParser();p.add_argument('--platform',choices=['linux','mac','windows'],required=True);p.add_argument('--arch',choices=['x86_64','arm64'],required=True);p.add_argument('--jobs',default=str(min(4, os.cpu_count() or 2)));p.add_argument('--mingw-prefix');a=p.parse_args()
root=Path(__file__).resolve().parents[1]
major='4' if a.platform=='windows' and a.arch=='arm64' else '3'
# Godot generates shared headers; each platform/architecture needs its own tree.
source=root/'.build'/f'godot{major}-{a.platform}-{a.arch}'
subprocess.run(['python3',str(root/'tools/fetch-godot.py'),major,str(source)],check=True)
go_library=root/'.build/go'/f'{a.platform}-{a.arch}'/'libtitanic_go.a'
go_cmd=['python3',str(root/'tools/build-go.py'),'--platform',a.platform,'--arch',a.arch,'--output',str(go_library)]
if a.mingw_prefix:go_cmd+=['--cc',a.mingw_prefix+'clang']
subprocess.run(go_cmd,check=True)
subprocess.run(['python3',str(root/'tools/install-module.py'),str(source),'--go-library',str(go_library)],check=True)
cmd=['scons','-C',str(source),'-j'+a.jobs,'disable_3d=yes']
if major=='4':
 cmd+=['platform=windows','target=template_release','arch=arm64','use_mingw=yes','use_static_cpp=yes','vulkan=no','d3d12=no']
 if a.mingw_prefix:cmd+=['mingw_prefix='+a.mingw_prefix.removesuffix('aarch64-w64-mingw32-')]
 cmd+=['use_llvm=yes']
else:
 cmd+=['platform='+{'linux':'x11','mac':'osx','windows':'windows'}[a.platform],'tools=no','target=release','debug_symbols=no','module_bullet_enabled=no','module_mono_enabled=no','use_static_cpp=yes']
 if a.platform=='mac':cmd+=['arch='+a.arch]
 elif a.platform=='windows':
  cmd+=['bits=64','use_mingw=yes','use_llvm=yes']
  if a.mingw_prefix:cmd+=['mingw_prefix_64='+a.mingw_prefix]
 else:
  cmd+=['bits=64']
  if a.arch=='arm64':cmd+=['arch=arm64']
subprocess.run(cmd,check=True)
binary_prefix = 'godot.' + ({'linux':'x11','mac':'osx','windows':'windows'}[a.platform] if major=='3' else 'windows') + '.'
binaries=[p for p in (source/'bin').iterdir() if p.is_file() and p.name.startswith(binary_prefix) and p.suffix not in ['.pdb','.a','.lib','.exp'] and not p.name.endswith('.console.exe')]
if len(binaries)!=1:raise SystemExit('Expected exactly one runtime binary: '+str(binaries))
out=root/'dist'/f'runtime-{a.platform}-{a.arch}'
if a.platform=='windows':out=out.with_suffix('.exe')
out.parent.mkdir(exist_ok=True);shutil.copy2(binaries[0],out);out.chmod(0o755)
print(out)
