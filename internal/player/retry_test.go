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

// With the fallback source on, a song NetEase fails is asked of it once,
// a greyed-out one only of it, and with it off, it is never asked.
func TestOpenAnyFallsBack(t *testing.T) {
	defer func(f, g func(int64, string) (*netease.Stream, error), d []time.Duration, on bool) {
		songURL, fallbackURL, retryDelays = f, g, d
		netease.Fallback.Store(on)
	}(songURL, fallbackURL, retryDelays, netease.Fallback.Load())
	retryDelays = []time.Duration{0, 0}
	refused := &netease.RefusedError{Code: -460}
	for _, c := range []struct {
		name     string
		on       bool
		track    netease.Track
		netease  error
		asked    int // of NetEase
		fellBack int
		want     error
	}{
		{"off", false, netease.Track{ID: 1}, netease.ErrUnavailable, 2, 0, netease.ErrUnavailable},
		{"unavailable", true, netease.Track{ID: 1}, netease.ErrUnavailable, 2, 1, netease.ErrFallbackUnavailable},
		{"blocked", true, netease.Track{ID: 1}, refused, 1, 1, refused},
		{"greyed", true, netease.Track{ID: 1, Fee: 1}, netease.ErrUnavailable, 0, 1, netease.ErrFallbackUnavailable},
		{"episode", true, netease.Track{ID: 1, ProgramID: 2}, netease.ErrUnavailable, 2, 0, netease.ErrUnavailable},
	} {
		netease.Fallback.Store(c.on)
		asked, fellBack := 0, 0
		songURL = func(int64, string) (*netease.Stream, error) { asked++; return nil, c.netease }
		fallbackURL = func(int64, string) (*netease.Stream, error) {
			fellBack++
			return nil, netease.ErrFallbackUnavailable
		}
		p := &Player{}
		p.latest.Store(1)
		_, _, err := p.openAny(c.track, "exhigh", 1)
		if err != c.want || asked != c.asked || fellBack != c.fellBack {
			t.Errorf("%s: got %v, NetEase asked %d, fallback %d; want %v, %d, %d", c.name, err, asked, fellBack, c.want, c.asked, c.fellBack)
		}
	}
}
