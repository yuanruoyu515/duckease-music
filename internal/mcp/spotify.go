package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

// Tools for the listener's Spotify playlists, and for keeping them in
// step with NetEase. Connecting to Spotify happens in the app's settings.

// A call looks for songs for callBudget, about a second each: a client
// gives up on a call after a minute or so, while the server would go on
// changing the account. What a call found is kept, and the next call
// picks up there.

// maxMissing is how many songs a reply lists as missing, the rest
// counted.
const maxMissing = 30

func spotifyTools() []tool {
	return []tool{
		{
			name: "spotify_playlists", title: "List Spotify playlists", readOnly: true,
			description: "The user's Spotify playlists. Lines: id, name, songs, kind (mine, collab, followed = songs can't be read), linked NetEase playlist id or -.",
			run:         func(json.RawMessage) (string, error) { return spotifyPlaylists() },
		},
		{
			name: "spotify_playlist", title: "Read a Spotify playlist", readOnly: true,
			description: "Songs of one of the user's own or collaborative Spotify playlists. Lines: n, title, artists, length, and the track id with ids=true (for fix_match).",
			params: map[string]any{
				"id":     strParam("Spotify playlist id, URI or link"),
				"query":  strParam("optional filter"),
				"offset": intParam("skip this many songs (default 0)"),
				"limit":  intParam("at most this many songs (default 300)"),
				"ids":    boolParam("add the Spotify track id to each line"),
			},
			required: []string{"id"},
			run:      spotifyPlaylist,
		},
		{
			name: "spotify_import", title: "Copy a Spotify playlist to NetEase",
			description: "Create a NetEase playlist for a Spotify playlist and link the two, then sync them as sync_playlist does. Songs are found by title, artist and length, for about 45 seconds a call: when the reply says to, call sync_playlist to go on.",
			params:      map[string]any{"id": strParam("Spotify playlist id, URI or link"), "name": strParam("NetEase playlist name; default the Spotify one's")},
			required:    []string{"id"},
			run:         spotifyImport,
		},
		{
			name: "spotify_export", title: "Copy a NetEase playlist to Spotify",
			description: "Create a private Spotify playlist for a NetEase playlist and link the two, then sync them. Call sync_playlist to go on when the reply says to.",
			params:      map[string]any{"id": playlistArg, "name": strParam("Spotify playlist name; default the NetEase one's")},
			required:    []string{"id"},
			run:         spotifyExport,
		},
		{
			name: "sync_links", title: "List linked playlists", readOnly: true,
			description: "Pairs of Spotify and NetEase playlists kept in step. Lines: spotify id, netease id, direction, name, last synced.",
			run:         func(json.RawMessage) (string, error) { return syncLinks() },
		},
		{
			name: "sync_playlist", title: "Sync linked playlists",
			description: "Bring songs across a linked pair (to-netease, to-spotify or both; default the pair's). Only adds, never removes, and doesn't put back songs the user removed since the last sync. Looks for songs for about 45 seconds a call and adds nothing until all are found: call again while the reply says to. Pass netease_id to link a new pair first. spotify_id \"liked\" is the user's Spotify Liked Songs.",
			params: map[string]any{
				"spotify_id": strParam("Spotify playlist id, URI or link"),
				"netease_id": intParam("the user's NetEase playlist id, to link a new pair"),
				"direction":  map[string]any{"type": "string", "enum": []string{"to-netease", "to-spotify", "both"}},
			},
			required: []string{"spotify_id"},
			run:      syncPlaylist,
		},
		{
			name: "spotify_search", title: "Search Spotify", readOnly: true,
			description: "Search Spotify for songs (at most 10 results, Spotify's limit for the user's app). Lines: uri, title, artists, album, length.",
			params:      map[string]any{"query": strParam("keywords; track:NAME artist:NAME narrows it")},
			required:    []string{"query"},
			run:         spotifySearch,
		},
		{
			name: "spotify_add_songs", title: "Add songs to a Spotify playlist",
			description: "Add songs, by Spotify track URI or id, to the end of one of the user's own or collaborative Spotify playlists; playlist \"liked\" likes them. Songs the playlist has are skipped.",
			params:      map[string]any{"id": strParam("Spotify playlist id, URI or link, or liked"), "uris": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "track URIs or ids, up to 200"}},
			required:    []string{"id", "uris"},
			run:         func(a json.RawMessage) (string, error) { return spotifyEdit(a, true) },
		},
		{
			name: "spotify_remove_songs", title: "Remove songs from a Spotify playlist", destructive: true,
			description: "Remove songs, by Spotify track URI or id, from one of the user's own or collaborative Spotify playlists; playlist \"liked\" unlikes them.",
			params:      map[string]any{"id": strParam("Spotify playlist id, URI or link, or liked"), "uris": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "track URIs or ids, up to 200"}},
			required:    []string{"id", "uris"},
			run:         func(a json.RawMessage) (string, error) { return spotifyEdit(a, false) },
		},
		{
			name: "fix_match", title: "Correct a song's NetEase match",
			description: "Set which NetEase song a Spotify track is, for imports and syncs from now on; netease_id 0 says NetEase hasn't it. Get track ids from spotify_playlist with ids=true.",
			params: map[string]any{
				"spotify_track": strParam("Spotify track id or URI"),
				"netease_id":    intParam("NetEase song id, or 0"),
			},
			required: []string{"spotify_track", "netease_id"},
			run:      fixMatch,
		},
	}
}

// spotifyID takes a Spotify id from what an agent may pass: the id, a URI
// (spotify:playlist:…) or a link (open.spotify.com/playlist/…?si=…).
func spotifyID(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, ":/"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

func spotifyUser() (string, error) {
	id, _ := spotify.Account()
	if id == "" {
		return "", errors.New("Spotify isn't connected: connect it in DuckEase Music's settings first")
	}
	return id, nil
}

func linkOf(spotifyID string) (crossplay.Link, bool) {
	for _, l := range crossplay.Links() {
		if l.Spotify == spotifyID {
			return l, true
		}
	}
	return crossplay.Link{}, false
}

func spotifyPlaylists() (string, error) {
	user, err := spotifyUser()
	if err != nil {
		return "", err
	}
	lists, err := spotify.Playlists()
	if err != nil {
		return "", err
	}
	linked := map[string]int64{}
	for _, l := range crossplay.Links() {
		linked[l.Spotify] = l.Netease
	}
	var b strings.Builder
	b.WriteString("id\tname\tsongs\tkind\tnetease\n")
	if !spotify.NeedsReconnect() {
		if n, err := spotify.LikedCount(); err == nil {
			ne := "-"
			if id, ok := linked[spotify.LikedID]; ok {
				ne = fmt.Sprint(id)
			}
			fmt.Fprintf(&b, "%s\tLiked Songs\t%d\tliked\t%s\n", spotify.LikedID, n, ne)
		}
	}
	for _, p := range lists {
		kind := "followed (first 100 songs readable)"
		switch {
		case p.Owner.ID == user:
			kind = "mine"
		case p.Collaborative:
			kind = "collab"
		}
		ne := "-"
		if id, ok := linked[p.ID]; ok {
			ne = fmt.Sprint(id)
		}
		fmt.Fprintf(&b, "%s\t%s\t%d\t%s\t%s\n", p.ID, clean(p.Name), p.Total(), kind, ne)
	}
	return b.String(), nil
}

func spotifyPlaylist(args json.RawMessage) (string, error) {
	var a struct {
		ID, Query     string
		Offset, Limit int
		IDs           bool `json:"ids"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	if a.Limit <= 0 {
		a.Limit = 300
	}
	tracks, err := spotify.PlaylistItems(spotifyID(a.ID))
	if err != nil {
		return "", err
	}
	q := strings.ToLower(a.Query)
	var b strings.Builder
	fmt.Fprintf(&b, "%d songs\nn\ttitle\tartists\tlength", len(tracks))
	if a.IDs {
		b.WriteString("\tid")
	}
	b.WriteByte('\n')
	shown, skipped := 0, 0
	for i, t := range tracks {
		if q != "" && !strings.Contains(strings.ToLower(t.Name+"\x00"+t.ArtistNames()+"\x00"+t.Album.Name), q) {
			continue
		}
		if skipped < a.Offset {
			skipped++
			continue
		}
		if shown == a.Limit {
			fmt.Fprintf(&b, "(more: pass offset %d)\n", a.Offset+shown)
			break
		}
		shown++
		secs := t.Duration / 1000
		fmt.Fprintf(&b, "%d\t%s\t%s\t%d:%02d", i+1, clean(t.Name), clean(t.ArtistNames()), secs/60, secs%60)
		if a.IDs {
			b.WriteString("\t" + t.ID)
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func listSome(lines []string) string {
	n := min(len(lines), maxMissing)
	out := make([]string, n)
	for i, l := range lines[:n] {
		out[i] = clean(l)
	}
	s := strings.Join(out, "\n")
	if len(lines) > n {
		s += fmt.Sprintf("\n(+%d more)", len(lines)-n)
	}
	return s
}

func report(r crossplay.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "added %d to NetEase, %d to Spotify\n", r.AddedToNetease, r.AddedToSpotify)
	if r.Removed > 0 {
		fmt.Fprintf(&b, "%d songs the user removed from one side since the last sync were left out\n", r.Removed)
	}
	if r.Refused > 0 {
		fmt.Fprintf(&b, "NetEase turned away %d songs as already in the playlist\n", r.Refused)
	}
	if r.Truncated {
		b.WriteString(fmt.Sprintf("the NetEase playlist has over %d songs; only the first %d were read\n", netease.MaxTracks, netease.MaxTracks))
	}
	if len(r.MissingOnNetease) > 0 {
		fmt.Fprintf(&b, "not found on NetEase (%d):\n%s\n", len(r.MissingOnNetease), listSome(r.MissingOnNetease))
	}
	if len(r.MissingOnSpotify) > 0 {
		fmt.Fprintf(&b, "not found on Spotify (%d):\n%s\n", len(r.MissingOnSpotify), listSome(r.MissingOnSpotify))
	}
	return b.String()
}

// budget bounds one call's searches by its time, and counts how far they
// got, telling the client as they go.
func budget(at *[2]int) crossplay.Budget {
	ctx := current()
	return crossplay.Budget{Stop: ctx.over, Progress: func(done, total int) {
		at[0], at[1] = done, total
		if done%10 == 0 || done == total {
			ctx.tell(done, total, "finding songs")
		}
	}}
}

// outcome says what a sync did, or how far it got and what to do next.
func outcome(r crossplay.Report, err error, at [2]int, spotifyID string) (string, error) {
	switch {
	case errors.Is(err, crossplay.ErrIncomplete):
		return fmt.Sprintf("songs found so far: %d of %d; nothing added yet (each call looks for about %s, so that it answers in time). Call sync_playlist with spotify_id %s to go on; matches found are kept.\n", at[0], at[1], callBudget, spotifyID), nil
	case crossplay.Blocked(err):
		return "", err // the server says how long to wait
	case err != nil:
		return "", fmt.Errorf("%w (%s)", err, r)
	}
	return report(r), nil
}

func spotifyImport(args json.RawMessage) (string, error) {
	var a struct{ ID, Name string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	a.ID = spotifyID(a.ID)
	user, err := spotifyUser()
	if err != nil {
		return "", err
	}
	if _, err := me(); err != nil {
		return "", err
	}
	if l, ok := linkOf(a.ID); ok {
		return "", fmt.Errorf("already linked to NetEase playlist %d; use sync_playlist", l.Netease)
	}
	lists, err := spotify.Playlists()
	if err != nil {
		return "", err
	}
	var pl *spotify.Playlist
	for i := range lists {
		if lists[i].ID == a.ID {
			pl = &lists[i]
		}
	}
	switch {
	case pl == nil:
		return "", fmt.Errorf("no Spotify playlist %s among the user's; call spotify_playlists", a.ID)
	case !pl.Editable(user):
		return "", spotify.ErrNotReadable
	}
	if a.Name == "" {
		a.Name = pl.Name
	}
	var at [2]int
	l, r, err := crossplay.ImportToNetease(a.ID, a.Name, budget(&at), nil)
	if l.Netease != 0 {
		changed(l.Netease)
		cache.Lock()
		cache.listsAt = cache.listsAt.AddDate(-1, 0, 0)
		cache.Unlock()
	}
	text, err := outcome(r, err, at, a.ID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("NetEase playlist %d %s\n%s", l.Netease, clean(a.Name), text), nil
}

func spotifyExport(args json.RawMessage) (string, error) {
	var a struct {
		ID   id     `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	if l, ok := neteaseLink(int64(a.ID)); ok {
		return "", fmt.Errorf("already linked to Spotify playlist %s; use sync_playlist", l.Spotify)
	}
	if a.Name == "" {
		pl, err := playlist(int64(a.ID))
		if err != nil {
			return "", err
		}
		a.Name = pl.Name
	}
	var at [2]int
	l, r, err := crossplay.ExportToSpotify(int64(a.ID), a.Name, budget(&at), nil)
	text, err := outcome(r, err, at, l.Spotify)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Spotify playlist %s %s\n%s", l.Spotify, clean(a.Name), text), nil
}

func neteaseLink(id int64) (crossplay.Link, bool) {
	for _, l := range crossplay.Links() {
		if l.Netease == id {
			return l, true
		}
	}
	return crossplay.Link{}, false
}

func syncLinks() (string, error) {
	links := crossplay.Links()
	if len(links) == 0 {
		return "no linked playlists\n", nil
	}
	var b strings.Builder
	b.WriteString("spotify\tnetease\tdirection\tname\tsynced\n")
	for _, l := range links {
		synced := "never"
		if !l.Synced.IsZero() {
			synced = l.Synced.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(&b, "%s\t%d\t%s\t%s\t%s\n", l.Spotify, l.Netease, l.Direction, clean(l.Name), synced)
	}
	return b.String(), nil
}

func syncPlaylist(args json.RawMessage) (string, error) {
	var a struct {
		Spotify   string              `json:"spotify_id"`
		Netease   id                  `json:"netease_id"`
		Direction crossplay.Direction `json:"direction"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	a.Spotify = spotifyID(a.Spotify)
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	l, ok := linkOf(a.Spotify)
	if a.Netease != 0 && (!ok || l.Netease != int64(a.Netease)) {
		p, err := own(int64(a.Netease))
		if err != nil {
			return "", err
		}
		l = crossplay.Link{Spotify: a.Spotify, Netease: p.ID, Direction: crossplay.Both, Name: p.Name}
	} else if !ok {
		return "", errors.New("this Spotify playlist isn't linked; pass netease_id to link it, or use spotify_import")
	}
	switch a.Direction {
	case "":
	case crossplay.ToNetease, crossplay.ToSpotify, crossplay.Both:
		l.Direction = a.Direction
	default:
		return "", fmt.Errorf("direction %q: use to-netease, to-spotify or both", a.Direction)
	}
	var at [2]int
	r, err := crossplay.Sync(l, budget(&at), nil)
	changed(l.Netease)
	return outcome(r, err, at, l.Spotify)
}

func fixMatch(args json.RawMessage) (string, error) {
	var a struct {
		Track   string `json:"spotify_track"`
		Netease id     `json:"netease_id"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	track := spotifyID(a.Track)
	if track == "" {
		return "", errors.New("empty spotify_track")
	}
	if err := crossplay.Pin(track, int64(a.Netease)); err != nil {
		return "", err
	}
	if a.Netease == 0 {
		return fmt.Sprintf("%s: marked as not on NetEase\n", track), nil
	}
	return fmt.Sprintf("%s → %d\n", track, a.Netease), nil
}

func spotifySearch(args json.RawMessage) (string, error) {
	var a struct{ Query string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	tracks, err := spotify.Search(a.Query, 10)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("uri\ttitle\tartists\talbum\tlength\n")
	for _, t := range tracks {
		secs := t.Duration / 1000
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d:%02d\n", t.URI, clean(t.Name), clean(t.ArtistNames()), clean(t.Album.Name), secs/60, secs%60)
	}
	if len(tracks) == 0 {
		b.WriteString("(nothing found)\n")
	}
	return b.String(), nil
}

// trackURI takes a Spotify track URI from a URI, an id or a link.
func trackURI(s string) string {
	id := spotifyID(s)
	if id == "" {
		return ""
	}
	return "spotify:track:" + id
}

func spotifyEdit(args json.RawMessage, add bool) (string, error) {
	var a struct {
		ID   string   `json:"id"`
		URIs []string `json:"uris"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	user, err := spotifyUser()
	if err != nil {
		return "", err
	}
	if err := tooMany(len(a.URIs), maxEdit, "songs"); err != nil {
		return "", err
	}
	id := spotifyID(a.ID)
	if !spotify.IsLiked(id) {
		lists, err := spotify.Playlists()
		if err != nil {
			return "", err
		}
		i := slices.IndexFunc(lists, func(p spotify.Playlist) bool { return p.ID == id })
		switch {
		case i < 0:
			return "", fmt.Errorf("no Spotify playlist %s among the user's; call spotify_playlists", id)
		case !lists[i].Editable(user):
			return "", errors.New("only the user's own or collaborative Spotify playlists can be changed")
		}
	}
	var uris []string
	seen := map[string]bool{}
	for _, u := range a.URIs {
		if u = trackURI(u); u != "" && !seen[u] {
			seen[u] = true
			uris = append(uris, u)
		}
	}
	if len(uris) == 0 {
		return "", errors.New("no track URIs")
	}
	if !add {
		if err := spotify.RemoveItems(id, uris); err != nil {
			return "", err
		}
		return fmt.Sprintf("removed %d songs\n", len(uris)), nil
	}
	have, err := spotify.PlaylistItems(id)
	if err != nil {
		return "", err
	}
	for _, t := range have {
		if seen[t.URI] {
			seen[t.URI] = false
		}
	}
	var fresh []string
	for _, u := range uris {
		if seen[u] {
			fresh = append(fresh, u)
		}
	}
	if len(fresh) > 0 {
		if err := spotify.AddItems(id, fresh); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("added %d songs (%d were there already)\n", len(fresh), len(uris)-len(fresh)), nil
}
