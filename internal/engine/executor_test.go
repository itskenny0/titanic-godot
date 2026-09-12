package engine

import (
	"fmt"
	"reflect"
	"testing"
)

func TestExecutorWaitsKeepHostResponsive(t *testing.T) {
	e := NewExecutor()
	defer e.Close()
	events := []string{}
	audioDone := false
	e.Start("conversation", func(task *Task) error {
		events = append(events, "start")
		task.Sleep(50)
		events = append(events, "animation")
		task.Wait(func() bool { return audioDone })
		events = append(events, "audio")
		task.NextFrame()
		events = append(events, "finished")
		return nil
	})
	e.Pump(0, true, 100)
	e.Pump(49, true, 100)
	if !reflect.DeepEqual(events, []string{"start"}) {
		t.Fatal(events)
	}
	e.Pump(50, true, 100)
	if !reflect.DeepEqual(events, []string{"start", "animation"}) {
		t.Fatal(events)
	}
	e.Start("input", func(task *Task) error { audioDone = true; return nil })
	e.Pump(50, false, 100)
	if !reflect.DeepEqual(events, []string{"start", "animation", "audio"}) {
		t.Fatal(events)
	}
	e.Pump(51, true, 100)
	if e.Pending() != 0 || !reflect.DeepEqual(events, []string{"start", "animation", "audio", "finished"}) {
		t.Fatal(events)
	}
}
func TestExecutorEffectsForkAndJoin(t *testing.T) {
	e := NewExecutor()
	defer e.Close()
	events := []string{}
	var child *Task
	parent := e.Start("parent", func(task *Task) error {
		child = task.Fork("child", func(task *Task) error {
			events = append(events, "child")
			task.NextFrame()
			return fmt.Errorf("child failure")
		})
		value, err := task.Effect(func() (any, error) {
			if e.Current() != task {
				t.Error("wrong active effect task")
			}
			events = append(events, "host effect")
			return "reply", nil
		})
		if err != nil || value != "reply" {
			return fmt.Errorf("effect result was lost")
		}
		events = append(events, "join")
		return task.Join(child)
	})
	e.Pump(0, true, 100)
	if parent.Done() || child.Done() {
		t.Fatal("task completed before next frame")
	}
	if !reflect.DeepEqual(events, []string{"host effect", "join", "child"}) {
		t.Fatal(events)
	}
	e.Pump(1, true, 100)
	if !parent.Done() || parent.Err() == nil || parent.Err().Error() != "child failure" {
		t.Fatal("join did not propagate child result")
	}
}
func TestExecutorCancellationRunsDeferredCleanup(t *testing.T) {
	e := NewExecutor()
	cleaned := 0
	for i := 0; i < 20; i++ {
		e.Start("waiting", func(task *Task) error {
			defer func() { cleaned++ }()
			task.Sleep(1000)
			t.Error("cancelled task resumed normally")
			return nil
		})
	}
	e.Pump(0, true, 100)
	pending := e.Start("not started", func(task *Task) error { t.Error("cancelled task started"); return nil })
	done := e.CancelAll()
	if len(done) != 21 || cleaned != 20 || e.Pending() != 0 || !pending.Done() {
		t.Fatalf("incomplete cancellation: %d/%d/%d", len(done), cleaned, e.Pending())
	}
	for _, c := range done {
		if c.Err != ErrCancelled {
			t.Fatal(c)
		}
	}
	e.Start("new game", func(task *Task) error { return nil })
	if done := e.Pump(0, true, 100); len(done) != 1 || done[0].Err != nil {
		t.Fatal(done)
	}
	e.Close()
}
func TestExecutorBudgetRetainsImmediateContinuations(t *testing.T) {
	e := NewExecutor()
	defer e.Close()
	count := 0
	task := e.Start("yield loop", func(task *Task) error {
		for count < 100 {
			count++
			task.Wait(func() bool { return true })
		}
		return nil
	})
	e.Pump(0, false, 5)
	if count != 5 || task.Done() {
		t.Fatalf("budget failed: %d", count)
	}
	for !task.Done() {
		e.Pump(0, false, 5)
	}
	if count != 100 {
		t.Fatal(count)
	}
}

func TestExecutorBudgetPreservesEffectReplies(t *testing.T) {
	e := NewExecutor()
	defer e.Close()
	effects, replies := 0, 0
	task := e.Start("effects", func(task *Task) error {
		for i := 0; i < 20; i++ {
			value, err := task.Effect(func() (any, error) { effects++; return effects, nil })
			if err != nil || value != i+1 {
				return fmt.Errorf("effect reply lost at %d: %v %v", i, value, err)
			}
			replies++
		}
		return nil
	})
	e.Pump(0, false, 3)
	if effects != 3 || replies != 2 {
		t.Fatalf("effect budget: %d/%d", effects, replies)
	}
	for !task.Done() {
		e.Pump(0, false, 3)
	}
	if task.Err() != nil || effects != 20 || replies != 20 {
		t.Fatalf("result: %v %d/%d", task.Err(), effects, replies)
	}
}
