// C ABI for the Go decoders. Input and output buffers belong to the caller and
// are borrowed only for the duration of a call; Go never retains their pointers.
package main

/*
#include <stdint.h>
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func order(big C.int) df.Order {
	if big != 0 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}
func bytes(p *C.uint8_t, n C.int64_t) []byte { return unsafe.Slice((*byte)(unsafe.Pointer(p)), int(n)) }
func report(p *C.char, n C.int64_t, err any) {
	if p == nil || n <= 0 {
		return
	}
	out := unsafe.Slice((*byte)(unsafe.Pointer(p)), int(n))
	count := copy(out[:len(out)-1], fmt.Sprint(err))
	out[count] = 0
}

//export taoot_decode_frame
func taoot_decode_frame(data *C.uint8_t, size C.int64_t, pixels, depth *C.uint8_t, capacity C.int64_t, big C.int, message *C.char, messageSize C.int64_t) (result C.int64_t) {
	result = -2
	defer func() {
		if e := recover(); e != nil {
			report(message, messageSize, e)
			result = -2
		}
	}()
	if size < 4 || size > 512<<20 || capacity <= 0 || capacity > 4096*4096 || data == nil || pixels == nil || depth == nil {
		report(message, messageSize, "invalid frame buffers")
		return
	}
	fb := df.FrameBuffer{Pixels: bytes(pixels, capacity), ZPixels: bytes(depth, capacity)}
	d := bytes(data, size)
	r := df.NewReader(d, order(big))
	h, w := int(r.I16()), int(r.I16())
	if h <= 0 || w <= 0 || int64(h)*int64(w) > int64(capacity) {
		report(message, messageSize, "frame buffer too short")
		return
	}
	f, err := df.DecodeFrame(d, &fb, order(big))
	if err != nil {
		report(message, messageSize, err)
		return
	}
	return C.int64_t(f.ZOffset)
}

//export taoot_decode_audio
func taoot_decode_audio(data *C.uint8_t, size C.int64_t, samples *C.float, capacity C.int64_t, big C.int, message *C.char, messageSize C.int64_t) (result C.int) {
	result = -1
	defer func() {
		if e := recover(); e != nil {
			report(message, messageSize, e)
			result = -1
		}
	}()
	if size < 48 || size > 512<<20 || capacity < 0 || capacity > 128<<20 || data == nil || (capacity > 0 && samples == nil) {
		report(message, messageSize, "invalid audio buffers")
		return
	}
	d := bytes(data, size)
	h, err := df.ReadAudioHeader(d, order(big))
	if err != nil {
		report(message, messageSize, err)
		return
	}
	n := h.ByteSize
	if h.Codec != 1 {
		n /= 2
	}
	if int64(n) != int64(capacity) {
		report(message, messageSize, "audio buffer size mismatch")
		return
	}
	_, err = df.DecodeAudio(d, order(big), unsafe.Slice((*float32)(unsafe.Pointer(samples)), int(capacity)))
	if err != nil {
		report(message, messageSize, err)
		return
	}
	return 0
}

//export taoot_stereo_pcm
func taoot_stereo_pcm(samples *C.float, count C.int64_t, pcm *C.uint8_t, volume, pan C.double) (result C.int) {
	result = -1
	defer func() {
		if recover() != nil {
			result = -1
		}
	}()
	if count < 0 || count > 32<<20 || (count > 0 && (samples == nil || pcm == nil)) {
		return
	}
	a := df.Audio{Samples: unsafe.Slice((*float32)(unsafe.Pointer(samples)), int(count))}
	if a.WriteStereoPCM(bytes(pcm, count*4), float64(volume), float64(pan)) != nil {
		return
	}
	return 0
}

func main() {}
