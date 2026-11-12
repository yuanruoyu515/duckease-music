package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The picker lists each album cover once, those of the songs searched
// for, and a click on one downloads it at full size for the playlist.
func TestCoverPickerSearch(t *testing.T) {
	asked := make(chan string, 1)
	fetchSongCover = func(url string) ([]byte, error) { asked <- url; return nil, nil }
	dir := t.TempDir()
	coversDir = func() string { return dir }
	defer func() { fetchSongCover, coversDir = fetchArt, userCoversDir }()

	a := pickTestApp(t)
	tracks := a.playlists[10].Tracks
	tracks[0].Album = netease.Album{Name: "感同身受", PicURL: "https://p.music.126.net/a.jpg"}
	tracks[1].Album = netease.Album{Name: "宝船", PicURL: "https://p.music.126.net/b.jpg"}
	tracks[2].Album = netease.Album{Name: "叶惠美", PicURL: "https://p.music.126.net/a.jpg"}
	if urls, _ := pickerCovers(tracks, ""); len(urls) != 2 {
		t.Errorf("listed %v, want each cover once", urls)
	}
	if urls, of := pickerCovers(tracks, "doudou"); len(urls) != 1 || of[0].ID != 2 {
		t.Errorf("found %v for doudou, want 宝船's cover", urls)
	}

	a.openCoverPicker(10)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Type("宝船")
	tt.Frame()
	if _, ok := tt.Find("一张相片 · 感同身受"); ok {
		t.Error("a cover the search doesn't find still shows")
	}
	if err := tt.Click("宝船 · 宝船"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	select {
	case got := <-asked:
		if got != netease.Sized("https://p.music.126.net/b.jpg", coverSide) {
			t.Errorf("downloaded %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a click on the cover downloaded nothing")
	}
}
