package netease

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The fallback source: GD音乐台 (music.gdstudio.xyz), a free public API by
// GD Studio that serves NetEase's own CDN addresses for songs this app
// cannot get from NetEase, as one its listener may not play. The site
// asks those who use it to credit it as "GD音乐台(music.gdstudio.xyz)",
// which the settings and NOTICE.md do, and allows 50 requests in five
// minutes: the app asks it only when a song fails or is greyed out, one
// try each, and keeps well below that.

// FallbackCredit is how GD Studio asks to be credited.
const FallbackCredit = "GD音乐台(music.gdstudio.xyz)"

const fallbackAPI = "https://music-api.gdstudio.xyz/api.php"

// Fallback, set from the settings, lets songs NetEase does not stream
// play from the fallback source: those that fail to load, and those it
// greys out, which Playable then takes for playable.
var Fallback atomic.Bool

// ErrFallbackUnavailable is the fallback source having no stream either.
var ErrFallbackUnavailable = errors.New("fallback source has no stream for this song")

// errFallbackBusy is a request not sent, to stay within the site's limit.
var errFallbackBusy = errors.New("fallback source: too many requests, try again later")

// fallbackLimit is how many requests go to the fallback source within
// fallbackWindow: below the 50 in five minutes the site allows.
const (
	fallbackLimit  = 40
	fallbackWindow = 5 * time.Minute
)

var fallbackSent struct {
	mu    sync.Mutex
	times []time.Time
}

// fallbackAllow reports whether a request may go now, and counts it.
func fallbackAllow(now time.Time) bool {
	s := &fallbackSent
	s.mu.Lock()
	defer s.mu.Unlock()
	keep := s.times[:0]
	for _, t := range s.times {
		if now.Sub(t) < fallbackWindow {
			keep = append(keep, t)
		}
	}
	s.times = keep
	if len(s.times) >= fallbackLimit {
		return false
	}
	s.times = append(s.times, now)
	return true
}

// FallbackUsage reports how many requests went to the fallback source
// within the last fallbackWindow, out of limit, and how long until the
// oldest of them no longer counts: 0 with none.
func FallbackUsage(now time.Time) (used, limit int, freeIn time.Duration) {
	s := &fallbackSent
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.times {
		if left := fallbackWindow - now.Sub(t); left > 0 {
			if used == 0 || left < freeIn {
				freeIn = left
			}
			used++
		}
	}
	return used, fallbackLimit, freeIn
}

// FallbackWindow is the span FallbackUsage counts requests over.
const FallbackWindow = fallbackWindow

// fallbackBitrates are the bit rates the site names the levels by.
var fallbackBitrates = map[string]string{
	"standard": "128",
	"higher":   "192",
	"exhigh":   "320",
	"lossless": "740",
	"hires":    "999",
}

// FallbackURL returns where to stream a track from the fallback source,
// at level or below it, as SongURL does from NetEase. The address expires
// after a while, so it is asked for each time a song loads, not kept.
func FallbackURL(id int64, level string) (*Stream, error) {
	if !fallbackAllow(time.Now()) {
		return nil, errFallbackBusy
	}
	br, ok := fallbackBitrates[level]
	if !ok {
		br = "320"
	}
	q := url.Values{"types": {"url"}, "source": {"netease"}, "id": {strconv.FormatInt(id, 10)}, "br": {br}}
	req, err := http.NewRequest("GET", fallbackAPI+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("fallback source: " + res.Status)
	}
	var r struct {
		URL  string `json:"url"`
		Br   int    `json:"br"`
		Size int64  `json:"size"` // in bytes, whatever its documentation says
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}
	return fallbackStream(r.URL, r.Br, r.Size, level)
}

// fallbackStream makes the stream the fallback source answered, of a
// level asked for, or says it has none.
func fallbackStream(u string, br int, size int64, level string) (*Stream, error) {
	if u == "" || br == 0 {
		return nil, ErrFallbackUnavailable
	}
	s := &Stream{URL: strings.Replace(u, "http://", "https://", 1), Size: size, Fallback: true}
	path := u
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	switch {
	case strings.HasSuffix(strings.ToLower(path), ".flac"):
		s.Type = "flac"
	case strings.HasSuffix(strings.ToLower(path), ".mp3"):
		s.Type = "mp3"
	default:
		return nil, errors.New("fallback source: unsupported format")
	}
	// The level names the song's file in the cache: what it served.
	switch {
	case s.Type == "flac" && Lossless(level):
		s.Level = level
	case s.Type == "flac":
		s.Level = "lossless"
	case br >= 320:
		s.Level = "exhigh"
	case br >= 192:
		s.Level = "higher"
	default:
		s.Level = "standard"
	}
	return s, nil
}
