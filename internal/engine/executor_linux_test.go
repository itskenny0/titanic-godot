package engine

import (
	"runtime"
	"syscall"
	"testing"
)

func TestHostEffectsRemainOnCallingThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	caller := syscall.Gettid()
	e := NewExecutor()
	defer e.Close()
	worker, effect := 0, 0
	task := e.Start("native callback", func(task *Task) error {
		worker = syscall.Gettid()
		_, err := task.Effect(func() (any, error) { effect = syscall.Gettid(); return nil, nil })
		return err
	})
	e.Pump(0, false, 10)
	if !task.Done() || task.Err() != nil || effect != caller || worker == caller {
		t.Fatalf("thread ownership: caller=%d worker=%d effect=%d error=%v", caller, worker, effect, task.Err())
	}
}
