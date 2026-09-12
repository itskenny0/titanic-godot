package script

import (
	"reflect"
	"testing"
)

func TestUTF16StringOperationsPreserveSurrogates(t *testing.T) {
	cases := [][]uint16{{}, {'a', 0xd83d, 0xde00, 'b'}, {0xd83d}, {0xde00}, {0xd800, 0xd800, 0xdc00, 0xdc00}, {0, 127, 255, 0xffff}}
	for _, units := range cases {
		got := UTF16Units(StringFromUTF16(units))
		if !reflect.DeepEqual(got, units) {
			t.Fatalf("round trip %x became %x", units, got)
		}
	}
	text := "A😀B"
	units := UTF16Units(text)
	if len(units) != 4 || StringFromUTF16(units) != text {
		t.Fatal("UTF-16 indexing changed")
	}
}
