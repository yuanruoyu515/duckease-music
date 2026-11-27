package crossplay

import (
	"path/filepath"
	"slices"
	"testing"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

func sp(id, name, artist string) spotify.Track {
	t := spotify.Track{ID: id, URI: "spotify:track:" + id, Name: name, Duration: 200000}
	t.Artists = append(t.Artists, struct {
		Name string `json:"name"`
	}{artist})
	return t
}

func ne(id int64, name, artist string) netease.Track {
	return netease.Track{ID: id, Name: name, Artists: []netease.Artist{{Name: artist}}, Duration: 200000}
}

// fakes stands both services in: Spotify's playlist has spotify, NetEase's
// has netease, and each search finds songs of the catalogues given.
func fakes(t *testing.T, onSpotify []spotify.Track, onNetease []netease.Track, spCat []spotify.Track, neCat []netease.Track) (toNetease *[]int64, toSpotify *[]string) {
	t.Helper()
	dir := t.TempDir()
	saved := struct {
		path     func() (string, error)
		items    func(string) ([]spotify.Track, error)
		add      func(string, []string) error
		detail   func(int64) (*netease.Playlist, error)
		neAdd    func(int64, ...int64) error
		neSearch func(string, int) ([]netease.Track, error)
		spSearch func(string, int) ([]spotify.Track, error)
	}{pathFn, spotifyItems, spotifyAdd, neteaseDetail, neteaseAdd, SearchNetease, searchSpotify}
	t.Cleanup(func() {
		pathFn, spotifyItems, spotifyAdd, neteaseDetail, neteaseAdd, SearchNetease, searchSpotify =
			saved.path, saved.items, saved.add, saved.detail, saved.neAdd, saved.neSearch, saved.spSearch
		kept, dirty = nil, false
	})
	kept, dirty = nil, false
	neteasePause, spotifyPause, neteaseAddPause = 0, 0, 0
	pathFn = func() (string, error) { return filepath.Join(dir, "crossplay.json"), nil }
	spotifyItems = func(string) ([]spotify.Track, error) { return onSpotify, nil }
	neteaseDetail = func(id int64) (*netease.Playlist, error) { return &netease.Playlist{ID: id, Tracks: onNetease}, nil }
	var neAdded []int64
	var spAdded []string
	neteaseAdd = func(_ int64, songs ...int64) error { neAdded = append(neAdded, songs...); return nil }
	spotifyAdd = func(_ string, uris []string) error { spAdded = append(spAdded, uris...); return nil }
	SearchNetease = func(string, int) ([]netease.Track, error) { return neCat, nil }
	searchSpotify = func(string, int) ([]spotify.Track, error) { return spCat, nil }
	return &neAdded, &spAdded
}

// Syncing both ways adds each side's missing songs once: a song NetEase
// has as the match of a Spotify one isn't sent back to Spotify, though
// Spotify's search would find another release of it.
func TestSyncBothAddsOnlyWhatIsMissing(t *testing.T) {
	shared := sp("a", "晴天", "周杰伦")
	reissue := sp("a2", "晴天", "周杰伦")
	onlySp := sp("b", "Stay", "Justin Bieber")
	toNe, toSp := fakes(t,
		[]spotify.Track{shared, onlySp},
		[]netease.Track{ne(1, "晴天", "周杰伦"), ne(3, "七里香", "周杰伦")},
		[]spotify.Track{reissue, sp("c", "七里香", "周杰伦")},
		[]netease.Track{ne(1, "晴天", "周杰伦"), ne(2, "Stay", "Justin Bieber")},
	)
	r, err := Sync(Link{Spotify: "S", Netease: 9, Direction: Both}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*toNe, []int64{2}) {
		t.Errorf("added to NetEase %v, want [2]", *toNe)
	}
	if !slices.Equal(*toSp, []string{"spotify:track:c"}) {
		t.Errorf("added to Spotify %v, want only 七里香", *toSp)
	}
	if r.AddedToNetease != 1 || r.AddedToSpotify != 1 {
		t.Errorf("report %+v", r)
	}
}

// Songs added to NetEase keep the Spotify playlist's order, though NetEase
// puts each one it adds at the top.
func TestImportKeepsOrder(t *testing.T) {
	a, b := sp("a", "One", "X"), sp("b", "Two", "Y")
	toNe, _ := fakes(t, []spotify.Track{a, b}, nil, nil, nil)
	SearchNetease = func(q string, _ int) ([]netease.Track, error) {
		if q == "One X" {
			return []netease.Track{ne(1, "One", "X")}, nil
		}
		return []netease.Track{ne(2, "Two", "Y")}, nil
	}
	if _, err := Sync(Link{Spotify: "S", Netease: 9, Direction: ToNetease}, nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*toNe, []int64{2, 1}) {
		t.Errorf("sent %v, want last first: [2 1]", *toNe)
	}
}

// A song matched once isn't searched for again, which keeps a second sync
// of a long playlist from asking NetEase hundreds of times.
func TestMatchesAreRemembered(t *testing.T) {
	fakes(t, []spotify.Track{sp("a", "One", "X")}, nil, nil, []netease.Track{ne(1, "One", "X")})
	asked := 0
	SearchNetease = func(string, int) ([]netease.Track, error) {
		asked++
		return []netease.Track{ne(1, "One", "X")}, nil
	}
	for range 2 {
		if _, err := Sync(Link{Spotify: "S", Netease: 9, Direction: ToNetease}, nil); err != nil {
			t.Fatal(err)
		}
		kept = nil // read back from disk
	}
	if asked != 1 {
		t.Errorf("searched %d times, want 1", asked)
	}
}
