#!/usr/bin/env python3
import argparse, subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('major', choices=['3','4']);p.add_argument('destination');a=p.parse_args()
revision={'3':'170ba337a5d9e4c1e40d63b89dc1ba297b71860b','4':'77dcf97d82cbfe4e4615475fa52ca03da645dbd8'}[a.major]
d=Path(a.destination)
if not (d/'.git').exists():
 d.mkdir(parents=True,exist_ok=True)
 subprocess.run(['git','init',str(d)],check=True)
 subprocess.run(['git','-C',str(d),'remote','add','origin','https://github.com/godotengine/godot.git'],check=True)
 subprocess.run(['git','-C',str(d),'fetch','--depth','1','origin',revision],check=True)
 subprocess.run(['git','-C',str(d),'checkout','--detach','FETCH_HEAD'],check=True)
actual=subprocess.check_output(['git','-C',str(d),'rev-parse','HEAD'],text=True).strip()
if actual!=revision:raise SystemExit('Unexpected Godot revision: '+actual)
