// Package crossplay carries playlists between Spotify and NetEase: it
// finds each song of one on the other, remembers what it found, and keeps
// pairs of playlists in step.
package crossplay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	neteasePause = 200 * time.Millisecond
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

var (
	mu     sync.Mutex
	kept   *store
	dirty  bool
	pathFn = func() (string, error) {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "crossplay.json"), nil
	}
)

// load returns the store, read from disk the first time. mu is held.
func load() *store {
	if kept != nil {
		return kept
	}
	kept = &store{}
	if p, err := pathFn(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			json.Unmarshal(b, kept)
		}
	}
	if kept.ToNetease == nil {
		kept.ToNetease = map[string]Found{}
	}
	if kept.ToSpotify == nil {
		kept.ToSpotify = map[string]Found{}
	}
	return kept
}

// Reload forgets what was read, so that the next call reads the file
// again: the app and the MCP server both write it.
func Reload() {
	mu.Lock()
	defer mu.Unlock()
	if !dirty {
		kept = nil
	}
}

// Save writes the store if it changed.
func Save() error {
	mu.Lock()
	defer mu.Unlock()
	if !dirty || kept == nil {
		return nil
	}
	p, err := pathFn()
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(kept, "", " ")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	dirty = false
	return nil
}

// Links returns the pairs of playlists kept in step.
func Links() []Link {
	mu.Lock()
	defer mu.Unlock()
	return append([]Link(nil), load().Links...)
}

// SetLink adds the pair l, or replaces the pair of its Spotify playlist.
func SetLink(l Link) {
	mu.Lock()
	defer mu.Unlock()
	s := load()
	for i, x := range s.Links {
		if x.Spotify == l.Spotify {
			s.Links[i] = l
			dirty = true
			return
		}
	}
	s.Links = append(s.Links, l)
	dirty = true
}

// Unlink stops keeping the Spotify playlist id in step.
func Unlink(id string) {
	mu.Lock()
	defer mu.Unlock()
	s := load()
	for i, x := range s.Links {
		if x.Spotify == id {
			s.Links = append(s.Links[:i], s.Links[i+1:]...)
			dirty = true
			return
		}
	}
}

// Pin keeps the listener's own choice of NetEase song for a Spotify one;
// 0 says it has none.
func Pin(spotifyID string, neteaseID int64) {
	mu.Lock()
	defer mu.Unlock()
	f := Found{At: time.Now(), Pinned: true, Score: 1}
	if neteaseID != 0 {
		f.ID = itoa(neteaseID)
	}
	load().ToNetease[spotifyID] = f
	dirty = true
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

// neteaseSong describes a NetEase track for matching.
func neteaseSong(t *netease.Track) match.Song {
	s := match.Song{Title: t.Name, Length: t.Length()}
	for _, a := range t.Artists {
		s.Artists = append(s.Artists, a.Name)
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
	var r *netease.RefusedError
	return errors.As(err, &r) && r.Blocked()
}

// NeteaseIDs finds each Spotify track on NetEase: the ids, 0 for those
// not found. Songs found before aren't looked for again. progress, if not
// nil, hears after each song. It stops at the first error, returning what
// it found so far.
func NeteaseIDs(tracks []spotify.Track, progress func(done, total int)) ([]int64, error) {
	ids := make([]int64, len(tracks))
	searched := 0
	defer Save()
	for i := range tracks {
		t := &tracks[i]
		mu.Lock()
		f, ok := known(load().ToNetease, t.ID)
		mu.Unlock()
		if !ok {
			if searched > 0 {
				time.Sleep(neteasePause)
			}
			searched++
			found, score, err := FindNetease(SongOf(t))
			if err != nil {
				return ids, err
			}
			f = Found{At: time.Now(), Score: score}
			if found != nil {
				f.ID = itoa(found.ID)
			}
			mu.Lock()
			load().ToNetease[t.ID] = f
			dirty = true
			mu.Unlock()
		}
		ids[i] = atoi(f.ID)
		if progress != nil {
			progress(i+1, len(tracks))
		}
	}
	return ids, nil
}

// SpotifyURIs finds each NetEase track on Spotify: the URIs, "" for those
// not found, as NeteaseIDs does the other way.
func SpotifyURIs(tracks []netease.Track, progress func(done, total int)) ([]string, error) {
	uris := make([]string, len(tracks))
	searched := 0
	defer Save()
	for i := range tracks {
		t := &tracks[i]
		key := itoa(t.ID)
		mu.Lock()
		f, ok := known(load().ToSpotify, key)
		mu.Unlock()
		if !ok {
			if searched > 0 {
				time.Sleep(spotifyPause)
			}
			searched++
			found, score, err := FindSpotify(neteaseSong(t))
			if err != nil {
				return uris, err
			}
			f = Found{At: time.Now(), Score: score}
			if found != nil {
				f.ID = found.URI
			}
			mu.Lock()
			load().ToSpotify[key] = f
			dirty = true
			mu.Unlock()
		}
		uris[i] = f.ID
		if progress != nil {
			progress(i+1, len(tracks))
		}
	}
	return uris, nil
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
