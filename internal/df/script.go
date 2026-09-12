package df

import (
	"encoding/binary"
	"fmt"
)

type TokenKind uint8

const (
	TokenEnd TokenKind = iota
	TokenString
	TokenInteger
	TokenVariable
	TokenBreak
	TokenOpcode
)

type Token struct {
	Kind   TokenKind
	Text   string
	Number uint32
	Opcode uint16
}

func DecodeScript(data []byte) ([]Token, error) {
	r := NewReader(data, binary.LittleEndian)
	var tokens []Token
	for {
		pos := r.Pos
		cmd, info := r.U16(), r.U32()
		r.Skip(2)
		if r.Err != nil {
			return nil, fmt.Errorf("script: %w", r.Err)
		}
		if cmd == 0 {
			return tokens, nil
		}
		t := Token{Number: info}
		switch cmd {
		case 3, 5:
			at := uint64(pos) + uint64(info)
			if at >= uint64(len(data)) {
				return nil, fmt.Errorf("script: text offset out of bounds")
			}
			text := NewReader(data, binary.LittleEndian)
			text.Seek(int(at))
			t.Text = text.PString()
			if text.Err != nil {
				return nil, text.Err
			}
			if cmd == 3 {
				t.Kind = TokenString
			} else {
				t.Kind = TokenVariable
			}
		case 4:
			t.Kind = TokenInteger
		case 6:
			t.Kind = TokenBreak
		default:
			name, ok := Opcodes[cmd]
			if !ok {
				return nil, fmt.Errorf("script: unknown opcode %d", cmd)
			}
			t.Kind = TokenOpcode
			t.Opcode = cmd
			t.Text = name
		}
		tokens = append(tokens, t)
	}
}
