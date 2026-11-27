package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

// spotApp is an app signed in to both services, with one Spotify playlist
// of the listener's whose songs NetEase has two of.
func spotApp() *app {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "me"}
	a.spot.user, a.spot.name = "u", "me"
	a.spot.lists = []spotify.Playlist{{ID: "S", Name: "Road Trip", Owner: spotify.User{ID: "u", DisplayName: "me"}}}
	a.spot.pages = map[string]*spotPage{"S": {
		tracks:  make([]spotify.Track, 3),
		matched: []netease.Track{{ID: 11, Name: "One"}, {ID: 12, Name: "Two"}},
		missing: []spotify.Track{{Name: "Three"}},
	}}
	return a
}

// A Spotify playlist in the sidebar opens its page, which reads its songs.
func TestSpotifyPlaylistOpensFromSidebar(t *testing.T) {
	read := make(chan string, 1)
	items := spotifyItems
	defer func() { spotifyItems = items }()
	spotifyItems = func(id string) ([]spotify.Track, error) { read <- id; return nil, nil }
	a := spotApp()
	a.spot.pages = nil
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Road Trip"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.route != (route{page: pageSpotify, query: "S"}) {
		t.Fatalf("opened %+v", a.route)
	}
	if got := wait(t, read, "the playlist's songs"); got != "S" {
		t.Errorf("read %q", got)
	}
}

// Play on a Spotify playlist's page plays the songs NetEase has of it.
func TestSpotifyPlaylistPlaysMatches(t *testing.T) {
	a := spotApp()
	a.route = route{page: pageSpotify, query: "S"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("播放"); err != nil {
		t.Fatal(err)
	}
	if q := a.pl.Queue(); len(q) != 2 || q[0].ID != 11 {
		t.Errorf("queued %v", q)
	}
}

// Copying to NetEase starts an import of the playlist, once however often
// it is clicked while it runs.
func TestSpotifyImportOnce(t *testing.T) {
	asked := make(chan string, 2)
	done := make(chan struct{})
	imp := crossImport
	defer func() { crossImport = imp; close(done) }()
	crossImport = func(id, name string, _ crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		asked <- id + " " + name
		<-done // runs on until the test ends
		return crossplay.Link{}, crossplay.Report{}, nil
	}
	a := spotApp()
	a.route = route{page: pageSpotify, query: "S"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	for range 2 {
		if err := tt.Click("复制到网易云"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if got := wait(t, asked, "the import"); got != "S Road Trip" {
		t.Errorf("imported %q", got)
	}
	select {
	case got := <-asked:
		t.Errorf("imported again: %q", got)
	default:
	}
}
