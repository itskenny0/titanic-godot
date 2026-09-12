package df

import (
	"fmt"
	"strings"
)

type BankChunk struct {
	Identifier             string
	ContainerLoc, IDOffset int
	Follow                 string
}
type LoopTable struct {
	Order   []int
	Records []BankChunk
}
type AudioBank struct {
	TrackName   string
	LoopChunks  []int
	Singles     map[string]BankChunk
	SingleOrder []string `json:"-"`
}

func ReadLoopTable(data []byte, order Order) (LoopTable, error) {
	t := LoopTable{Order: []int{}, Records: []BankChunk{}}
	if len(data) < 270 {
		return t, nil
	}
	r := NewReader(data, order)
	n := max(0, min(r.i16At(4), 130))
	for i := 0; i < n; i++ {
		t.Order = append(t.Order, r.i16At(6+i*2))
	}
	n = r.table(r.i32At(266), 270, 26)
	for i := 0; i < n; i++ {
		at := 270 + i*26
		t.Records = append(t.Records, BankChunk{Identifier: r.nameAt(at+10, 15), ContainerLoc: r.i32At(at + 4), IDOffset: at + 10})
	}
	return t, r.Err
}
func ReadLoopChunks(data []byte, order Order) ([]BankChunk, error) {
	t, err := ReadLoopTable(data, order)
	out := []BankChunk{}
	for _, i := range t.Order {
		if i >= 1 && i <= len(t.Records) {
			out = append(out, t.Records[i-1])
		}
	}
	return out, err
}
func ReadOneShotChunks(data []byte, idSize, followSize int, order Order) ([]BankChunk, error) {
	r := NewReader(data, order)
	stride := 11 + idSize
	if followSize > 0 {
		stride += 1 + followSize
	}
	n := r.table(r.i32At(4), 8, stride)
	out := []BankChunk{}
	for i := 0; i < n; i++ {
		at := 8 + i*stride
		c := BankChunk{Identifier: r.nameAt(at+10, idSize), ContainerLoc: r.i32At(at + 4), IDOffset: at + 10}
		if followSize > 0 {
			c.Follow = r.nameAt(at+11+idSize, followSize)
		}
		out = append(out, c)
	}
	return out, r.Err
}
func ReadAudioBank(data []byte) (*AudioBank, error) {
	d, err := openAsset(data)
	if err != nil {
		return nil, err
	}
	r := d.reader(0, d.file.Order)
	if Version(r.Data, d.file.Order) == 1 {
		return nil, fmt.Errorf("unsupported v1 SND bank")
	}
	b := &AudioBank{TrackName: r.nameAt(36), LoopChunks: []int{}, Singles: map[string]BankChunk{}}
	if strings.HasSuffix(strings.ToLower(b.TrackName), ".wav") {
		b.TrackName = b.TrackName[:len(b.TrackName)-4]
	}
	loop, single := r.i32At(28), r.i32At(32)
	if loop > 0 && loop < len(d.file.Containers) {
		chunks, err := ReadLoopChunks(d.file.Data(loop), d.file.Order)
		if err != nil {
			return nil, err
		}
		for _, c := range chunks {
			b.LoopChunks = append(b.LoopChunks, c.ContainerLoc)
		}
	}
	if single > 0 && single < len(d.file.Containers) {
		chunks, err := ReadOneShotChunks(d.file.Data(single), 15, 0, d.file.Order)
		if err != nil {
			return nil, err
		}
		for _, c := range chunks {
			key := strings.ToLower(c.Identifier[strings.LastIndex(c.Identifier, "/")+1:])
			if _, found := b.Singles[key]; !found {
				b.SingleOrder = append(b.SingleOrder, key)
			}
			b.Singles[key] = c
		}
	}
	return b, d.err()
}
