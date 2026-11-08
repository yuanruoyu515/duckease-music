package player

import (
	"errors"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

// A track that fails to open is tried twice more, a song NetEase says can't
// be streamed once more, and none more once another track is loading.
func TestOpenRetryingTries(t *testing.T) {
	defer func(f func(int64, string) (*netease.Stream, error), d []time.Duration) {
		songURL, retryDelays = f, d
	}(songURL, retryDelays)
	retryDelays = []time.Duration{0, 0}
	for _, c := range []struct {
		err   error
		stale bool
		want  int
	}{
		{errors.New("song url: code -460"), false, 3},
		{netease.ErrUnavailable, false, 2},
		{errors.New("503 Service Unavailable"), true, 1},
	} {
		p := &Player{}
		p.latest.Store(1)
		if c.stale {
			p.latest.Store(2)
		}
		tries := 0
		songURL = func(int64, string) (*netease.Stream, error) { tries++; return nil, c.err }
		if _, _, err := p.openRetrying(netease.Track{ID: 1}, "exhigh", 1); err != c.err {
			t.Errorf("%v: got %v", c.err, err)
		}
		if tries != c.want {
			t.Errorf("%v (stale %v): tried %d times, want %d", c.err, c.stale, tries, c.want)
		}
	}
}

// NetEase's risk control (-460) is asked once, not again, and songs not in
// the cache are not asked for until the cooldown ends.
func TestBlockedCoolsDown(t *testing.T) {
	defer func(f func(int64, string) (*netease.Stream, error), d []time.Duration) {
		songURL, retryDelays = f, d
	}(songURL, retryDelays)
	retryDelays = []time.Duration{0, 0}
	tries := 0
	songURL = func(int64, string) (*netease.Stream, error) {
		tries++
		return nil, &netease.RefusedError{Code: -460, Message: "Cheating"}
	}
	p := &Player{}
	p.latest.Store(1)
	if _, _, err := p.openRetrying(netease.Track{ID: 1}, "exhigh", 1); !blocked(err) {
		t.Fatalf("got %v, want the block", err)
	}
	if _, _, err := p.openRetrying(netease.Track{ID: 2}, "exhigh", 1); err != errCooling {
		t.Fatalf("within the cooldown: got %v", err)
	}
	if tries != 1 {
		t.Fatalf("asked NetEase %d times, want 1", tries)
	}
	p.blockedUntil.Store(time.Now().Add(-time.Second).UnixNano())
	p.openRetrying(netease.Track{ID: 2}, "exhigh", 1)
	if tries != 2 {
		t.Fatalf("after the cooldown: asked %d times, want 2", tries)
	}
}

// Clicking the track that is loading again starts no other load of it.
func TestLoadingTrackChosenAgain(t *testing.T) {
	tr := netease.Track{ID: 7, Name: "ROLY-POLY"}
	p := &Player{queue: []netease.Track{{ID: 3}, tr}, order: []int{0, 1}, at: 1, track: &tr, loading: true, gen: 4}
	p.load(1)
	if p.gen != 4 || !p.playing {
		t.Fatalf("gen %d, playing %v: want the same load, playing", p.gen, p.playing)
	}
}
