package engine

import (
	"regexp"
	"slices"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/df"
	"github.com/itskenny0/titanic-godot/internal/script"
)

type BootPlan struct {
	Resources, Casts []string
	LandingSet       *string
	Volumes          []string
}

var extensionPattern = regexp.MustCompile(`(?i)\.[a-z0-9]+$`)

func setFileName(name string) string {
	if extensionPattern.MatchString(name) {
		return name
	}
	return name + ".set"
}
func walkExpr(e *script.Expr, call func(*script.Expr)) {
	if e == nil {
		return
	}
	switch e.Kind {
	case "call":
		call(e)
		for _, a := range e.Args {
			walkExpr(a, call)
		}
	case "binary":
		walkExpr(e.Left, call)
		walkExpr(e.Right, call)
	case "unary":
		walkExpr(e.Left, call)
	}
}
func walkStatements(body []script.Stmt, call func(*script.Expr)) {
	for _, s := range body {
		switch s.Kind {
		case "assign", "call", "return":
			walkExpr(s.Value, call)
		case "if", "while":
			walkExpr(s.Condition, call)
			walkStatements(s.Body, call)
			walkStatements(s.Else, call)
		case "switch":
			walkExpr(s.Value, call)
			for _, c := range s.Cases {
				walkExpr(c.Match, call)
				walkStatements(c.Body, call)
			}
		case "for":
			walkExpr(s.From, call)
			walkExpr(s.To, call)
			walkExpr(s.Step, call)
			walkStatements(s.Body, call)
		}
	}
}
func literalArg(call *script.Expr) string {
	if len(call.Args) > 0 && call.Args[0].Kind == "string" {
		return call.Args[0].Text
	}
	return ""
}
func ReadBootPlan(data []byte) BootPlan {
	out := BootPlan{Resources: []string{}, Casts: []string{}, Volumes: []string{}}
	file, err := df.ReadFile(data)
	if err != nil {
		return out
	}
	codes := map[string][]*script.Handler{}
	for loc := 1; loc < len(file.Containers); loc++ {
		tokens, err := df.DecodeScript(file.Data(loc))
		if err != nil || len(tokens) == 0 {
			continue
		}
		s, err := script.Parse(tokens)
		if err != nil {
			continue
		}
		for _, name := range s.Order {
			h := s.Handlers[name]
			key := strings.ToLower(name)
			codes[key] = append(codes[key], h)
		}
	}
	add := func(into *[]string, file string) {
		key := strings.ToLower(file)
		if !slices.Contains(*into, key) {
			*into = append(*into, key)
		}
	}
	visited := map[string]bool{"advanceday": true, "advancetour": true}
	var visit func(string)
	visit = func(handler string) {
		key := strings.ToLower(handler)
		if visited[key] {
			return
		}
		visited[key] = true
		for _, h := range codes[key] {
			walkStatements(h.Body, func(call *script.Expr) {
				name := strings.ToLower(call.Text)
				file := literalArg(call)
				switch name {
				case "opencastfile", "openshopfile", "openstagefile", "opentrackfile", "opensetfile", "playmovie", "playtheme", "playnewtheme":
					if file != "" {
						if name == "opensetfile" {
							file = setFileName(file)
						}
						add(&out.Resources, file)
						if name == "opencastfile" {
							add(&out.Casts, file)
						}
					}
					return
				}
				if call.Opcode == 0 {
					visit(name)
				}
			})
		}
	}
	visit("boot")
	for _, handler := range []string{"advanceday", "advancetour"} {
		if out.LandingSet != nil {
			break
		}
		for _, h := range codes[handler] {
			walkStatements(h.Body, func(call *script.Expr) {
				if out.LandingSet != nil {
					return
				}
				switch strings.ToLower(call.Text) {
				case "initall", "changeset", "opensetfile", "gotospecial":
					if room := literalArg(call); room != "" {
						name := strings.ToLower(setFileName(room))
						out.LandingSet = &name
					}
				}
			})
		}
	}
	for _, h := range codes["setpath"] {
		walkStatements(h.Body, func(call *script.Expr) {
			if strings.ToLower(call.Text) == "currentcd" {
				if volume := literalArg(call); volume != "" {
					add(&out.Volumes, volume)
				}
			}
		})
	}
	return out
}
