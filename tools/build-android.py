#!/usr/bin/env python3
"""Build the small ARM64 Android release template from the prepared Godot tree."""
from pathlib import Path
import argparse, os, subprocess

p = argparse.ArgumentParser()
p.add_argument('--jobs', type=int, default=min(4, os.cpu_count() or 2))
a = p.parse_args()
root = Path(__file__).resolve().parents[1]
source = root/'.build/godot4'
# Go decodes the game's pictures, audio and movies. Godot only presents the 2D UI.
# Keep the advanced text server for shaping, font metrics and native text input.
options = ['platform=android', 'target=template_release', 'arch=arm64',
           'vulkan=no', 'disable_3d=yes', 'debug_symbols=no', 'optimize=size', 'lto=thin',
           'modules_enabled_by_default=no', 'module_gdscript_enabled=yes',
           'module_freetype_enabled=yes', 'module_text_server_adv_enabled=yes',
           'module_titanic_enabled=yes']
subprocess.run(['scons', '-C', str(source), *options, '-j'+str(a.jobs)], check=True)
# The pinned Gradle build excludes automatic SCons tasks for command-line builds.
subprocess.run(['./gradlew', '--no-daemon', 'copyReleaseBinaryToBin'],
               cwd=source/'platform/android/java', check=True)
