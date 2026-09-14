#!/usr/bin/env python3
from pathlib import Path
import argparse,os,subprocess
p=argparse.ArgumentParser();p.add_argument('--godot',required=True);p.add_argument('--sdk',required=True);p.add_argument('--release',action='store_true');p.add_argument('--bundle-patches',action='store_true');a=p.parse_args()
root=Path(__file__).resolve().parents[1];project=root/'.build/godot4-project'
build='release' if a.release else 'debug'
(root/'dist').mkdir(exist_ok=True)
subprocess.run(['python3',str(root/'tools/prepare-notices.py'),'--output',str(project/'notices'),'--godot-source',str(root/'.build/godot4')],check=True)
# The APK template contains the ARM64 engine module and native document picker.
preset=f'''[preset.0]
name="Android"
platform="Android"
runnable=true
export_filter="all_resources"
include_filter="required_files.json,fonts/LICENSE.txt,patches/*,patches/files/*,notices/*"
exclude_filter="native/*,integration.gd,*-test.gd{'' if a.bundle_patches else ',patches/files/*'}"
export_path=""
[preset.0.options]
custom_template/{build}="{root}/.build/godot4/bin/android_{build}.apk"
architectures/armeabi-v7a=false
architectures/arm64-v8a=true
architectures/x86=false
architectures/x86_64=false
package/unique_name="cat.kenny.taoot"
package/name="Titanic"
version/code=9
version/name="0.3.6"
screen/immersive_mode=true
permissions/internet=true
launcher_icons/main_192x192="res://icons/titanic.png"
launcher_icons/adaptive_foreground_432x432="res://icons/titanic.png"
launcher_icons/adaptive_background_432x432="res://icons/titanic.png"
keystore/{build}="{root}/packaging/android/debug.keystore"
keystore/{build}_user="androiddebugkey"
keystore/{build}_password="android"
'''
(project/'export_presets.cfg').write_text(preset)
settings=Path.home()/'.config/godot/editor_settings-4.3.tres'
settings.parent.mkdir(parents=True,exist_ok=True)
# Run in an isolated XDG_CONFIG_HOME in CI to avoid replacing personal editor settings.
config=os.environ.get('XDG_CONFIG_HOME')
if config:settings=Path(config)/'godot/editor_settings-4.3.tres';settings.parent.mkdir(parents=True,exist_ok=True)
if settings.exists():
 text=settings.read_text()
else:text='[gd_resource type="EditorSettings" format=3]\n[resource]\n'
import re
for key,value in {'export/android/android_sdk_path':a.sdk,'export/android/java_sdk_path':os.environ.get('JAVA_HOME','/usr/lib/jvm/java-17-openjdk-amd64')}.items():
 text=re.sub(r'^'+re.escape(key)+r'\s*=.*\n','',text,flags=re.M)
 text+=f'{key} = "{value}"\n'
text=re.sub(r'^export/android/shutdown_adb_on_exit\s*=.*\n', '', text, flags=re.M)
text+='export/android/shutdown_adb_on_exit = false\n'
settings.write_text(text)
key=root/'packaging/android/debug.keystore'
if not key.is_file():raise SystemExit('Missing shared Android debug keystore: '+str(key))
subprocess.run([a.godot,'--headless','--path',str(project),'--editor','--import'],check=True)
subprocess.run([a.godot,'--headless','--path',str(project),'--export-'+build,'Android',str(root/'dist'/f'titanic-android-arm64-{build}.apk')],check=True)
