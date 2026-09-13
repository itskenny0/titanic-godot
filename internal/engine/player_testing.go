package engine

import (
	"fmt"

	"github.com/itskenny0/titanic-godot/internal/script"
)

type PlayerTestCommand struct {
	Op, Name string
	Value    any
}

// TestOperation exposes the few state controls needed by the Godot integration
// test. It is disabled in ordinary boots and never evaluates source code.
func (p *Player) TestOperation(c PlayerTestCommand) (any, bool, error) {
	if !p.Testing || p.Host == nil {
		return nil, false, fmt.Errorf("runtime testing is disabled")
	}
	s := p.Host.Session
	switch c.Op {
	case "set_global":
		var value script.Value
		switch v := c.Value.(type) {
		case string:
			value = script.Str(v)
		case float64:
			value = script.Num(v)
		case bool:
			value = script.Bool(v)
		default:
			return nil, false, fmt.Errorf("unsupported test value")
		}
		s.Interp.Globals.Set(c.Name, value)
		return nil, false, nil
	case "get_global":
		v, _ := s.Interp.Globals.Get(c.Name)
		if v.IsString {
			return v.Text, false, nil
		}
		return v.Number, false, nil
	case "snapshot":
		b, err := s.SnapshotSave()
		return b, true, err
	case "fail_tick":
		p.testFailTick = true
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("unknown test operation %q", c.Op)
}
