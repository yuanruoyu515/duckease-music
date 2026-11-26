package player

import (
	"sync/atomic"
	"testing"
	"time"
)

// blocking is a stream whose reads wait until it's let go, as one does
// for bytes that haven't come in.
type blocking struct {
	stream
	release chan struct{}
}

func (b blocking) Read(dst []float32) (int, error) { <-b.release; return len(dst), nil }

// A read that waits on its download says the song is loading once it has
// waited starvedAfter, and no more once it returns; before, a seek past
// what had come in only went quiet.
func TestStarvedReadLoads(t *testing.T) {
	var clock atomic.Int64
	clock.Store(time.Unix(1, 0).UnixNano())
	ended := make(chan struct{}, 1)
	b := blocking{release: make(chan struct{})}
	w := &watched{stream: b, now: func() time.Time { return time.Unix(0, clock.Load()) }, ended: func() { ended <- struct{}{} }}
	go w.Read(make([]float32, 8))
	for w.since.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	if w.starved() {
		t.Fatal("loading before it waited")
	}
	clock.Add(int64(starvedAfter))
	if !w.starved() {
		t.Fatal("doesn't say it's loading")
	}
	close(b.release)
	select {
	case <-ended:
	case <-time.After(2 * time.Second):
		t.Fatal("not told the wait ended")
	}
	if w.starved() {
		t.Error("still loading once the read returned")
	}
}
