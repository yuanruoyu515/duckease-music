package mcp

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/netease"
)

// fake replaces NetEase with one listener who owns playlist 1, with songs
// 10 and 11, and has saved playlist 2 of someone else's.
func fake(t *testing.T) (added *[][]int64) {
	t.Helper()
	acc, lists, detail, add, find, wait := account, userPlaylists, playlistDetail, addToPlaylist, searchSongs, pause
	forget := func() {
		cache.uid, cache.playlists = 0, nil
		cache.listsAt = time.Time{}
	}
	t.Cleanup(func() {
		account, userPlaylists, playlistDetail, addToPlaylist, searchSongs, pause = acc, lists, detail, add, find, wait
		forget()
	})
	forget()
	pause = 0
	account = func() (*netease.Profile, error) { return &netease.Profile{UserID: 7, Nickname: "me"}, nil }
	userPlaylists = func(int64) ([]netease.Playlist, []netease.Playlist, error) {
		return []netease.Playlist{{ID: 1, Name: "mine", TrackCount: 2}}, []netease.Playlist{{ID: 2, Name: "theirs", Subscribed: true}}, nil
	}
	playlistDetail = func(id int64) (*netease.Playlist, error) {
		return &netease.Playlist{ID: id, Name: "mine", TrackCount: 2, Tracks: []netease.Track{{ID: 10, Name: "a"}, {ID: 11, Name: "b"}}}, nil
	}
	var calls [][]int64
	addToPlaylist = func(_ int64, songs ...int64) error {
		calls = append(calls, songs)
		return nil
	}
	return &calls
}

func callTool(t *testing.T, name string, args any) (string, bool) {
	t.Helper()
	b, _ := json.Marshal(args)
	for _, tl := range newTools() {
		if tl.name == name {
			res := call(tl, b)
			text := res["content"].([]map[string]any)[0]["text"].(string)
			return text, res["isError"] == true
		}
	}
	t.Fatalf("no tool %s", name)
	return "", false
}

// Adding skips songs the playlist has, and sends the rest last first, so
// that they end up at its top in the order given.
func TestAddSongsKeepsOrderAndSkipsKnown(t *testing.T) {
	added := fake(t)
	text, isErr := callTool(t, "add_songs", map[string]any{"id": 1, "song_ids": []any{20, 10, "21", 20}})
	if isErr {
		t.Fatal(text)
	}
	if len(*added) != 1 || !slices.Equal((*added)[0], []int64{21, 20}) {
		t.Errorf("sent %v, want [[21 20]]", *added)
	}
}

// Someone else's playlist can't be changed, and nothing is sent.
func TestEditRefusesOthersPlaylists(t *testing.T) {
	added := fake(t)
	text, isErr := callTool(t, "add_songs", map[string]any{"id": 2, "song_ids": []int{20}})
	if !isErr || !strings.Contains(text, "someone else") || len(*added) > 0 {
		t.Errorf("got %q, error %v, sent %v", text, isErr, *added)
	}
}

// Matching stops at NetEase's -460, instead of asking on for every line.
func TestMatchStopsWhenRefused(t *testing.T) {
	fake(t)
	asked := 0
	find := crossplay.SearchNetease
	defer func() { crossplay.SearchNetease = find }()
	crossplay.SearchNetease = func(string, int) ([]netease.Track, error) {
		asked++
		return nil, &netease.RefusedError{Code: -460}
	}
	text, _ := callTool(t, "match", map[string]any{"songs": []string{"a - x", "b - y", "c - z"}})
	if asked != 1 || !strings.Contains(text, "-460") {
		t.Errorf("asked %d times: %s", asked, text)
	}
}

// The server answers initialize and tools/list, and says nothing to a
// notification, which a client would take for a reply to something else.
func TestServeProtocol(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":"x","method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"nope"}
`)
	var out bytes.Buffer
	if err := Serve(in, &out, "test"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d replies: %s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"protocolVersion":"2024-11-05"`) || !strings.Contains(lines[1], `"id":"x"`) || !strings.Contains(lines[2], "-32601") {
		t.Errorf("replies:\n%s", out.String())
	}
}

// A live take isn't a duplicate of the studio song, but the same song
// with the artist spelled "S H E" or "S.H.E" is.
func TestDuplicatesKeepVersionsApart(t *testing.T) {
	fake(t)
	playlistDetail = func(id int64) (*netease.Playlist, error) {
		return &netease.Playlist{ID: id, Tracks: []netease.Track{
			{ID: 1, Name: "迫不及待", Artists: []netease.Artist{{Name: "S H E"}}},
			{ID: 2, Name: "迫不及待", Artists: []netease.Artist{{Name: "S.H.E"}}},
			{ID: 3, Name: "十面埋伏", Artists: []netease.Artist{{Name: "S.H.E"}}},
			{ID: 4, Name: "十面埋伏(Live)", Artists: []netease.Artist{{Name: "S.H.E"}}},
		}}, nil
	}
	text, _ := callTool(t, "duplicates", map[string]any{"id": 1})
	if !strings.HasPrefix(text, "1 groups") || strings.Contains(text, "十面埋伏") {
		t.Errorf("got:\n%s", text)
	}
}

// A NetEase link gives its id parameter, not userid's, which a live test
// read as playlist 1.
func TestIDFromLink(t *testing.T) {
	var got id
	if err := json.Unmarshal([]byte(`"https://music.163.com/#/playlist?id=17752554350&userid=1"`), &got); err != nil || got != 17752554350 {
		t.Errorf("got %d, %v", got, err)
	}
}

// Matching stops when the call's time is up, saying which lines are left,
// so that the client gets its answer before it gives up waiting.
func TestMatchStopsInTime(t *testing.T) {
	fake(t)
	budget, mp := callBudget, matchPause
	defer func() { callBudget, matchPause = budget, mp }()
	callBudget, matchPause = 0, 0
	asked := 0
	find := crossplay.SearchNetease
	defer func() { crossplay.SearchNetease = find }()
	crossplay.SearchNetease = func(string, int) ([]netease.Track, error) { asked++; return nil, nil }
	text, isErr := callTool(t, "match", map[string]any{"songs": []string{"a - x", "b - y", "c - z"}})
	if isErr || asked != 1 || !strings.Contains(text, "lines 2-3") {
		t.Errorf("asked %d times: %s", asked, text)
	}
}

// NetEase holding searches for a few seconds after a 405 is waited out
// within the call, not handed to the agent as an error.
func TestMatchWaitsOutTooOften(t *testing.T) {
	fake(t)
	mp := matchPause
	defer func() { matchPause = mp }()
	matchPause = 0
	asked := 0
	find := crossplay.SearchNetease
	defer func() { crossplay.SearchNetease = find }()
	crossplay.SearchNetease = func(string, int) ([]netease.Track, error) {
		asked++
		if asked == 1 {
			return nil, &netease.CoolingError{Until: time.Now().Add(10 * time.Millisecond), TooOften: true}
		}
		return []netease.Track{{ID: 5, Name: "a", Artists: []netease.Artist{{Name: "x"}}}}, nil
	}
	text, _ := callTool(t, "match", map[string]any{"songs": []string{"a - x"}})
	if asked != 2 || !strings.Contains(text, "matched 1/1") {
		t.Errorf("asked %d times: %s", asked, text)
	}
}

// A call that runs past its limit is answered with an error, and the
// call after it, while it still runs, is told so at once rather than
// waiting behind it, as a sync stuck for five minutes made a status
// check wait too.
func TestStuckCallDoesNotHoldOthers(t *testing.T) {
	fake(t)
	limit, wait := callLimit, busyWait
	defer func() { callLimit, busyWait = limit, wait }()
	callLimit, busyWait = 50*time.Millisecond, 20*time.Millisecond
	release := make(chan struct{})
	defer close(release)
	searchSongs = func(string, int) ([]netease.Track, error) { <-release; return nil, nil }
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"a"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"playlists"}}
`)
	var out bytes.Buffer
	if err := Serve(in, &out, "test"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "took over") || !strings.Contains(lines[1], "still running") {
		t.Errorf("replies:\n%s", out.String())
	}
}

// While the keychain waits on the listener, the server still answers
// initialize, and a call says to look for the dialog instead of hanging.
func TestServeWhileKeychainWaits(t *testing.T) {
	ready, wait := SessionReady, sessionWait
	defer func() { SessionReady, sessionWait = ready, wait }()
	SessionReady, sessionWait = make(chan struct{}), 10*time.Millisecond
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"playlists"}}
`)
	var out bytes.Buffer
	if err := Serve(in, &out, "test"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "protocolVersion") || !strings.Contains(lines[1], "Always Allow") {
		t.Errorf("replies:\n%s", out.String())
	}
}
