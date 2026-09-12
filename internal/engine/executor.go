// Package engine hosts the native Titanic gameplay runtime.
package engine

import (
	"errors"
	"fmt"
)

var ErrCancelled = errors.New("game task cancelled")

type cancelled struct{}
type taskReply struct {
	value any
	err   error
}
type taskRequest struct {
	task   *Task
	ready  func() bool
	effect func() (any, error)
	done   bool
	err    error
}

// Executor runs one script continuation at a time. Its public methods belong to
// the Godot thread. A script runs on a parked Go stack and hands control back at
// each wait or host effect. No two scripts can mutate session state together.
// Host effects execute inside Pump, on its caller's thread, never on a worker.
// The C entry point must retain its OS thread while calling Pump.
type Executor struct {
	requests chan taskRequest
	runnable []*Task
	waiting  []taskRequest
	current  *Task
	tasks    map[*Task]bool
	now      float64
	frame    uint64
	nextID   uint64
	closed   bool
}
type Task struct {
	ID        uint64
	Name      string
	executor  *Executor
	resume    chan taskReply
	reply     taskReply
	cancelled bool
	done      bool
	err       error
}
type Completion struct {
	ID   uint64
	Name string
	Err  error
}

func NewExecutor() *Executor {
	return &Executor{requests: make(chan taskRequest), tasks: map[*Task]bool{}}
}
func (e *Executor) Now() float64 { return e.now }

// AdvanceClock updates deadlines without running continuations during a host tick.
func (e *Executor) AdvanceClock(now float64) {
	if now > e.now {
		e.now = now
	}
}
func (e *Executor) Frame() uint64  { return e.frame }
func (e *Executor) Current() *Task { return e.current }
func (e *Executor) Pending() int   { return len(e.tasks) }
func (t *Task) Done() bool         { return t.done }
func (t *Task) Err() error         { return t.err }
func (e *Executor) Start(name string, run func(*Task) error) *Task {
	if e.closed {
		panic("start on closed executor")
	}
	e.nextID++
	t := &Task{ID: e.nextID, Name: name, executor: e, resume: make(chan taskReply)}
	e.tasks[t] = true
	e.runnable = append(e.runnable, t)
	go func() {
		var err error
		defer func() {
			if v := recover(); v != nil {
				if _, ok := v.(cancelled); ok {
					err = ErrCancelled
				} else {
					err = fmt.Errorf("task %s panicked: %v", name, v)
				}
			}
			e.requests <- taskRequest{task: t, done: true, err: err}
		}()
		if reply := <-t.resume; reply.err != nil {
			panic(cancelled{})
		}
		err = run(t)
	}()
	return t
}
func (t *Task) suspend(request taskRequest) taskReply {
	if t.cancelled {
		panic(cancelled{})
	}
	request.task = t
	t.executor.requests <- request
	reply := <-t.resume
	if reply.err == ErrCancelled {
		panic(cancelled{})
	}
	return reply
}

// Wait suspends even when ready is already true, preserving continuation order.
func (t *Task) Wait(ready func() bool) { t.suspend(taskRequest{ready: ready}) }
func (t *Task) NextFrame() {
	frame := t.executor.frame
	t.Wait(func() bool { return t.executor.frame > frame })
}
func (t *Task) Sleep(ms float64) {
	deadline := t.executor.now + ms
	t.Wait(func() bool { return t.executor.now >= deadline })
}
func (t *Task) Join(other *Task) error {
	if other == t {
		return fmt.Errorf("task cannot await itself")
	}
	t.Wait(other.Done)
	return other.Err()
}
func (t *Task) Effect(fn func() (any, error)) (any, error) {
	reply := t.suspend(taskRequest{effect: fn})
	return reply.value, reply.err
}
func (t *Task) Fork(name string, run func(*Task) error) *Task {
	v, err := t.Effect(func() (any, error) { return t.executor.Start(name, run), nil })
	if err != nil {
		panic(err)
	}
	return v.(*Task)
}
func (e *Executor) finish(request taskRequest) Completion {
	t := request.task
	t.done = true
	t.err = request.err
	delete(e.tasks, t)
	return Completion{t.ID, t.Name, t.err}
}
func (e *Executor) wake() {
	waiting := e.waiting
	e.waiting = nil
	for _, r := range waiting {
		if r.ready == nil || r.ready() {
			e.runnable = append(e.runnable, r.task)
		} else {
			e.waiting = append(e.waiting, r)
		}
	}
}

// Pump services ready continuations without waiting for a future clock tick or
// event. The budget limits immediate yield loops; remaining work is retained.
func (e *Executor) Pump(now float64, advanceFrame bool, budget int) []Completion {
	if e.closed {
		return nil
	}
	e.AdvanceClock(now)
	if advanceFrame {
		e.frame++
	}
	e.wake()
	out := []Completion{}
	if budget <= 0 {
		budget = 10000
	}
	for len(e.runnable) > 0 && budget > 0 {
		t := e.runnable[0]
		e.runnable = e.runnable[1:]
		e.current = t
		t.resume <- t.reply
		t.reply = taskReply{}
		for {
			r := <-e.requests
			budget--
			if r.task != t {
				panic("concurrent game continuation")
			}
			if r.done {
				out = append(out, e.finish(r))
				break
			}
			if r.effect != nil {
				value, err := r.effect()
				if budget <= 0 {
					t.reply = taskReply{value, err}
					e.runnable = append(e.runnable, t)
					break
				}
				t.resume <- taskReply{value, err}
				continue
			}
			e.waiting = append(e.waiting, r)
			break
		}
		e.current = nil
		e.wake()
	}
	return out
}

// CancelAll drains each stack separately, so deferred session cleanup stays
// serialized too. A restart can discard all pending waits without leaking them.
func (e *Executor) CancelAll() []Completion {
	if e.current != nil {
		panic("cancel while a game task is executing")
	}
	out := []Completion{}
	for t := range e.tasks {
		e.current = t
		t.cancelled = true
		t.resume <- taskReply{err: ErrCancelled}
		r := <-e.requests
		if r.task != t || !r.done {
			panic("cancelled game task did not exit")
		}
		out = append(out, e.finish(r))
		e.current = nil
	}
	e.runnable = nil
	e.waiting = nil
	return out
}
func (e *Executor) Close() { e.CancelAll(); e.closed = true }
