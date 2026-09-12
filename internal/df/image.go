package df

import "fmt"

type FrameBuffer struct {
	Pixels, ZPixels []byte
	Width, Height   int
}
type Frame struct {
	Width, Height int
	HasZ          bool
	ZOffset       int
}

func (f *FrameBuffer) Ensure(w, h int) error {
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
		return fmt.Errorf("invalid frame dimensions %dx%d", w, h)
	}
	n := w * h
	if len(f.Pixels) < n {
		p := make([]byte, n)
		copy(p, f.Pixels)
		f.Pixels = p
		f.ZPixels = make([]byte, n)
	}
	f.Width, f.Height = w, h
	return nil
}

// DecodeFrame updates a persistent indexed buffer. Rows may reference either
// already decoded pixels or pixels below them from the preceding frame.
func DecodeFrame(data []byte, fb *FrameBuffer, order Order) (Frame, error) {
	r := NewReader(data, order)
	h, w := int(r.I16()), int(r.I16())
	if r.Err != nil {
		return Frame{}, r.Err
	}
	if err := fb.Ensure(w, h); err != nil {
		return Frame{}, err
	}
	out := fb.Pixels
	pos, lookback := 0, 0
	copyBack := func(n, back int) error {
		src := pos - back
		if src < 0 || src > len(out)-n {
			return fmt.Errorf("frame: invalid back-reference %d at %d", back, pos)
		}
		if back > 0 && back < n {
			for i := 0; i < n; i++ {
				out[pos+i] = out[src+i]
			}
		} else {
			copy(out[pos:pos+n], out[src:src+n])
		}
		return nil
	}
	for row := 0; row < h; row++ {
		mode := int(r.U8() >> 2)
		written := 0
		if mode == 1 {
			copy(out[pos:pos+w], r.Take(w))
			pos += w
			written = w
		}
		switch {
		case mode <= 5:
			lookback = w * (6 - mode)
		case mode <= 9:
			lookback = w * (5 - mode)
		case mode == 10:
			written = w
			pos += w
		case mode <= 18:
			if mode <= 14 {
				lookback = w * (15 - mode)
			} else {
				lookback = w * (14 - mode)
			}
			if err := copyBack(w, lookback); err != nil {
				return Frame{}, err
			}
			written = w
			pos += w
		default:
			return Frame{}, fmt.Errorf("frame: bad row mode %d at row %d", mode, row)
		}
		for written < w && r.Err == nil {
			b := r.U8()
			run, n := int(b&7), int(b>>3)
			if n == 0 {
				n = 32 + int(r.U8())
			}
			if n > w-written {
				return Frame{}, fmt.Errorf("frame: run overruns row %d", row)
			}
			switch run {
			case 2: // Keep the preceding frame's pixels.
			case 3:
				if err := copyBack(n, lookback); err != nil {
					return Frame{}, err
				}
			case 4:
				if pos == 0 {
					return Frame{}, fmt.Errorf("frame: missing previous pixel")
				}
				v := out[pos-1]
				for i := 0; i < n; i++ {
					out[pos+i] = v
				}
			case 5:
				copy(out[pos:pos+n], r.Take(n))
			case 6:
				v := r.U8()
				for i := 0; i < n; i++ {
					out[pos+i] = v
				}
			case 7:
				if err := copyBack(n, int(r.U16())); err != nil {
					return Frame{}, err
				}
			default:
				if err := decodeBits(r, out, pos, n, run, lookback); err != nil {
					return Frame{}, err
				}
			}
			written += n
			pos += n
		}
		if r.Err != nil {
			return Frame{}, r.Err
		}
	}
	f := Frame{w, h, r.Pos < len(data), -1}
	if f.HasZ {
		f.ZOffset = r.Pos
		if err := decodeZ(data, r.Pos, h, fb.ZPixels, r.Order); err != nil {
			return Frame{}, err
		}
	}
	return f, nil
}

func decodeBits(r *Reader, out []byte, pos, count, mode, rowBack int) error {
	i, back := 0, 1
	if mode == 0 {
		out[pos] = r.U8()
		i++
	} else {
		back = rowBack
	}
	// The codec peeks ahead through the end of a run. Missing lookahead bytes
	// are zero; consumed bytes must still be present.
	peek := func(p int) uint32 {
		if p < 0 || p >= len(r.Data) {
			return 0
		}
		return uint32(r.Data[p])
	}
	at := r.Pos
	flags := peek(at)<<24 | peek(at+1)<<16 | peek(at+2)<<8 | peek(at+3)
	at += 2
	bitPos := 16
	for ; i < count; i++ {
		src := pos + i - back
		if src < 0 || src >= len(out) {
			return fmt.Errorf("frame: invalid bit predictor at %d", pos+i)
		}
		first := 0
		for j := 15; j >= 0; j-- {
			if flags&(uint32(1)<<uint(j+16)) != 0 {
				first = j
				break
			}
		}
		switch {
		case first == 15:
			out[pos+i] = out[src]
			bitPos--
			flags <<= 1
		case first < 8:
			out[pos+i] = byte(flags>>16) + out[src]
			bitPos -= 16
			flags <<= 16
		default:
			diff := 15 - first
			if flags&(uint32(1)<<uint(first+15)) != 0 {
				out[pos+i] = out[src] + byte(diff)
			} else {
				out[pos+i] = out[src] - byte(diff)
			}
			bitPos -= diff + 2
			flags <<= uint(diff + 2)
		}
		if bitPos < 0 {
			flags >>= uint(-bitPos)
			at += 2
			flags |= peek(at)<<8 | peek(at+1)
			flags <<= uint(-bitPos)
			bitPos += 16
		}
	}
	if bitPos >= 8 {
		at--
	}
	if at > len(r.Data) {
		return fmt.Errorf("frame: truncated bit stream")
	}
	r.Pos = at
	return r.Err
}

func decodeZ(data []byte, start, height int, out []byte, order Order) error {
	r := NewReader(data, order)
	pos := 0
	for y := 0; y < height; y++ {
		r.Seek(start + y*2)
		offset := int(r.U16())
		r.Seek(start + offset)
		runs := int(r.U8())
		for j := 0; j < runs; j++ {
			n, v := int(r.U8()), r.U8()
			if n > len(out)-pos {
				return fmt.Errorf("frame: depth layer exceeds image")
			}
			for i := 0; i < n; i++ {
				out[pos+i] = v
			}
			pos += n
		}
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

func IndexedRGBA(pixels, palette, rgba []byte) error {
	if len(palette) < 1024 || len(pixels) > len(rgba)/4 {
		return fmt.Errorf("invalid palette or RGBA buffer size")
	}
	for i, index := range pixels {
		p := int(index) * 4
		copy(rgba[i*4:i*4+3], palette[p:p+3])
		rgba[i*4+3] = 255
	}
	return nil
}
