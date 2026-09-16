package main

/*
#include <stdlib.h>
#include "../../native/module/player_api.h"
static void taoot_bridge_call(void *callback, uintptr_t host, const char *method, const char *args, const uint8_t *data, int64_t size, TaootResult *out) {
 ((TaootBridgeFn)callback)(host, method, args, data, size, out);
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/cgo"
	"unsafe"

	"github.com/itskenny0/titanic-godot/internal/engine"
	"github.com/itskenny0/titanic-godot/internal/iso9660"
	"github.com/itskenny0/titanic-godot/internal/patches"
)

type nativePlayer struct {
	*engine.Player
	patches patches.Manager
}

type nativePlayerBridge struct {
	host     C.uintptr_t
	callback unsafe.Pointer
}

func (b *nativePlayerBridge) call(method string, args any, data []byte) ([]byte, int, error) {
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, 0, err
	}
	m, a := C.CString(method), C.CString(string(encoded))
	defer C.free(unsafe.Pointer(m))
	defer C.free(unsafe.Pointer(a))
	var input unsafe.Pointer
	if len(data) > 0 {
		input = C.CBytes(data)
		defer C.free(input)
	}
	var result C.TaootResult
	C.taoot_bridge_call(b.callback, b.host, m, a, (*C.uint8_t)(input), C.int64_t(len(data)), &result)
	if result.data != nil {
		defer C.free(unsafe.Pointer(result.data))
	}
	if result.size < 0 || result.size > 512<<20 || result.size > 0 && result.data == nil {
		return nil, 0, fmt.Errorf("invalid platform response buffer")
	}
	var out []byte
	if result.data != nil {
		out = C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	}
	if result.kind < 0 {
		return nil, 0, fmt.Errorf("platform callback: %s", out)
	}
	return out, int(result.kind), nil
}
func (b *nativePlayerBridge) Read(path string) ([]byte, error) {
	data, kind, err := b.call("read", map[string]string{"path": path}, nil)
	if err != nil {
		return nil, err
	}
	if kind != 2 {
		return nil, nil
	}
	return data, nil
}
func (b *nativePlayerBridge) Write(path string, data []byte) error {
	out, _, err := b.call("write", map[string]string{"path": path}, data)
	if err != nil {
		return err
	}
	var status struct{ Error string }
	if len(out) > 0 {
		if err = json.Unmarshal(out, &status); err != nil {
			return err
		}
	}
	if status.Error != "" {
		return fmt.Errorf("%s", status.Error)
	}
	return nil
}
func (b *nativePlayerBridge) Measure(text, font string) (float64, error) {
	out, _, err := b.call("measure", map[string]string{"text": text, "font": font}, nil)
	if err != nil {
		return 0, err
	}
	var width float64
	if err = json.Unmarshal(out, &width); err != nil {
		return 0, err
	}
	return width, nil
}

//export taoot_player_new
func taoot_player_new(host C.uintptr_t, callback unsafe.Pointer) (handle C.uintptr_t) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		if recover() != nil {
			handle = 0
		}
	}()
	if callback == nil {
		return 0
	}
	return C.uintptr_t(cgo.NewHandle(&nativePlayer{Player: engine.NewPlayer(&nativePlayerBridge{host: host, callback: callback})}))
}

//export taoot_player_close
func taoot_player_close(handle C.uintptr_t) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() { _ = recover() }()
	h := cgo.Handle(handle)
	p := h.Value().(*nativePlayer)
	defer h.Delete()
	p.patches.Cancel()
	p.Close()
}
func playerResult(out *C.TaootResult, data []byte, kind int) {
	out.kind = C.int32_t(kind)
	out.size = C.int64_t(len(data))
	out.data = nil
	if len(data) > 0 {
		out.data = (*C.uint8_t)(C.CBytes(data))
	}
}

//export taoot_player_call
func taoot_player_call(handle C.uintptr_t, method, args *C.char, out *C.TaootResult) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if out == nil {
		return
	}
	*out = C.TaootResult{}
	defer func() {
		if err := recover(); err != nil {
			playerResult(out, []byte(fmt.Sprint(err)), -1)
		}
	}()
	p := cgo.Handle(handle).Value().(*nativePlayer)
	name, raw := C.GoString(method), []byte(C.GoString(args))
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var value any
	var err error
	switch name {
	case "iso_index":
		var request struct {
			Path string `json:"path"`
		}
		err = json.Unmarshal(raw, &request)
		if err == nil {
			var entries map[string]iso9660.Entry
			entries, err = iso9660.Open(request.Path)
			value = map[string]any{"files": entries}
		}
		if err != nil {
			value = map[string]string{"error": err.Error()}
			err = nil
		}
	case "patch_start":
		var request patches.Request
		err = json.Unmarshal(raw, &request)
		if err == nil {
			err = p.patches.Start(request)
		}
	case "patch_status":
		value = p.patches.Status()
	case "patch_cancel":
		p.patches.Cancel()

	case "boot":
		var config engine.PlayerConfig
		err = json.Unmarshal(raw, &config)
		if err == nil {
			err = p.Boot(config)
		}
	case "tick":
		var c struct {
			DT float64 `json:"dt"`
		}
		err = json.Unmarshal(raw, &c)
		if err == nil {
			err = p.Tick(c.DT)
		}
	case "command":
		var c engine.PlayerCommand
		err = json.Unmarshal(raw, &c)
		if err == nil {
			err = p.Command(c)
		}
	case "profile":
		var c struct {
			On bool `json:"on"`
		}
		err = json.Unmarshal(raw, &c)
		if err == nil {
			p.Profile(c.On)
		}
	case "state":
		value = p.State()
	case "adaptive_layout":
		value = p.AdaptiveLayout()
	case "adaptive_atlas":
		playerResult(out, p.AdaptiveAtlas(), 2)
		return
	case "controls":
		value = p.ControllerSurface(false)
	case "targets":
		value = p.ControllerSurface(true)
	case "events":
		value = p.Events()
	case "overlay":
		value = p.Overlay()
	case "timings":
		value = p.Timings()
	case "test":
		var c engine.PlayerTestCommand
		err = json.Unmarshal(raw, &c)
		if err == nil {
			var binary bool
			value, binary, err = p.TestOperation(c)
			if err == nil && binary {
				playerResult(out, value.([]byte), 2)
				return
			}
		}
	case "memory":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		value = map[string]any{"allocated": m.HeapSys, "used": m.HeapAlloc, "objects": m.HeapObjects, "gc": m.NumGC, "runtime": "go"}
	case "frame":
		playerResult(out, p.Frame(), 2)
		return
	case "audio":
		var c struct {
			ID uint64 `json:"id"`
		}
		err = json.Unmarshal(raw, &c)
		if err == nil {
			playerResult(out, p.TakeAudio(c.ID), 2)
			return
		}
	default:
		err = fmt.Errorf("unknown player operation %q", name)
	}
	if err != nil {
		playerResult(out, []byte(err.Error()), -1)
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		playerResult(out, []byte(err.Error()), -1)
		return
	}
	playerResult(out, data, 1)
}
