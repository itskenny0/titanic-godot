#!/usr/bin/env python3
"""Check that archived build sources match every tracked file at a revision."""
import argparse
import hashlib
import subprocess
import tarfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('archive')
parser.add_argument('--ref', required=True)
args = parser.parse_args()
entries = subprocess.check_output(['git', 'ls-tree', '-rz', args.ref]).split(b'\0')
expected = {}
for entry in entries:
    if not entry:
        continue
    metadata, name = entry.split(b'\t', 1)
    mode, kind, digest = metadata.decode().split()
    if kind != 'blob' or mode not in ('100644', '100755'):
        raise SystemExit('Unsupported source entry: ' + name.decode())
    expected['titanic-godot/' + name.decode()] = digest
with tarfile.open(args.archive, 'r:gz') as archive:
    seen = set()
    for member in archive:
        if not member.isfile() or member.name not in expected or member.name in seen:
            raise SystemExit('Unexpected archive member: ' + member.name)
        seen.add(member.name)
        digest = hashlib.sha1(f'blob {member.size}\0'.encode())
        with archive.extractfile(member) as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b''):
                digest.update(block)
        if digest.hexdigest() != expected[member.name]:
            raise SystemExit('Source differs from tag: ' + member.name)
    if seen != expected.keys():
        raise SystemExit('Archive is missing tagged source files')
print(f'Verified {len(seen)} source files against {args.ref}')
