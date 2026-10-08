#!/usr/bin/env python3
"""Check indexed Android folder reads independently of a device/provider."""
from pathlib import Path
import os, subprocess, tempfile
root=Path(__file__).resolve().parents[1]
java=Path(os.environ['JAVA_HOME'])/'bin' if os.environ.get('JAVA_HOME') else Path('/usr/bin')
with tempfile.TemporaryDirectory() as out:
 subprocess.run([str(java/'javac'),'-d',out,str(root/'native/android/HdPackFolder.java'),str(root/'tests/HdPackFolderTest.java')],check=True)
 subprocess.run([str(java/'java'),'-cp',out,'HdPackFolderTest'],check=True)
