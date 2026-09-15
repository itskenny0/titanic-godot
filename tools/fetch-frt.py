#!/usr/bin/env python3
"""Fetch checksum-pinned PortMaster runtimes while packaging, never on the console."""
import argparse
import hashlib
import json
from pathlib import Path
import tempfile
import urllib.request

root = Path(__file__).resolve().parents[1]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--output', type=Path, default=root/'.build/frt-runtime')
a = p.parse_args()
manifest = json.loads((root/'packaging/portmaster/runtime.json').read_text())
a.output.mkdir(parents=True, exist_ok=True)
for name, entry in manifest['files'].items():
    target = a.output/name
    data = target.read_bytes() if target.is_file() else b''
    if len(data) != entry['size'] or hashlib.sha256(data).hexdigest() != entry['sha256']:
        url = f"https://raw.githubusercontent.com/PortsMaster/PortMaster-New/{manifest['revision']}/runtimes/{name}"
        print('Downloading pinned runtime:', name, flush=True)
        with urllib.request.urlopen(url, timeout=60) as response:
            data = response.read(entry['size']+1)
        if len(data) != entry['size'] or data[:4] != b'hsqs' or hashlib.sha256(data).hexdigest() != entry['sha256']:
            raise SystemExit('FRT runtime size or checksum mismatch: '+name)
        with tempfile.NamedTemporaryFile(dir=a.output, delete=False) as output:
            temporary = Path(output.name)
            output.write(data)
        temporary.replace(target)
    print('Verified runtime:', name, flush=True)
