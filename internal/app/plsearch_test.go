package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Typing in a playlist's search field lists only the songs it finds, a
// click plays the whole playlist from that song, and a search that finds
// none offers to search all songs.
func TestPlaylistSearch(t *testing.T) {
	queries := make(chan string, 1)
	searchResults = func(q string, _ netease.SearchType, _, _ int) (netease.SearchResult, error) {
		queries <- q
		return netease.SearchResult{}, nil
	}
	defer func() { searchResults = netease.SearchPage }()

	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("搜索歌单内歌曲"); err != nil {
		t.Fatal(err)
	}
	tt.Type("doudou")
	tt.Frame()
	if tt.HasText("一张相片") {
		t.Fatal("a song the search doesn't find still listed")
	}
	if err := tt.Click("宝船"); err != nil {
		t.Fatal(err)
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != 2 || len(a.pl.Queue()) != 2 {
		t.Fatalf("playing %v of %d: want 宝船 of the whole playlist", cur, len(a.pl.Queue()))
	}

	a.plFilter.query = "不存在"
	tt.Frame()
	if err := tt.Click("在全部歌曲中搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.route.page != pageSearch || a.route.query != "不存在" || <-queries != "不存在" {
		t.Fatalf("route %+v: want the search of all songs", a.route)
	}
}
