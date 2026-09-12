#!/usr/bin/env python3
from pathlib import Path
import shutil,sys
root=Path(__file__).resolve().parents[1]
target=Path(sys.argv[1])/'modules/titanic'
shutil.copytree(root/'native/module',target,dirs_exist_ok=True)
shutil.copytree(root/'vendor/quickjs',target/'quickjs',ignore=shutil.ignore_patterns('.git'),dirs_exist_ok=True)
