#!/usr/bin/env python3
"""Build the Go gameplay library for a Godot or PortMaster target."""
from pathlib import Path
import argparse, os, platform, subprocess

p = argparse.ArgumentParser()
p.add_argument('--platform', choices=['linux', 'windows', 'mac', 'android'], required=True)
p.add_argument('--arch', choices=['x86_64', 'arm64', 'armhf'], required=True)
p.add_argument('--cc')
p.add_argument('--output', type=Path)
a = p.parse_args()
if a.platform == 'android' and a.arch != 'arm64': p.error('Android builds target ARM64')
if a.platform in ['windows','mac'] and a.arch == 'armhf': p.error('armhf is only a Linux handheld target')
root = Path(__file__).resolve().parents[1]
shared = a.platform == 'android'
out = a.output or root/'.build/go'/f'{a.platform}-{a.arch}'/('libtitanic_go.so' if shared else 'libtitanic_go.a')
out = out.resolve(); out.parent.mkdir(parents=True, exist_ok=True)
env = os.environ.copy()
env['CGO_ENABLED'] = '1'
env['GOOS'] = {'mac': 'darwin'}.get(a.platform, a.platform)
env['GOARCH'] = {'x86_64': 'amd64', 'arm64': 'arm64', 'armhf': 'arm'}[a.arch]
if a.arch == 'armhf': env['GOARM'] = '7'
if a.cc: env['CC'] = a.cc
elif a.platform == 'windows': env['CC'] = ('aarch64' if a.arch == 'arm64' else 'x86_64')+'-w64-mingw32-clang'
elif a.platform == 'android':
    sdk = Path(env.get('ANDROID_HOME', env.get('ANDROID_SDK_ROOT', '/opt/android-sdk')))
    env['CC'] = str(sdk/'ndk/26.1.10909125/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android23-clang')
elif a.platform == 'mac':
    env['CC'] = 'clang'
    flags = f'-arch {a.arch} -mmacosx-version-min=12.0'
    env['CGO_CFLAGS'] = flags; env['CGO_LDFLAGS'] = flags
elif a.platform == 'linux':
    host = platform.machine().lower()
    if a.arch == 'arm64' and host not in ['aarch64','arm64']: env['CC'] = 'aarch64-linux-gnu-gcc'
    elif a.arch == 'armhf' and host not in ['armv7l','arm']: env['CC'] = 'arm-linux-gnueabihf-gcc'
if shared:
    env['CGO_LDFLAGS'] = env.get('CGO_LDFLAGS','')+' -Wl,-z,max-page-size=16384 -Wl,-soname,libtitanic_go.so'
subprocess.run(['go','build','-trimpath','-buildmode='+('c-shared' if shared else 'c-archive'),'-o',str(out),'./cmd/native-codecs'],cwd=root,env=env,check=True)
print(out)
