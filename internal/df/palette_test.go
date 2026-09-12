package df

import (
	"encoding/binary"
	"testing"
)

func TestPaletteOrderAndReservedColors(t *testing.T) {
	raw := make([]byte, 24)
	for i := range raw {
		raw[i] = byte(i)
	}
	pc := PaletteRGBA(raw, 3, nil)
	mac := PaletteRGBA(raw, 3, binary.BigEndian)
	if pc[0] != 0 || pc[1] != 0 || pc[2] != 0 || pc[3] != 255 || pc[4] != 11 || pc[5] != 13 || pc[6] != 15 || pc[7] != 255 || pc[1020] != 255 || pc[1023] != 255 {
		t.Fatal("PC palette or reserved colors differ")
	}
	if mac[0] != 2 || mac[1] != 4 || mac[2] != 6 || mac[4] != 10 || mac[5] != 12 || mac[6] != 14 || mac[1023] != 0 {
		t.Fatal("Mac palette component order differs")
	}
}
