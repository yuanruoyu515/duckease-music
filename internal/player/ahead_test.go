package player

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

// aheadQueue is a player on the first of tracks with ids, and NetEase
// answering each as the same song on a test server, counting what it's
// asked for.
func aheadQueue(t *testing.T, ids ...int64) (*Player, func(int64) int) {
	t.Helper()
	b, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	t.Cleanup(srv.Close)
	var mu sync.Mutex
	asked := map[int64]int{}
	old := songURL
	t.Cleanup(func() { songURL = old })
	songURL = func(id int64, level string) (*netease.Stream, error) {
		mu.Lock()
		asked[id]++
		mu.Unlock()
		return &netease.Stream{URL: srv.URL, Size: int64(len(b)), Type: "mp3", Level: level}, nil
	}
	p := &Player{at: 0}
	for i, id := range ids {
		p.queue = append(p.queue, netease.Track{ID: id, Duration: 5000})
		p.order = append(p.order, i)
	}
	t.Cleanup(p.dropAhead)
	return p, func(id int64) int { mu.Lock(); defer mu.Unlock(); return asked[id] }
}

// The next song is asked for ahead once, and then opens from what came in
// without NetEase being asked for it again.
func TestNextSongComesInAhead(t *testing.T) {
	p, asked := aheadQueue(t, 1, 2)
	p.getAhead("exhigh")
	p.getAhead("exhigh")
	pre := p.takeAhead(p.queue[1], "exhigh")
	if pre == nil {
		t.Fatal("nothing asked for ahead")
	}
	info, s, err := openAhead(pre, p.queue[1])
	if err != nil || s == nil || info == nil {
		t.Fatalf("opened %v %v %v", info, s, err)
	}
	defer s.Close()
	if n := asked(2); n != 1 {
		t.Errorf("asked NetEase for the next song %d times", n)
	}
	if p.takeAhead(p.queue[1], "exhigh") != nil {
		t.Error("taken twice")
	}
}

// Nothing is asked for ahead after the last song unless the queue repeats,
// nor while one song repeats, nor for a song NetEase greys out, nor while
// songs are not asked for after its risk control turned one away: one more
// request then would only make it last longer.
func TestAheadAsksOnlyWhenItShould(t *testing.T) {
	p, asked := aheadQueue(t, 1, 2)
	p.at = 1
	p.getAhead("exhigh")
	if p.ahead != nil {
		t.Error("asked for a song after the last")
	}
	p.repeat = RepeatAll
	p.getAhead("exhigh")
	if p.ahead == nil || p.ahead.id != 1 {
		t.Errorf("repeating, has %+v ahead", p.ahead)
	}
	p.repeat = RepeatOne
	p.getAhead("exhigh")
	if p.ahead != nil {
		t.Error("asked ahead while one song repeats")
	}

	p.repeat, p.at = RepeatOff, 0
	p.queue[1].Fee = 1 // VIP, greyed out without a privilege
	p.getAhead("exhigh")
	if p.ahead != nil {
		t.Error("asked ahead for a greyed-out song")
	}
	p.queue[1].Fee = 0
	p.blockedUntil.Store(time.Now().Add(time.Minute).UnixNano())
	p.getAhead("exhigh")
	if p.ahead != nil || asked(2) != 0 {
		t.Error("asked ahead within the -460 cooldown")
	}
}

// Risk control turning away the song asked for ahead starts the cooldown,
// as it does for one asked for to play, and the song is asked for again
// when its turn comes.
func TestAheadBlockedCoolsDown(t *testing.T) {
	p, _ := aheadQueue(t, 1, 2)
	songURL = func(int64, string) (*netease.Stream, error) {
		return nil, &netease.RefusedError{Code: -460}
	}
	p.getAhead("exhigh")
	pre := p.takeAhead(p.queue[1], "exhigh")
	if _, s, _ := openAhead(pre, p.queue[1]); s != nil {
		t.Fatal("opened what was turned away")
	}
	if time.Now().UnixNano() >= p.blockedUntil.Load() {
		t.Error("no cooldown after -460 ahead")
	}
}
