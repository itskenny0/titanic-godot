package df

import "encoding/binary"

func PaletteRGBA(raw []byte, colorCount int, order Order) []byte {
	out := make([]byte, 1024)
	hi := 1
	if order == binary.BigEndian {
		hi = 0
	}
	for i := 0; i < min(256, max(0, colorCount)); i++ {
		for c := 0; c < 3; c++ {
			at := i*8 + 2 + c*2 + hi
			if at < len(raw) {
				out[i*4+c] = raw[at]
			}
		}
		out[i*4+3] = 255
	}
	out[3] = 255
	if order != binary.BigEndian {
		clear(out[:3])
		for i := 1020; i < 1024; i++ {
			out[i] = 255
		}
	}
	return out
}
