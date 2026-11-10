package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func trackNames(tracks []netease.Track) []string {
	names := make([]string, len(tracks))
	for i, t := range tracks {
		names[i] = t.Name
	}
	return names
}

// Songs sort by name as the listener reads them, Latin letters first,
// then Chinese by pinyin (阿 before 八, which has fewer strokes), and by
// length; the lucky order changes each time.
func TestPlaylistOrders(t *testing.T) {
	tracks := []netease.Track{
		{ID: 1, Name: "周", Duration: 3},
		{ID: 2, Name: "butterfly", Duration: 1},
		{ID: 3, Name: "安静", Duration: 2},
		{ID: 4, Name: "Daylight", Duration: 4},
		{ID: 5, Name: "八", Duration: 0},
		{ID: 6, Name: "阿", Duration: 5},
	}
	var o plOrder
	if got := trackNames(o.tracks(tracks, sortName, 0)); !slices.Equal(got, []string{"butterfly", "Daylight", "阿", "安静", "八", "周"}) {
		t.Errorf("by name: %v", got)
	}
	if got := trackNames(o.tracks(tracks, sortLongest, 0)); !slices.Equal(got, []string{"阿", "Daylight", "周", "安静", "butterfly", "八"}) {
		t.Errorf("longest first: %v", got)
	}
	if got := trackNames(o.tracks(tracks, sortReverse, 0)); !slices.Equal(got, []string{"阿", "八", "Daylight", "安静", "butterfly", "周"}) {
		t.Errorf("reversed: %v", got)
	}
	many := make([]netease.Track, 50)
	for i := range many {
		many[i].ID = int64(i)
	}
	first := slices.Clone(o.tracks(many, sortLucky, 1))
	if second := o.tracks(many, sortLucky, 2); slices.EqualFunc(first, second, func(a, b netease.Track) bool { return a.ID == b.ID }) {
		t.Error("the lucky order came out the same twice")
	}
}

// Trying one's luck reorders the playlist, and a click plays it in that
// order from the song clicked.
func TestPlaylistLucky(t *testing.T) {
	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("试试手气"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.plFilter.sort != sortLucky {
		t.Fatalf("sort %d: want the lucky order", a.plFilter.sort)
	}
	shown := a.plFilter.tracks(a.playlists[10])
	if err := tt.Click(shown[0].Name); err != nil {
		t.Fatal(err)
	}
	if q := a.pl.Queue(); len(q) != 2 || q[0].ID != shown[0].ID || a.pl.Current().ID != shown[0].ID {
		t.Fatalf("queue %v: want the lucky order, from its first", trackNames(q))
	}
}
