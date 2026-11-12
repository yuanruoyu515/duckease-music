package app

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// pickTestApp is editTestApp with a third song in playlist 10, and no
// pause between a batch's requests.
func pickTestApp(t *testing.T) *app {
	old := bulkPause
	bulkPause = 0
	playlistDetail = func(id int64) (*netease.Playlist, error) { return &netease.Playlist{ID: id}, nil }
	t.Cleanup(func() { bulkPause, playlistDetail = old, netease.PlaylistDetail })
	a := editTestApp()
	pl := a.playlists[10]
	pl.Tracks = append(pl.Tracks, netease.Track{ID: 3, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}, Duration: 200000})
	return a
}

// While songs are picked, a click on a row picks it rather than playing
// it, Shift-click picks those between, and Escape stops picking.
func TestPickSongs(t *testing.T) {
	a := pickTestApp(t)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("批量选择"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("一张相片"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ClickWith(ui.Shift, "晴天"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.pickedSongs(a.playlists[10].Tracks); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Errorf("picked %v, want 1 to 3 by Shift-click", got)
	}
	if a.pl.Current() != nil {
		t.Error("a click while picking played a song")
	}
	if err := tt.Click("宝船"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.pickedSongs(a.playlists[10].Tracks); !slices.Equal(got, []int64{1, 3}) {
		t.Errorf("picked %v after a click on a picked song, want it dropped", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.pick.on {
		t.Error("Escape did not stop picking")
	}
}

// Removing the songs picked asks first, then takes them out of the
// playlist at once, in one request.
func TestRemovePickedSongs(t *testing.T) {
	calls := make(chan []int64, 1)
	removeFromPlaylist = func(pid int64, songs ...int64) error { calls <- songs; return nil }
	defer func() { removeFromPlaylist = netease.RemoveFromPlaylist }()

	a := pickTestApp(t)
	a.startPicking(10)
	a.pick.ids[1], a.pick.ids[3] = true, true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("移除"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	select {
	case <-calls:
		t.Fatal("removed without asking")
	default:
	}
	if !a.ask.open {
		t.Fatal("did not ask before removing")
	}
	a.ask.do()
	if got := <-calls; !slices.Equal(got, []int64{1, 3}) {
		t.Errorf("removed %v, want 1 and 3", got)
	}
	if n := len(a.playlists[10].Tracks); n != 1 {
		t.Errorf("%d songs listed, want the one not picked", n)
	}
}

// Adding the songs picked to another playlist sends them all at once.
func TestAddPickedSongs(t *testing.T) {
	type add struct {
		pid   int64
		songs []int64
	}
	calls := make(chan add, 1)
	addToPlaylist = func(pid int64, songs ...int64) error { calls <- add{pid, songs}; return nil }
	defer func() { addToPlaylist = netease.AddToPlaylist }()

	a := pickTestApp(t)
	a.startPicking(10)
	a.pick.ids[2], a.pick.ids[3] = true, true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("添加到歌单"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("睡前"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	got := <-calls
	if got.pid != 11 || !slices.Equal(got.songs, []int64{2, 3}) {
		t.Errorf("added %+v, want songs 2 and 3 to playlist 11", got)
	}
}

// Songs the playlist has already are left out of what is added, and with
// all of them there nothing is asked.
func TestAddInChunksSkipsSongsThere(t *testing.T) {
	old := bulkPause
	bulkPause = 0
	playlistDetail = func(id int64) (*netease.Playlist, error) {
		return &netease.Playlist{ID: id, Tracks: []netease.Track{{ID: 2}}}, nil
	}
	var added []int64
	addToPlaylist = func(pid int64, songs ...int64) error { added = append(added, songs...); return nil }
	defer func() {
		bulkPause, playlistDetail, addToPlaylist = old, netease.PlaylistDetail, netease.AddToPlaylist
	}()

	if err := addInChunks(1, []int64{1, 2, 3}); err != nil {
		t.Fatalf("got %v, want the others added", err)
	}
	if !slices.Equal(added, []int64{1, 3}) {
		t.Errorf("added %v, want 1 and 3", added)
	}
	added = nil
	if err := addInChunks(1, []int64{2}); !errors.Is(err, netease.ErrInPlaylist) || added != nil {
		t.Errorf("all there: got %v and added %v, want ErrInPlaylist and nothing asked", err, added)
	}
}

// Deleting playlists picked in the manager asks first, then deletes each,
// leaving the liked songs, and leaves the page of one deleted.
func TestManagerDeletes(t *testing.T) {
	deleted := make(chan int64, 4)
	deletePlaylist = func(id int64) error { deleted <- id; return nil }
	defer func() { deletePlaylist = netease.DeletePlaylist }()

	a := pickTestApp(t)
	a.history = []route{{page: pageHome}}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("管理我的歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if err := tt.Click("全选"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.ask.open {
		t.Fatal("did not ask before deleting")
	}
	a.ask.do()
	got := []int64{<-deleted, <-deleted}
	if !slices.Equal(got, []int64{10, 11}) {
		t.Errorf("deleted %v, want 10 and 11, not the liked songs", got)
	}
	if len(a.mine) != 1 || a.mine[0].ID != 5 {
		t.Errorf("the sidebar lists %v, want the liked songs alone", a.mine)
	}
	if a.route.page == pagePlaylist {
		t.Errorf("still on the deleted playlist's page")
	}
}

// Copying saved playlists makes one of the listener's for each, of the
// same name and songs.
func TestManagerCopiesSaved(t *testing.T) {
	type add struct {
		pid   int64
		songs []int64
	}
	adds := make(chan add, 1)
	addToPlaylist = func(pid int64, songs ...int64) error { adds <- add{pid, songs}; return nil }
	names := make(chan string, 1)
	createPlaylist = func(name string) (*netease.Playlist, error) {
		names <- name
		return &netease.Playlist{ID: 30, Name: name}, nil
	}
	a := pickTestApp(t)
	playlistDetail = func(id int64) (*netease.Playlist, error) {
		if id != 20 {
			return &netease.Playlist{ID: id}, nil
		}
		return &netease.Playlist{ID: id, Tracks: []netease.Track{{ID: 7}, {ID: 8}}}, nil
	}
	defer func() {
		addToPlaylist, createPlaylist, playlistDetail = netease.AddToPlaylist, netease.CreatePlaylist, netease.PlaylistDetail
	}()

	a.openManager(true)
	a.manager.ids[20] = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("复制为我的歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := <-names; got != "飙升榜" {
		t.Errorf("made %q, want 飙升榜", got)
	}
	if got := <-adds; got.pid != 30 || !slices.Equal(got.songs, []int64{7, 8}) {
		t.Errorf("added %+v, want 7 and 8 to playlist 30", got)
	}
}
