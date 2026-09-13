#!/usr/bin/env python3
from pathlib import Path
import shutil,sys
root=Path(__file__).resolve().parents[1];source=root/'.build/godot4'
java=source/'platform/android/java/lib/src/cat/kenny/taoot';java.mkdir(parents=True,exist_ok=True)
shutil.copy2(root/'native/android/TitanicFiles.java',java/'TitanicFiles.java')
(source/'platform/android/java/lib/src/io/github/itskenny0/titanic/TitanicFiles.java').unlink(missing_ok=True)
manifest=source/'platform/android/java/app/AndroidManifest.xml';text=manifest.read_text()
text=text.replace("io.github.itskenny0.titanic.TitanicFiles", "cat.kenny.taoot.TitanicFiles")
if 'org.godotengine.plugin.v1.TitanicFiles' not in text:text=text.replace('        <profileable','        <meta-data android:name="org.godotengine.plugin.v1.TitanicFiles" android:value="cat.kenny.taoot.TitanicFiles" />\n        <profileable')
manifest.write_text(text)
# Use the pinned NDK installed by CI and align the ARM64 shared library for 16 KiB pages.
for relative in ['platform/android/detect.py','platform/android/java/app/config.gradle']:
 p=source/relative;p.write_text(p.read_text().replace('23.2.8568313','26.1.10909125'))
p=source/'platform/android/detect.py';text=p.read_text()
if '-Wl,-z,max-page-size=16384' not in text:text=text.replace('    # Link flags','    env.Append(LINKFLAGS=["-Wl,-z,max-page-size=16384"])\n\n    # Link flags')
p.write_text(text)

# NDK 26 moved libc++ into the LLVM sysroot.
p=source/'platform/android/SCsub';text=p.read_text()
text=text.replace('str(env["ANDROID_NDK_ROOT"]) + "/sources/cxx-stl/llvm-libc++/libs/" + lib_arch_dir + "/libc++_shared.so"', 'str(env["ANDROID_NDK_ROOT"]) + "/toolchains/llvm/prebuilt/linux-x86_64/sysroot/usr/lib/" + {"arm64-v8a": "aarch64-linux-android", "armeabi-v7a": "arm-linux-androideabi", "x86": "i686-linux-android", "x86_64": "x86_64-linux-android"}[lib_arch_dir] + "/libc++_shared.so"')
p.write_text(text)

# Static libc++ avoids the NDK 26 shared library's 4 KiB page alignment.
p=source/'platform/android/detect.py';text=p.read_text()
if '-static-libstdc++' not in text:text=text.replace('    # Link flags', '    env.Append(LINKFLAGS=["-static-libstdc++"])\n\n    # Link flags')
p.write_text(text)
p=source/'platform/android/SCsub';text=p.read_text().replace('    env_android.Command(out_dir + "/libc++_shared.so", stl_lib_path, Copy("$TARGET", "$SOURCE"))', '    # libc++ is linked statically into libgodot_android.so.')
p.write_text(text)
(source/'platform/android/java/lib/libs/debug/arm64-v8a/libc++_shared.so').unlink(missing_ok=True)

# R8 shrinks the release Java/Kotlin code. Keep JNI and reflective plugin entry points.
shutil.copy2(root/'packaging/android/proguard-rules.pro', source/'platform/android/java/app/titanic-proguard.pro')
p=source/'platform/android/java/app/build.gradle';text=p.read_text()
if 'titanic-proguard.pro' not in text:
 text=text.replace('        release {\n            // Signing', "        release {\n            minifyEnabled true\n            proguardFiles getDefaultProguardFile('proguard-android-optimize.txt'), 'titanic-proguard.pro'\n            // Signing")
p.write_text(text)
