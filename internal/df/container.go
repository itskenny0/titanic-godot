package df

import "fmt"

const HeaderSize = 1024

type Container struct {
	ID   int32
	Data []byte
	Gap  bool
}
type FileHeader struct {
	FourCC         int32
	FileSize       int32
	ContainerCount int32
	Type           int32
	GapWhere       int32
}
type File struct {
	Header     FileHeader
	Containers []Container
	HeaderRaw  []byte
	Order      Order
}

func ReadContainer(data []byte, pos int, order Order) (Container, error) {
	r := NewReader(data, order)
	r.Seek(pos)
	c := Container{ID: r.I32()}
	size := uint64(r.U32())
	if size > uint64(len(data)) {
		return c, fmt.Errorf("container at %d: invalid size %d", pos, size)
	}
	c.Data = r.Take(int(size))
	return c, r.Err
}

func ReadFile(data []byte) (*File, error) {
	r := NewReader(data, DetectOrder(data))
	f := &File{Order: r.Order}
	f.Header.FourCC = r.I32()
	f.Header.FileSize = r.I32()
	r.Skip(12)
	f.Header.ContainerCount = r.I32()
	f.Header.Type = r.I32()
	f.Header.GapWhere = r.I32()
	if r.Err != nil {
		return nil, r.Err
	}
	n := int(f.Header.ContainerCount)
	if len(data) < HeaderSize || n < 0 || n > (len(data)-HeaderSize)/4 {
		return nil, fmt.Errorf("invalid container count %d for %d bytes", n, len(data))
	}
	f.HeaderRaw = append([]byte(nil), data[:HeaderSize]...)
	f.Containers = make([]Container, n)
	r.Seek(HeaderSize)
	for i := range f.Containers {
		pos := uint64(r.U32())
		gap := pos <= HeaderSize
		switch f.Header.Type {
		case 1:
			gap = int32(i) == f.Header.GapWhere
		case 2:
			gap = int32(i) == f.Header.GapWhere || int32(i) == f.Header.GapWhere-1
		}
		if gap {
			f.Containers[i] = Container{ID: int32(i), Data: make([]byte, 8), Gap: true}
			continue
		}
		if pos > uint64(len(data)) {
			return nil, fmt.Errorf("container %d position %d exceeds file", i, pos)
		}
		c, err := ReadContainer(data, int(pos), f.Order)
		if err != nil {
			return nil, fmt.Errorf("container %d: %w", i, err)
		}
		f.Containers[i] = c
	}
	return f, nil
}

func (f *File) Data(loc int) []byte {
	if loc < 0 || loc >= len(f.Containers) || f.Containers[loc].Gap {
		return nil
	}
	return f.Containers[loc].Data
}

// Patch leaves the source file bytes unchanged, including on edit failure.
func (f *File) Patch(loc int, edit func([]byte) error) error {
	d := f.Data(loc)
	if d == nil {
		return fmt.Errorf("missing container %d", loc)
	}
	d = append([]byte(nil), d...)
	if err := edit(d); err != nil {
		return err
	}
	f.Containers[loc].Data = d
	return nil
}

func (f *File) Bytes() ([]byte, error) {
	gap := func(i int) bool {
		switch f.Header.Type {
		case 1:
			return int32(i) == f.Header.GapWhere
		case 2:
			return int32(i) == f.Header.GapWhere || int32(i) == f.Header.GapWhere-1
		}
		return f.Containers[i].Gap
	}
	total := uint64(HeaderSize) + uint64(len(f.Containers))*4
	for i, c := range f.Containers {
		if !gap(i) {
			total += 8 + uint64(len(c.Data))
		}
	}
	if total > 0x7fffffff {
		return nil, fmt.Errorf("container file too large: %d", total)
	}
	out := make([]byte, int(total))
	copy(out, f.HeaderRaw)
	o := f.Order
	if o == nil {
		o = DetectOrder(nil)
	}
	o.PutUint32(out, uint32(f.Header.FourCC))
	o.PutUint32(out[4:], uint32(total))
	o.PutUint32(out[20:], uint32(len(f.Containers)))
	o.PutUint32(out[24:], uint32(f.Header.Type))
	o.PutUint32(out[28:], uint32(f.Header.GapWhere))
	pos := HeaderSize + len(f.Containers)*4
	for i, c := range f.Containers {
		if gap(i) {
			continue
		}
		o.PutUint32(out[HeaderSize+i*4:], uint32(pos))
		o.PutUint32(out[pos:], uint32(c.ID))
		o.PutUint32(out[pos+4:], uint32(len(c.Data)))
		copy(out[pos+8:], c.Data)
		pos += 8 + len(c.Data)
	}
	return out, nil
}
