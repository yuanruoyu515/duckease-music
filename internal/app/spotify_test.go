package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		from:    []string{"spotify:track:a", "spotify:track:b"},
		missing: []spotify.Track{{ID: "c", URI: "spotify:track:c", Name: "Three"}},
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

// Copying to NetEase asks for the new playlist's name, the Spotify one's
// to start with, and imports under the name typed, once however often it
// is asked while it runs.
func TestSpotifyImportOnce(t *testing.T) {
	asked := make(chan string, 2)
	done := make(chan struct{})
	imp := crossImport
	defer func() { crossImport = imp; close(done) }()
	crossImport = func(id, name string, _ crossplay.Budget, _ crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		asked <- id + " " + name
		<-done // runs on until the test ends
		return crossplay.Link{}, crossplay.Report{}, nil
	}
	a := spotApp()
	a.route = route{page: pageSpotify, query: "S"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("复制到网易云"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d := a.spot.importing; !d.open || d.name != "Road Trip" {
		t.Fatalf("the dialog is %+v", d)
	}
	a.spot.importing.name = "公路旅行"
	if err := tt.Click("复制"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := wait(t, asked, "the import"); got != "S 公路旅行" {
		t.Errorf("imported %q", got)
	}
	a.openSpotImport("S", "Road Trip", false)
	a.submitSpotImport()
	select {
	case got := <-asked:
		t.Errorf("imported again: %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// Linking with a playlist the listener has syncs into it, only to
// NetEase unless they ask for both ways.
func TestSpotifyLinkExisting(t *testing.T) {
	asked := make(chan string, 1)
	link := crossLink
	defer func() { crossLink = link }()
	crossLink = func(sp string, ne int64, name string, d crossplay.Direction, _ crossplay.Budget, _ crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		asked <- fmt.Sprintf("%s %d %s", sp, ne, d)
		return crossplay.Link{}, crossplay.Report{}, nil
	}
	a := spotApp()
	a.mine = []netease.Playlist{{ID: 5, Name: "我喜欢的音乐", SpecialType: 5}, {ID: 7, Name: "公路"}}
	a.openSpotImport("S", "Road Trip", true)
	a.spot.importing.target = "公路"
	a.submitSpotImport()
	if got := wait(t, asked, "the link"); got != "S 7 to-netease" {
		t.Errorf("linked %q", got)
	}
	a.spot.busy = nil
	a.openSpotImport("S", "Road Trip", true)
	a.spot.importing.target, a.spot.importing.both = "我喜欢的音乐", true
	a.submitSpotImport()
	if got := wait(t, asked, "the link both ways"); got != "S 5 both" {
		t.Errorf("linked %q", got)
	}
}

// A Spotify playlist's page opens as it was kept, reading nothing off
// Spotify and searching NetEase for nothing.
func TestSpotifyPageOpensAsKept(t *testing.T) {
	items, find, read := spotifyItems, neteaseIDsOf, readSpotCache
	defer func() { spotifyItems, neteaseIDsOf, readSpotCache = items, find, read }()
	a := spotApp()
	kept := make(chan string, 1)
	readSpotCache = func(id string) (spotCache, bool) { kept <- id; return spotCache{Tracks: make([]spotify.Track, 2)}, true }
	asked := make(chan string, 2)
	spotifyItems = func(id string) ([]spotify.Track, error) { asked <- "spotify"; return nil, nil }
	neteaseIDsOf = func([]spotify.Track, crossplay.Budget) ([]int64, error) { asked <- "netease"; return nil, nil }
	a.spot.pages = nil
	a.open(route{page: pageSpotify, query: "S"})
	wait(t, kept, "the kept page")
	select {
	case got := <-asked:
		t.Errorf("opening asked %s", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// Update looks for the songs on NetEase; leaving the page lets it go on,
// and coming back or pressing it again doesn't start it again.
func TestSpotifyUpdateMatchesOnce(t *testing.T) {
	items, find := spotifyItems, neteaseIDsOf
	defer func() { spotifyItems, neteaseIDsOf = items, find }()
	spotifyItems = func(string) ([]spotify.Track, error) { return make([]spotify.Track, 3), nil }
	started, release := make(chan struct{}, 2), make(chan struct{})
	neteaseIDsOf = func(tracks []spotify.Track, b crossplay.Budget) ([]int64, error) {
		started <- struct{}{}
		<-release
		return make([]int64, len(tracks)), nil
	}
	defer close(release)
	a := spotApp()
	a.route = route{page: pageSpotify, query: "S"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("更新"); err != nil {
		t.Fatal(err)
	}
	wait(t, started, "the matching")
	a.open(route{page: pageHome})
	a.open(route{page: pageSpotify, query: "S"})
	a.updateSpotPage("S")
	select {
	case <-started:
		t.Error("the matching started again")
	case <-time.After(50 * time.Millisecond):
	}
	if pg := a.spot.pages["S"]; pg == nil || !pg.matching || len(pg.matched) != 2 {
		t.Errorf("the page's matching was dropped, or what it showed: %+v", pg)
	}
}

// A song's menu on a Spotify playlist's page takes it out of the
// playlist, at once on the page and then on Spotify.
func TestSpotifyRemoveSong(t *testing.T) {
	removed := make(chan string, 1)
	rm := spotifyRemove
	defer func() { spotifyRemove = rm }()
	spotifyRemove = func(id string, uris []string) error { removed <- id + " " + strings.Join(uris, ","); return nil }
	a := spotApp()
	a.route = route{page: pageSpotify, query: "S"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("One"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("从这个 Spotify 歌单移除"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	if got := wait(t, removed, "the removal"); got != "S spotify:track:a" {
		t.Errorf("removed %q", got)
	}
	if pg := a.spot.pages["S"]; len(pg.matched) != 1 || pg.matched[0].ID != 12 || pg.from[0] != "spotify:track:b" {
		t.Errorf("the page shows %v %v", pg.matched, pg.from)
	}
}

// Unticking a playlist in the chooser hides it from the sidebar.
func TestSpotifyChooserHides(t *testing.T) {
	a := spotApp()
	a.openSpotChooser()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Road Trip"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.spotHidden("S") {
		t.Fatalf("hidden %v", a.settings.SpotifyHidden)
	}
	a.spot.choosing.open = false
	tt.Frame()
	if err := tt.Click("Road Trip"); err == nil {
		t.Error("the hidden playlist is still in the sidebar")
	}
}

// With NM_FRAMES set, saves a Spotify playlist's page as its songs are
// found, the duck partway across, in light and dark.
func TestSpotifyLoadingFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("NM_FRAMES not set")
	}
	for _, dark := range []bool{false, true} {
		a := spotApp()
		if dark {
			a.settings.Appearance = "dark"
		}
		a.route = route{page: pageSpotify, query: "S"}
		a.spot.pages = map[string]*spotPage{"S": {loading: true, matching: true, done: 412, total: 1302}}
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(dark)
		for range 30 {
			tt.Frame()
		}
		name := "spotify-loading-light.png"
		if dark {
			name = "spotify-loading-dark.png"
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
}

// A Spotify playlist's page searches its songs and sorts them as a NetEase
// playlist's page does, and Play plays them in the order shown.
func TestSpotifyPageSearchesAndSorts(t *testing.T) {
	a := spotApp()
	a.spot.pages["S"].matched = []netease.Track{{ID: 11, Name: "Zebra"}, {ID: 12, Name: "Apple"}, {ID: 13, Name: "Mango"}}
	a.route = route{page: pageSpotify, query: "S"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("搜索歌单内歌曲"); err != nil {
		t.Fatal(err)
	}
	tt.Type("man")
	tt.Frame()
	if shown := a.plFilter.match(a.spotTracks(a.spot.pages["S"])); len(shown) != 1 {
		t.Fatalf("search for man shows %v", shown)
	}
	a.plFilter.query = ""
	a.plFilter.sort = sortName
	tt.Frame()
	if err := tt.Click("播放"); err != nil {
		t.Fatal(err)
	}
	if q := a.pl.Queue(); len(q) != 3 || q[0].Name != "Apple" || q[2].Name != "Zebra" {
		t.Errorf("played %v, want by name", q)
	}
}
