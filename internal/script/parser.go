package script

import (
	"fmt"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
)

type parseError struct{ message string }

func (e parseError) Error() string { return e.message }

type parser struct {
	tokens     []df.Token
	pos, depth int
}

func (p *parser) fail(message string) {
	panic(parseError{fmt.Sprintf("script token %d: %s", p.pos, message)})
}
func (p *parser) peek() df.Token {
	if p.pos >= len(p.tokens) {
		return df.Token{}
	}
	return p.tokens[p.pos]
}
func (p *parser) next() df.Token {
	t := p.peek()
	if t.Kind == df.TokenEnd {
		p.fail("unexpected end")
	}
	p.pos++
	return t
}
func (p *parser) at(op string) bool { t := p.peek(); return t.Kind == df.TokenOpcode && t.Text == op }
func (p *parser) take(op string) bool {
	if p.at(op) {
		p.pos++
		return true
	}
	return false
}
func (p *parser) expect(op string) {
	if !p.take(op) {
		p.fail("expected " + op)
	}
}
func (p *parser) breaks() {
	for p.peek().Kind == df.TokenBreak {
		p.pos++
	}
}
func (p *parser) boundary() bool {
	return p.at("code") || p.at("endcode") || p.at("case") || p.at("endswitch")
}
func (p *parser) closer(op string) {
	if p.peek().Kind == df.TokenEnd || p.at("code") || p.at("endcode") || (op != "endswitch" && (p.at("case") || p.at("endswitch"))) {
		return
	}
	p.expect(op)
}
func (p *parser) emptyArgs() {
	if p.at("(") && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Kind == df.TokenOpcode && p.tokens[p.pos+1].Text == ")" {
		p.pos += 2
	}
}
func (p *parser) line() {
	for p.peek().Kind != df.TokenEnd && p.peek().Kind != df.TokenBreak {
		p.pos++
	}
}
func (p *parser) name() string {
	t := p.next()
	if t.Kind != df.TokenVariable {
		p.fail("expected variable name")
	}
	return t.Text
}

func Parse(tokens []df.Token) (result *Script, err error) {
	defer func() {
		if e := recover(); e != nil {
			if failure, ok := e.(parseError); ok {
				result = nil
				err = failure
			} else {
				panic(e)
			}
		}
	}()
	p := &parser{}
	hasHandlers := false
	for _, t := range tokens {
		if t.Kind == df.TokenOpcode && t.Opcode == 1 {
			continue
		}
		p.tokens = append(p.tokens, t)
		if t.Kind == df.TokenOpcode && t.Text == "code" {
			hasHandlers = true
		}
	}
	result = &Script{Handlers: make(map[string]*Handler)}
	p.breaks()
	for p.peek().Kind != df.TokenEnd {
		if p.take("code") {
			h := &Handler{Name: p.name()}
			p.expect("(")
			for !p.take(")") {
				h.Params = append(h.Params, p.name())
				p.take(",")
			}
			h.Body = p.block("endcode", "code")
			p.take("endcode")
			key := strings.ToLower(h.Name)
			if _, ok := result.Handlers[key]; !ok {
				result.Order = append(result.Order, key)
			}
			result.Handlers[key] = h
		} else {
			at := p.pos
			stmt, e := p.topStatement()
			if e != nil {
				if !hasHandlers {
					return nil, e
				}
				p.pos = at
				for p.peek().Kind != df.TokenEnd && !p.at("code") {
					p.pos++
				}
			} else {
				result.TopLevel = append(result.TopLevel, stmt)
			}
		}
		p.breaks()
	}
	return result, nil
}
func (p *parser) topStatement() (stmt Stmt, err error) {
	defer func() {
		if e := recover(); e != nil {
			if failure, ok := e.(parseError); ok {
				err = failure
			} else {
				panic(e)
			}
		}
	}()
	return p.statement(), nil
}
func (p *parser) block(closers ...string) []Stmt {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 512 {
		p.fail("nesting exceeds limit")
	}
	body := []Stmt{}
	p.breaks()
	for p.peek().Kind != df.TokenEnd && !p.boundary() {
		for _, op := range closers {
			if p.at(op) {
				return body
			}
		}
		body = append(body, p.statement())
		p.breaks()
	}
	return body
}
func (p *parser) statement() Stmt {
	t := p.peek()
	if t.Kind == df.TokenOpcode {
		switch t.Text {
		case "global", "local", "dumpglobal", "dumplocal":
			p.pos++
			s := Stmt{Kind: t.Text, Names: []string{p.name()}}
			for p.take(",") {
				s.Names = append(s.Names, p.name())
			}
			return s
		case "exitcode", "passcode":
			p.pos++
			p.emptyArgs()
			return Stmt{Kind: t.Text}
		case "return":
			p.pos++
			s := Stmt{Kind: "return"}
			if p.peek().Kind != df.TokenEnd && p.peek().Kind != df.TokenBreak {
				s.Value = p.expr(1)
			}
			return s
		case "if":
			p.pos++
			s := Stmt{Kind: "if", Condition: p.expr(1)}
			s.Body = p.block("else", "endif")
			if p.take("else") {
				s.Else = p.block("endif")
			}
			p.closer("endif")
			return s
		case "switch":
			p.pos++
			s := Stmt{Kind: "switch", Value: p.expr(1)}
			p.block("case", "endswitch")
			for p.take("case") {
				c := Case{Match: p.expr(1)}
				c.Body = p.block("case", "endswitch")
				s.Cases = append(s.Cases, c)
			}
			p.closer("endswitch")
			return s
		case "while":
			p.pos++
			s := Stmt{Kind: "while", Condition: p.expr(1)}
			s.Body = p.block("endwhile")
			p.expect("endwhile")
			return s
		case "for":
			p.pos++
			s := Stmt{Kind: "for", Name: p.name()}
			p.expect("=")
			s.From = p.expr(1)
			p.expect("to")
			s.To = p.expr(1)
			if p.take("step") {
				s.Step = p.expr(1)
			}
			s.Body = p.block("endfor")
			p.expect("endfor")
			return s
		case "/":
			p.line()
			return Stmt{Kind: "noop"}
		}
		e := p.expr(1)
		if e.Kind != "call" {
			p.fail("expected command")
		}
		return Stmt{Kind: "call", Value: e}
	}
	if t.Kind == df.TokenVariable {
		p.pos++
		if p.take("=") {
			return Stmt{Kind: "assign", Name: t.Text, Value: p.expr(1)}
		}
		p.pos--
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Text == "(" {
			e := p.expr(1)
			if e.Kind != "call" {
				p.fail("expected call")
			}
			return Stmt{Kind: "call", Value: e}
		}
		p.line()
		return Stmt{Kind: "noop"}
	}
	p.fail("unexpected statement")
	return Stmt{}
}

var precedence = map[string]int{"|": 1, "&": 2, "=": 3, "!=": 3, ">": 3, "<": 3, ">=": 3, "<=": 3, "@": 4, "+": 5, "-": 5, "*": 6, "/": 6}

func (p *parser) expr(min int) *Expr {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 512 {
		p.fail("expression nesting exceeds limit")
	}
	left := p.unary()
	for {
		t := p.peek()
		prec := precedence[t.Text]
		if t.Kind != df.TokenOpcode || prec < min {
			break
		}
		p.pos++
		left = &Expr{Kind: "binary", Text: t.Text, Left: left, Right: p.expr(prec + 1)}
	}
	return left
}
func (p *parser) unary() *Expr {
	if p.at("not") || p.at("-") {
		t := p.next()
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > 512 {
			p.fail("unary nesting exceeds limit")
		}
		return &Expr{Kind: "unary", Text: t.Text, Left: p.unary()}
	}
	t := p.next()
	switch t.Kind {
	case df.TokenInteger:
		return &Expr{Kind: "number", Number: float64(t.Number)}
	case df.TokenString:
		return &Expr{Kind: "string", Text: t.Text}
	case df.TokenVariable:
		if p.at("(") {
			return p.call(t)
		}
		return &Expr{Kind: "variable", Text: t.Text}
	case df.TokenOpcode:
		switch t.Text {
		case "true", "false":
			return &Expr{Kind: "bool", Bool: t.Text == "true"}
		case "me", "target":
			return &Expr{Kind: t.Text}
		case "(":
			e := p.expr(1)
			p.expect(")")
			return e
		default:
			if t.Opcode < 8000 || t.Opcode >= 9000 {
				if p.at("(") {
					return p.call(t)
				}
				return &Expr{Kind: "call", Opcode: t.Opcode, Text: t.Text}
			}
		}
	}
	p.fail("unexpected expression")
	return nil
}
func (p *parser) call(t df.Token) *Expr {
	p.expect("(")
	e := &Expr{Kind: "call", Text: t.Text, Opcode: t.Opcode}
	for !p.take(")") {
		e.Args = append(e.Args, p.expr(1))
		p.take(",")
	}
	return e
}
