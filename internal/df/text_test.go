package df

import (
	"bytes"
	"testing"
)

func TestMacRomanHostInput(t *testing.T) {
	raw := make([]byte, 256)
	for i := range raw {
		raw[i] = byte(i)
	}
	if got := EncodeMacRoman(DecodeMacRoman(Latin1(raw)), 256); !bytes.Equal(got, raw) {
		t.Fatal("Macintosh encoding does not round trip all bytes")
	}
	if got := EncodeMacRoman("Café😀x", 5); !bytes.Equal(got, []byte{'C', 'a', 'f', 0x8e, '?'}) {
		t.Fatalf("host input encoding: %x", got)
	}
	if len(EncodeMacRoman("x", 0)) != 0 {
		t.Fatal("zero-byte input limit ignored")
	}
}
