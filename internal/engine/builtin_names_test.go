package engine

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestEveryReferenceBuiltinIsRegistered(t *testing.T) {
	b, err := os.ReadFile("../../tests/fixtures/builtin-names.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	s := builtinTestSession(t)
	got := []string{}
	for name := range s.Interp.Builtins {
		got = append(got, name)
	}
	for name := range s.Interp.SpecialForms {
		got = append(got, name)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		for _, name := range want {
			if !slices.Contains(got, name) {
				t.Error("missing command", name)
			}
		}
		for _, name := range got {
			if !slices.Contains(want, name) {
				t.Error("unexpected command", name)
			}
		}
	}
}
