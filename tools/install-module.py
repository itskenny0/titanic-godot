#!/usr/bin/env python3
from pathlib import Path
import argparse,shutil
p=argparse.ArgumentParser();p.add_argument('source',type=Path);p.add_argument('--go-library',type=Path);a=p.parse_args()
root=Path(__file__).resolve().parents[1]
target=a.source/'modules/titanic'
shutil.copytree(root/'native/module',target,dirs_exist_ok=True)
shutil.copytree(root/'vendor/quickjs',target/'quickjs',ignore=shutil.ignore_patterns('.git'),dirs_exist_ok=True)
if a.go_library:
 (target/'go').mkdir(exist_ok=True)
 for old in (target/'go').glob('libtitanic_go.*'):old.unlink()
 shutil.copy2(a.go_library,target/'go'/a.go_library.name)
 if a.go_library.suffix=='.so':
  for build in ['debug','release']:
   libs=a.source/'platform/android/java/lib/libs'/build/'arm64-v8a'
   libs.mkdir(parents=True,exist_ok=True);shutil.copy2(a.go_library,libs/a.go_library.name)
