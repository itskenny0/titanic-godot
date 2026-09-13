#!/usr/bin/env python3
"""Exercise the Go C ABI, including platform callbacks on the calling thread."""
import argparse
import ctypes as C
import json
from pathlib import Path
import threading

parser = argparse.ArgumentParser()
parser.add_argument('library', type=Path)
args = parser.parse_args()
lib = C.CDLL(str(args.library.resolve()))
libc = C.CDLL(None)
libc.malloc.argtypes = [C.c_size_t]
libc.malloc.restype = C.c_void_p
libc.free.argtypes = [C.c_void_p]

class Result(C.Structure):
    _fields_ = [('data', C.c_void_p), ('size', C.c_int64), ('kind', C.c_int32)]

Callback = C.CFUNCTYPE(None, C.c_size_t, C.c_char_p, C.c_char_p, C.c_void_p, C.c_int64, C.POINTER(Result))
caller = threading.get_native_id()
calls = []
errors = []

def reply(out, value, kind):
    data = value if isinstance(value, bytes) else json.dumps(value).encode()
    ptr = libc.malloc(max(1, len(data)))
    C.memmove(ptr, data, len(data))
    out[0] = Result(ptr, len(data), kind)

@Callback
def bridge(host, method, raw, data, size, out):
    try:
        assert threading.get_native_id() == caller, 'Godot callback moved to a worker thread'
        assert host == 73
        method = method.decode()
        payload = json.loads(raw)
        calls.append((method, payload))
        if method == 'read':
            out[0] = Result(None, 0, 0)
        elif method == 'measure':
            reply(out, len(payload['text']) * 7, 1)
        elif method == 'write':
            raise AssertionError('unexpected write')
        else:
            raise AssertionError(f'unknown platform operation {method}')
    except Exception as error:
        errors.append(str(error))
        reply(out, str(error).encode(), -1)

lib.taoot_player_new.argtypes = [C.c_size_t, Callback]
lib.taoot_player_new.restype = C.c_size_t
lib.taoot_player_close.argtypes = [C.c_size_t]
lib.taoot_player_call.argtypes = [C.c_size_t, C.c_char_p, C.c_char_p, C.POINTER(Result)]
handle = lib.taoot_player_new(73, bridge)
assert handle

def call(method, payload=None, expected=1):
    result = Result()
    lib.taoot_player_call(handle, method.encode(), json.dumps(payload or {}).encode(), C.byref(result))
    try:
        data = C.string_at(result.data, result.size) if result.size else b''
        assert result.kind == expected, (method, result.kind, data)
        return json.loads(data) if expected == 1 else data
    finally:
        if result.data:
            libc.free(result.data)

try:
    assert call('state') == {}
    call('boot', {'index': {'1/bootfile': 'owned/bootfile'}})
    assert any(method == 'read' for method, _ in calls), calls
    assert call('state')['ready'] is True
    assert any(e['type'] == 'ready' for e in call('events'))
    assert call('events') == []
    call('command', {'action': 'pause', 'on': True})
    state = call('state')
    assert state['paused'] and state['frozen']
    call('tick', {'dt': 50})
    call('command', {'action': 'pause', 'on': False})
    assert not call('state')['frozen']
    call('command', {'action': 'new'})
    dialog = next(e for e in call('events') if e['type'] == 'dialog')
    call('command', {'action': 'reply', 'id': dialog['id'], 'value': False})
    assert not any(e['type'] == 'restart' for e in call('events'))
    assert call('frame', expected=2) == b''
    assert call('audio', {'id': 9999}, expected=2) == b''
    assert call('memory')['runtime'] == 'go'
    assert b'unknown player operation' in call('unsupported', expected=-1)
    assert call('state')['ready'], 'recoverable error damaged runtime'
    assert not errors, errors
finally:
    lib.taoot_player_close(handle)
# Stale handles are rejected without dereferencing freed memory.
assert call('state', expected=-1)
print('Native C ABI tests passed')
