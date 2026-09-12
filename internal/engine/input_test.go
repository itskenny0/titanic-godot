package engine

import "testing"

func TestInputQueueOrdering(t *testing.T) {
	q := new(EventQueue)
	q.Post(QueuedEvent{Kind: "keydown", Key: "left"}, false)
	q.Post(QueuedEvent{Kind: "mousedown", X: 12, Y: 34}, false)
	q.Post(QueuedEvent{Kind: "keydown", Key: "right"}, false)
	q.Post(QueuedEvent{Kind: "keydown", Key: "left", Special: true}, true)
	first, _ := q.Take()
	if first.Kind != "mousedown" || q.Dropped != 1 || q.Posted != 4 {
		t.Fatal("coalescing disturbed click order")
	}
	second, _ := q.Take()
	third, _ := q.Take()
	if second.Key != "right" || third.Key != "left" || !third.Special {
		t.Fatal("repeat was not placed at the end")
	}
	for i := 0; i < 33; i++ {
		q.Post(QueuedEvent{Kind: "mousedown", X: float64(i)}, true)
	}
	if q.Len() != 32 || q.Dropped != 2 {
		t.Fatal("clicks were coalesced or queue overflow differs")
	}
	for i := 0; i < 31; i++ {
		e, _ := q.Take()
		if e.X != float64(i) {
			t.Fatal("overflow discarded an older event")
		}
	}
	last, _ := q.Take()
	if last.X != 32 {
		t.Fatal("overflow did not replace the newest queued event")
	}
	q.Post(QueuedEvent{Kind: "keydown", Key: "x"}, false)
	q.Flush()
	if q.Len() != 0 || q.Has("keydown") || q.Dropped != 3 {
		t.Fatal("flush accounting differs")
	}
}
func TestPackedPointerCoordinates(t *testing.T) {
	for _, v := range [][2]float64{{-1, -2}, {32767, -32768}, {65537, 65538}, {1.9, -2.9}} {
		p := PackPoint(v[0], v[1])
		if PointX(float64(p)) != int16(int32JS(v[0])) || PointY(float64(p)) != int16(int32JS(v[1])) {
			t.Fatal("signed point round trip failed", v, p)
		}
	}
}
func TestSeededRandom(t *testing.T) {
	random := SeededRandom(12345)
	want := []float64{0.9797282677609473, 0.3067522644996643, 0.484205421525985, 0.817934412509203, 0.5094283693470061}
	for _, v := range want {
		if got := random(); got != v {
			t.Fatal("random sequence differs", got, v)
		}
	}
}
