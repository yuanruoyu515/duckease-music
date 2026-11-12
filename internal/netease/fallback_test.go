package netease

import (
	"testing"
	"time"
)

// The fallback source's answer becomes a stream of its format and level,
// or none for an empty one.
func TestFallbackStream(t *testing.T) {
	for _, c := range []struct {
		url, level     string
		br             int
		typ, wantLevel string
		unavailable    bool
	}{
		{"http://m801.music.126.net/a/b.mp3", "exhigh", 320, "mp3", "exhigh", false},
		{"https://m801.music.126.net/a/b.mp3?x=1", "hires", 128, "mp3", "standard", false},
		{"https://m801.music.126.net/a/b.flac", "hires", 1607, "flac", "hires", false},
		{"https://m801.music.126.net/a/b.flac", "exhigh", 1607, "flac", "lossless", false},
		{"", "exhigh", 0, "", "", true},
	} {
		s, err := fallbackStream(c.url, c.br, 1, c.level)
		if c.unavailable {
			if err != ErrFallbackUnavailable {
				t.Errorf("%q: got %v, want unavailable", c.url, err)
			}
			continue
		}
		if err != nil || s.Type != c.typ || s.Level != c.wantLevel || s.URL[:8] != "https://" || !s.Fallback {
			t.Errorf("%q at %s: got %+v, %v", c.url, c.level, s, err)
		}
	}
}

// No more than fallbackLimit requests go within fallbackWindow.
func TestFallbackLimit(t *testing.T) {
	fallbackSent.times = nil
	defer func() { fallbackSent.times = nil }()
	now := time.Unix(1000, 0)
	for i := range fallbackLimit {
		if !fallbackAllow(now) {
			t.Fatalf("request %d turned away", i+1)
		}
	}
	if fallbackAllow(now.Add(time.Minute)) {
		t.Fatal("one over the limit allowed")
	}
	if !fallbackAllow(now.Add(fallbackWindow)) {
		t.Fatal("not allowed once the window passed")
	}
}

// Usage counts the requests within the window, and says when the oldest
// of them stops counting.
func TestFallbackUsage(t *testing.T) {
	fallbackSent.times = nil
	defer func() { fallbackSent.times = nil }()
	now := time.Unix(1000, 0)
	fallbackAllow(now)
	fallbackAllow(now.Add(time.Minute))
	used, limit, freeIn := FallbackUsage(now.Add(2 * time.Minute))
	if used != 2 || limit != fallbackLimit || freeIn != 3*time.Minute {
		t.Fatalf("got %d/%d, free in %v", used, limit, freeIn)
	}
	used, _, freeIn = FallbackUsage(now.Add(fallbackWindow))
	if used != 1 || freeIn != time.Minute {
		t.Fatalf("after the first expired: got %d, free in %v", used, freeIn)
	}
	if used, _, freeIn = FallbackUsage(now.Add(time.Hour)); used != 0 || freeIn != 0 {
		t.Fatalf("an hour on: got %d, free in %v", used, freeIn)
	}
}
