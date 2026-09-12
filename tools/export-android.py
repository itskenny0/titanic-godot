#!/usr/bin/env python3
from pathlib import Path
import argparse,os,subprocess
p=argparse.ArgumentParser();p.add_argument('--godot',required=True);p.add_argument('--sdk',required=True);a=p.parse_args()
root=Path(__file__).resolve().parents[1];project=root/'.build/godot4-project'
subprocess.run(['python3',str(root/'tools/prepare-notices.py'),'--output',str(project/'notices'),'--godot-source',str(root/'.build/godot4')],check=True)
# The APK template contains the ARM64 engine module and native document picker.
preset=f'''[preset.0]
name="Android"
platform="Android"
runnable=true
export_filter="all_resources"
include_filter="engine.js,required_files.json,fonts/LICENSE.txt,patches/*,patches/files/*,notices/*"
exclude_filter="native/*,integration.gd,*-test.gd"
export_path=""
[preset.0.options]
custom_template/debug="{root}/.build/godot4/bin/android_debug.apk"
architectures/armeabi-v7a=false
architectures/arm64-v8a=true
architectures/x86=false
architectures/x86_64=false
package/unique_name="cat.kenny.taoot"
package/name="Titanic"
version/code=1
version/name="0.1.0"
screen/immersive_mode=true
keystore/debug="{root}/.tools/android-debug.keystore"
keystore/debug_user="androiddebugkey"
keystore/debug_password="android"
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
key=root/'.tools/android-debug.keystore';key.parent.mkdir(exist_ok=True)
if not key.exists():subprocess.run(['keytool','-genkeypair','-keystore',str(key),'-storepass','android','-alias','androiddebugkey','-keypass','android','-keyalg','RSA','-keysize','2048','-validity','10000','-dname','CN=Titanic Development'],check=True)
subprocess.run([a.godot,'--headless','--path',str(project),'--editor','--import'],check=True)
subprocess.run([a.godot,'--headless','--path',str(project),'--export-debug','Android',str(root/'dist/titanic-android-arm64-debug.apk')],check=True)
