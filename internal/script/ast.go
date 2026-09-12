// Package script implements Titanic's compiled DreamFactory language in Go.
// Grammar and semantics follow the pinned dreamREfactory interpreter.
package script

type Expr struct {
	Kind        string
	Number      float64
	Text        string
	Bool        bool
	Opcode      uint16
	Left, Right *Expr
	Args        []*Expr
}
type Case struct {
	Match *Expr
	Body  []Stmt
}
type Stmt struct {
	Kind, Name                       string
	Names                            []string
	Value, Condition, From, To, Step *Expr
	Body, Else                       []Stmt
	Cases                            []Case
}
type Handler struct {
	Name   string
	Params []string
	Body   []Stmt
}
type Script struct {
	Handlers map[string]*Handler
	Order    []string
	TopLevel []Stmt
}
