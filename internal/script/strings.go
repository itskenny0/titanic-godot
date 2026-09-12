package script

import (
	"unicode/utf16"
	"unicode/utf8"
)

// UTF16Units retains isolated surrogate code units produced by legacy string
// operations. Internally those use WTF-8; ordinary host text remains UTF-8.
func UTF16Units(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for len(s) > 0 {
		if len(s) >= 3 && s[0] == 0xed && s[1] >= 0xa0 && s[1] <= 0xbf && s[2]&0xc0 == 0x80 {
			out = append(out, uint16(s[0]&15)<<12|uint16(s[1]&63)<<6|uint16(s[2]&63))
			s = s[3:]
			continue
		}
		r, n := utf8.DecodeRuneInString(s)
		s = s[n:]
		if r <= 0xffff {
			out = append(out, uint16(r))
		} else {
			a, b := utf16.EncodeRune(r)
			out = append(out, uint16(a), uint16(b))
		}
	}
	return out
}
func StringFromUTF16(units []uint16) string {
	out := make([]byte, 0, len(units))
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xd800 && u <= 0xdbff && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] <= 0xdfff {
			out = utf8.AppendRune(out, utf16.DecodeRune(rune(u), rune(units[i+1])))
			i++
		} else if u >= 0xd800 && u <= 0xdfff {
			out = append(out, 0xe0|byte(u>>12), 0x80|byte(u>>6&63), 0x80|byte(u&63))
		} else {
			out = utf8.AppendRune(out, rune(u))
		}
	}
	return string(out)
}
