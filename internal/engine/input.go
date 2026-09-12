package engine

// EventQueue keeps the original 32-entry input policy. When full, the newest
// queued event is replaced; older clicks retain their order.
const EventCapacity = 32

type QueuedEvent struct {
	Kind, Key string
	Special   bool
	X, Y      float64
}
type EventQueue struct {
	q                      [EventCapacity]QueuedEvent
	n                      int
	Posted, Taken, Dropped uint64
}

func (q *EventQueue) Len() int               { return q.n }
func (q *EventQueue) Pending() []QueuedEvent { return append([]QueuedEvent{}, q.q[:q.n]...) }
func (q *EventQueue) Post(e QueuedEvent, coalesce bool) {
	q.Posted++
	if coalesce {
		out := 0
		for i := 0; i < q.n; i++ {
			old := q.q[i]
			if old.Kind == "keydown" && e.Kind == "keydown" && old.Key == e.Key {
				q.Dropped++
				continue
			}
			q.q[out] = old
			out++
		}
		q.n = out
	}
	if q.n == EventCapacity {
		q.n--
		q.Dropped++
	}
	q.q[q.n] = e
	q.n++
}
func (q *EventQueue) Take() (QueuedEvent, bool) {
	if q.n == 0 {
		return QueuedEvent{}, false
	}
	e := q.q[0]
	copy(q.q[:], q.q[1:q.n])
	q.n--
	q.q[q.n] = QueuedEvent{}
	q.Taken++
	return e, true
}
func (q *EventQueue) Has(kind string) bool {
	for _, e := range q.q[:q.n] {
		if e.Kind == kind {
			return true
		}
	}
	return false
}
func (q *EventQueue) Flush() { q.Dropped += uint64(q.n); clear(q.q[:]); q.n = 0 }
func PackPoint(x, y float64) int32 {
	return int32(uint32(int32JS(x)&65535)<<16 | uint32(int32JS(y)&65535))
}
func PointX(p float64) int16 { return int16(int32JS(p) >> 16) }
func PointY(p float64) int16 { return int16(int32JS(p)) }

// SeededRandom reproduces the pinned engine's deterministic uint32 generator.
func SeededRandom(seed uint32) func() float64 {
	return func() float64 {
		seed += 0x6d2b79f5
		t := seed
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}
