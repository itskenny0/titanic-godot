package script

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/itskenny0/titanic-godot/internal/df"
)

func hash(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
	h := sha256.Sum256(bytes.TrimSuffix(b.Bytes(), []byte{'\n'}))
	return hex.EncodeToString(h[:])
}
func exprJSON(e *Expr) any {
	if e == nil {
		return nil
	}
	m := map[string]any{}
	switch e.Kind {
	case "number":
		m["t"] = "int"
		m["v"] = e.Number
	case "string":
		m["t"] = "str"
		m["v"] = e.Text
	case "bool":
		m["t"] = "bool"
		m["v"] = e.Bool
	case "me", "target":
		m["t"] = e.Kind
	case "variable":
		m["t"] = "var"
		m["name"] = e.Text
	case "unary":
		m["t"] = "un"
		m["op"] = e.Text
		m["e"] = exprJSON(e.Left)
	case "binary":
		m["t"] = "bin"
		m["op"] = e.Text
		m["l"] = exprJSON(e.Left)
		m["r"] = exprJSON(e.Right)
	case "call":
		m["t"] = "call"
		m["name"] = e.Text
		if e.Opcode != 0 {
			m["id"] = e.Opcode
		}
		args := make([]any, 0, len(e.Args))
		for _, a := range e.Args {
			args = append(args, exprJSON(a))
		}
		m["args"] = args
	}
	return m
}
func stmtsJSON(stmts []Stmt) any {
	out := make([]any, 0, len(stmts))
	for _, s := range stmts {
		m := map[string]any{"t": s.Kind}
		switch s.Kind {
		case "global", "local", "dumpglobal", "dumplocal":
			m["t"] = "decl"
			m["kind"] = s.Kind
			m["names"] = s.Names
		case "assign":
			m["name"] = s.Name
			m["value"] = exprJSON(s.Value)
		case "call":
			m["t"] = "callstmt"
			m["call"] = exprJSON(s.Value)
		case "return":
			if s.Value != nil {
				m["value"] = exprJSON(s.Value)
			}
		case "if":
			m["cond"] = exprJSON(s.Condition)
			m["then"] = stmtsJSON(s.Body)
			if s.Else != nil {
				m["else_"] = stmtsJSON(s.Else)
			}
		case "while":
			m["cond"] = exprJSON(s.Condition)
			m["body"] = stmtsJSON(s.Body)
		case "for":
			m["varName"] = s.Name
			m["from"] = exprJSON(s.From)
			m["to"] = exprJSON(s.To)
			if s.Step != nil {
				m["step"] = exprJSON(s.Step)
			}
			m["body"] = stmtsJSON(s.Body)
		case "switch":
			m["subject"] = exprJSON(s.Value)
			cases := make([]any, 0, len(s.Cases))
			for _, c := range s.Cases {
				cases = append(cases, map[string]any{"match": exprJSON(c.Match), "body": stmtsJSON(c.Body)})
			}
			m["cases"] = cases
		}
		out = append(out, m)
	}
	return out
}
func scriptJSON(s *Script) any {
	codes := make([]any, 0, len(s.Order))
	for _, key := range s.Order {
		h := s.Handlers[key]
		params := append([]string{}, h.Params...)
		codes = append(codes, []any{key, map[string]any{"name": h.Name, "params": params, "body": stmtsJSON(h.Body)}})
	}
	return map[string]any{"codes": codes, "topLevel": stmtsJSON(s.TopLevel)}
}
func tokensJSON(tokens []df.Token) any {
	out := make([]any, 0, len(tokens))
	for _, t := range tokens {
		m := map[string]any{}
		switch t.Kind {
		case df.TokenString:
			m["kind"] = "str"
			m["value"] = t.Text
		case df.TokenInteger:
			m["kind"] = "int"
			m["value"] = t.Number
		case df.TokenVariable:
			m["kind"] = "var"
			m["name"] = t.Text
		case df.TokenBreak:
			m["kind"] = "break"
			m["indent"] = t.Number
		case df.TokenOpcode:
			m["kind"] = "op"
			m["id"] = t.Opcode
			m["name"] = t.Text
		}
		out = append(out, m)
	}
	return out
}
func TestReferenceScripts(t *testing.T) {
	path := os.Getenv("TAOOT_SCRIPT_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_SCRIPT_REFERENCE to local pinned-reference hashes")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path    string
		Scripts []struct {
			Loc         int
			Tokens, AST string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range corpus {
		t.Run(entry.Path, func(t *testing.T) {
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := df.ReadFile(data)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range entry.Scripts {
				tokens, err := df.DecodeScript(file.Data(want.Loc))
				if err != nil {
					t.Fatalf("container %d: %v", want.Loc, err)
				}
				if hash(tokensJSON(tokens)) != want.Tokens {
					t.Fatalf("container %d token mismatch", want.Loc)
				}
				s, err := Parse(tokens)
				if want.AST == "" {
					if err == nil {
						t.Errorf("container %d accepted invalid reference script", want.Loc)
					}
					continue
				}
				if err != nil {
					t.Fatalf("container %d parse: %v", want.Loc, err)
				}
				if hash(scriptJSON(s)) != want.AST {
					t.Fatalf("container %d AST mismatch", want.Loc)
				}
				count++
			}
		})
	}
	t.Logf("Compared %d scripts", count)
}
