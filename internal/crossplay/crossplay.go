// Package crossplay carries playlists between Spotify and NetEase: it
// finds each song of one on the other, remembers what it found, and keeps
// pairs of playlists in step.
package crossplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/match"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

// The searches of either service; tests, here and of the packages using
// crossplay, replace them.
var (
	SearchNetease = netease.Search
	searchSpotify = spotify.Search
)

// Pauses between searches, so that neither service sees a burst: NetEase
// turns those away (-460), and Spotify counts a Development Mode
// account's calls against one quota.
var (
	neteasePause = 800 * time.Millisecond
	spotifyPause = 150 * time.Millisecond
)

// retryAfter is how long a song found on neither side waits before it is
// looked for again: catalogues grow.
const retryAfter = 7 * 24 * time.Hour

// Found is what a search found of a song: the id on the other side, or
// none, and when.
type Found struct {
	ID    string    `json:"id,omitempty"` // NetEase id, or Spotify URI
	Score float64   `json:"score,omitempty"`
	At    time.Time `json:"at"`
	// Pinned is a match the listener chose, which searches don't replace.
	Pinned bool `json:"pinned,omitempty"`
}

// Link pairs a Spotify playlist with a NetEase one, to be kept in step.
type Link struct {
	Spotify   string    `json:"spotify"`
	Netease   int64     `json:"netease"`
	Name      string    `json:"name"`
	Direction Direction `json:"direction"`
	Synced    time.Time `json:"synced,omitempty"`
	// What each side had after the last sync: a song gone from a side
	// since, the listener took out, and a sync doesn't put it back.
	SeenSpotify []string `json:"seenSpotify,omitempty"` // track URIs
	SeenNetease []int64  `json:"seenNetease,omitempty"`
}

// Direction is which way songs go when a pair is synced.
type Direction string

const (
	ToNetease Direction = "to-netease" // Spotify's songs are added to NetEase
	ToSpotify Direction = "to-spotify"
	Both      Direction = "both"
)

// store is what crossplay keeps on disk, in the user data folder.
type store struct {
	ToNetease map[string]Found `json:"toNetease"` // by Spotify id
	ToSpotify map[string]Found `json:"toSpotify"` // by NetEase id
	Links     []Link           `json:"links"`
}

// The app and the MCP server both keep crossplay.json, each in its own
// process: what one reads it reads anew when the file changed, and what
// it writes it writes onto what the file has then, so that neither
// loses what the other wrote.
var (
	mu      sync.Mutex
	kept    *store
	keptMod time.Time // the file's modification time when read
	// Matches found and not yet written, which a write lays over the
	// file's.
	pendingNE = map[string]Found{}
	pendingSP = map[string]Found{}
	pathFn    = func() (string, error) {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "crossplay.json"), nil
	}
)

// read returns what the file has, and when it was changed. A file that
// isn't JSON, as one cut short, is put aside as crossplay.json.bad, not
// written over, and the store starts empty.
func read() (*store, time.Time) {
	s := &store{}
	var mod time.Time
	if p, err := pathFn(); err == nil {
		if fi, err := os.Stat(p); err == nil {
			mod = fi.ModTime()
			if b, err := os.ReadFile(p); err == nil && json.Unmarshal(b, s) != nil {
				os.Rename(p, p+".bad")
				s, mod = &store{}, time.Time{}
			}
		}
	}
	if s.ToNetease == nil {
		s.ToNetease = map[string]Found{}
	}
	if s.ToSpotify == nil {
		s.ToSpotify = map[string]Found{}
	}
	for k, f := range pendingNE {
		s.ToNetease[k] = f
	}
	for k, f := range pendingSP {
		s.ToSpotify[k] = f
	}
	return s, mod
}

// load returns the store, read again if the file changed since. mu is
// held.
func load() *store {
	var mod time.Time
	if p, err := pathFn(); err == nil {
		if fi, err := os.Stat(p); err == nil {
			mod = fi.ModTime()
		}
	}
	if kept == nil || !mod.Equal(keptMod) {
		kept, keptMod = read()
	}
	return kept
}

// update changes the store with fn, if not nil, onto what the file has
// now, and writes it with the matches pending. mu is held.
func update(fn func(*store)) error {
	s, _ := read()
	if fn != nil {
		fn(s)
	}
	p, err := pathFn()
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", " ")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", p, os.Getpid())
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	clear(pendingNE)
	clear(pendingSP)
	kept, keptMod = s, time.Time{}
	if fi, err := os.Stat(p); err == nil {
		keptMod = fi.ModTime()
	}
	return nil
}

// Save writes the matches found since the last write.
func Save() error {
	mu.Lock()
	defer mu.Unlock()
	if len(pendingNE) == 0 && len(pendingSP) == 0 {
		return nil
	}
	return update(nil)
}

// remember keeps a match found, to be written by Save. mu is held.
func remember(toNetease bool, key string, f Found) {
	s := load()
	if toNetease {
		pendingNE[key], s.ToNetease[key] = f, f
	} else {
		pendingSP[key], s.ToSpotify[key] = f, f
	}
}

// Links returns the pairs of playlists kept in step.
func Links() []Link {
	mu.Lock()
	defer mu.Unlock()
	return append([]Link(nil), load().Links...)
}

// SetLink adds the pair l, or replaces the pair of its Spotify playlist,
// and writes it.
func SetLink(l Link) error {
	mu.Lock()
	defer mu.Unlock()
	return update(func(s *store) {
		for i, x := range s.Links {
			if x.Spotify == l.Spotify {
				s.Links[i] = l
				return
			}
		}
		s.Links = append(s.Links, l)
	})
}

// Unlink stops keeping the Spotify playlist id in step.
func Unlink(id string) error {
	mu.Lock()
	defer mu.Unlock()
	return update(func(s *store) {
		s.Links = slices.DeleteFunc(s.Links, func(l Link) bool { return l.Spotify == id })
	})
}

// Pin keeps the listener's own choice of NetEase song for a Spotify one,
// which searches don't replace; 0 says NetEase hasn't it.
func Pin(spotifyID string, neteaseID int64) error {
	mu.Lock()
	defer mu.Unlock()
	f := Found{ID: itoa(neteaseID), At: time.Now(), Pinned: true, Score: 1}
	return update(func(s *store) { s.ToNetease[spotifyID] = f })
}

// known returns what was found of key before, if it still holds.
func known(m map[string]Found, key string) (Found, bool) {
	f, ok := m[key]
	if !ok {
		return f, false
	}
	if f.ID == "" && !f.Pinned && time.Since(f.At) > retryAfter {
		return f, false
	}
	return f, true
}

// SongOf describes a Spotify track for matching.
func SongOf(t *spotify.Track) match.Song {
	s := match.Song{Title: t.Name, Length: t.Length()}
	for _, a := range t.Artists {
		s.Artists = append(s.Artists, a.Name)
	}
	return s
}

// neteaseSong describes a NetEase track for matching, its artists by
// their other names too.
func neteaseSong(t *netease.Track) match.Song {
	s := match.Song{Title: t.Name, Length: t.Length()}
	for _, a := range t.Artists {
		s.Artists = append(s.Artists, a.Name)
		s.Artists = append(s.Artists, a.Alias...)
		s.Artists = append(s.Artists, a.Trans...)
	}
	return s
}

// FindNetease returns the NetEase song most like want and how alike, or
// nil when none is alike enough.
func FindNetease(want match.Song) (*netease.Track, float64, error) {
	q := want.Title
	if len(want.Artists) > 0 {
		q += " " + want.Artists[0]
	}
	songs, err := SearchNetease(q, 8)
	if err != nil {
		return nil, 0, err
	}
	cands := make([]match.Song, len(songs))
	for i := range songs {
		cands[i] = neteaseSong(&songs[i])
	}
	i, score := match.Best(want, cands)
	if i < 0 {
		return nil, score, nil
	}
	return &songs[i], score, nil
}

// FindSpotify returns the Spotify track most like want, or nil.
func FindSpotify(want match.Song) (*spotify.Track, float64, error) {
	q := "track:" + strings.ReplaceAll(match.TitleWords(want.Title), `"`, "")
	if len(want.Artists) > 0 {
		q += " artist:" + strings.ReplaceAll(want.Artists[0], `"`, "")
	}
	tracks, err := searchSpotify(q, 10)
	if err == nil && len(tracks) == 0 {
		// Field filters miss songs whose names differ in script or
		// spelling between the services: try the plain words.
		time.Sleep(spotifyPause)
		q = want.Title
		if len(want.Artists) > 0 {
			q += " " + want.Artists[0]
		}
		tracks, err = searchSpotify(q, 10)
	}
	if err != nil {
		return nil, 0, err
	}
	cands := make([]match.Song, len(tracks))
	for i := range tracks {
		cands[i] = SongOf(&tracks[i])
	}
	i, score := match.Best(want, cands)
	if i < 0 {
		return nil, score, nil
	}
	return &tracks[i], score, nil
}

// Blocked reports whether err is NetEase's risk control turning requests
// away, after which searching on only lengthens it.
func Blocked(err error) bool {
	b, _ := netease.Blocked(err)
	return b
}

// Budget bounds a search for many songs: at most MaxSearches searches (0
// for no bound), until Stop says to, as when the page asking is left.
// Progress, if set, hears after each song.
type Budget struct {
	MaxSearches int
	Stop        func() bool
	Progress    func(done, total int)
}

// ErrIncomplete is a search for many songs that ran out of its Budget
// before the last: what it found is kept, and the next picks up there.
var ErrIncomplete = errors.New("crossplay: more songs to look for")

// saveEvery is how many songs found are written at once, so that a long
// search cut short keeps most of them.
const saveEvery = 25

// findAll finds each of n songs by its key in the side's matches, looking
// for those not found before with search, within b. found holds the
// match of each, "" for none; it stops at the first error.
func findAll(n int, toNetease bool, key func(i int) string, search func(i int) (string, float64, error), pause time.Duration, b Budget) ([]string, error) {
	found := make([]string, n)
	searched := 0
	defer Save()
	for i := range n {
		k := key(i)
		mu.Lock()
		s := load()
		m := s.ToSpotify
		if toNetease {
			m = s.ToNetease
		}
		f, ok := known(m, k)
		mu.Unlock()
		if !ok {
			if (b.MaxSearches > 0 && searched >= b.MaxSearches) || (b.Stop != nil && b.Stop()) {
				return found, ErrIncomplete
			}
			if searched > 0 {
				time.Sleep(pause)
			}
			searched++
			id, score, err := search(i)
			if err != nil {
				return found, err
			}
			f = Found{ID: id, At: time.Now(), Score: score}
			mu.Lock()
			remember(toNetease, k, f)
			mu.Unlock()
			if searched%saveEvery == 0 {
				Save()
			}
		}
		found[i] = f.ID
		if b.Progress != nil {
			b.Progress(i+1, n)
		}
	}
	return found, nil
}

// NeteaseIDs finds each Spotify track on NetEase: the ids, 0 for those
// not found. Songs found before aren't looked for again.
func NeteaseIDs(tracks []spotify.Track, b Budget) ([]int64, error) {
	found, err := findAll(len(tracks), true,
		func(i int) string { return tracks[i].ID },
		func(i int) (string, float64, error) {
			t, score, err := FindNetease(SongOf(&tracks[i]))
			if t == nil {
				return "", score, err
			}
			return itoa(t.ID), score, err
		}, neteasePause, b)
	ids := make([]int64, len(found))
	for i, f := range found {
		ids[i] = atoi(f)
	}
	return ids, err
}

// KnownNeteaseIDs returns the NetEase ids of the Spotify tracks found
// before, 0 for the others, without searching: unknown counts those not
// looked for yet, or due to be looked for again.
func KnownNeteaseIDs(tracks []spotify.Track) (ids []int64, unknown int) {
	mu.Lock()
	defer mu.Unlock()
	s := load()
	ids = make([]int64, len(tracks))
	for i := range tracks {
		f, ok := known(s.ToNetease, tracks[i].ID)
		if !ok {
			unknown++
			continue
		}
		ids[i] = atoi(f.ID)
	}
	return ids, unknown
}

// SpotifyURIs finds each NetEase track on Spotify: the URIs, "" for those
// not found, as NeteaseIDs does the other way.
func SpotifyURIs(tracks []netease.Track, b Budget) ([]string, error) {
	return findAll(len(tracks), false,
		func(i int) string { return itoa(tracks[i].ID) },
		func(i int) (string, float64, error) {
			t, score, err := FindSpotify(neteaseSong(&tracks[i]))
			if t == nil {
				return "", score, err
			}
			return t.URI, score, err
		}, spotifyPause, b)
}

func itoa(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
