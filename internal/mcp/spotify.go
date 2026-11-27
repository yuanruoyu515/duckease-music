package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/spotify"
)

// Tools for the listener's Spotify playlists, and for keeping them in
// step with NetEase. Connecting to Spotify happens in the app's settings.

func spotifyTools() []tool {
	return []tool{
		{
			name: "spotify_playlists", title: "List Spotify playlists", readOnly: true,
			description: "The user's Spotify playlists. Lines: id, name, songs, kind (mine, collab, followed = songs can't be read), linked NetEase playlist id or -.",
			run:         func(json.RawMessage) (string, error) { return spotifyPlaylists() },
		},
		{
			name: "spotify_playlist", title: "Read a Spotify playlist", readOnly: true,
			description: "Songs of one of the user's own or collaborative Spotify playlists. Lines: n, title, artists, length. Pass query to filter.",
			params:      map[string]any{"id": strParam("Spotify playlist id"), "query": strParam("optional filter")},
			required:    []string{"id"},
			run:         spotifyPlaylist,
		},
		{
			name: "spotify_import", title: "Copy a Spotify playlist to NetEase",
			description: "Create a NetEase playlist with the songs of a Spotify playlist, found by title, artist and length, and link the two. Slow: about 0.3 s per song not matched before. Lists the songs NetEase lacks.",
			params:      map[string]any{"id": strParam("Spotify playlist id"), "name": strParam("NetEase playlist name; default the Spotify one's")},
			required:    []string{"id"},
			run:         spotifyImport,
		},
		{
			name: "spotify_export", title: "Copy a NetEase playlist to Spotify",
			description: "Create a private Spotify playlist with the songs of a NetEase playlist and link the two. Lists the songs Spotify lacks.",
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
			description: "Bring songs across a linked pair (to-netease, to-spotify or both; default the pair's). Only adds songs, never removes. Pass netease_id to link a new pair first.",
			params: map[string]any{
				"spotify_id": strParam("Spotify playlist id"),
				"netease_id": intParam("NetEase playlist id, to link a new pair"),
				"direction":  map[string]any{"type": "string", "enum": []string{"to-netease", "to-spotify", "both"}},
			},
			required: []string{"spotify_id"},
			run:      syncPlaylist,
		},
	}
}

func spotifyUser() (string, error) {
	id, _ := spotify.Account()
	if id == "" {
		return "", errors.New("Spotify isn't connected: connect it in DuckEase Music's settings first")
	}
	return id, nil
}

func linkOf(spotifyID string) (crossplay.Link, bool) {
	crossplay.Reload()
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
	var b strings.Builder
	b.WriteString("id\tname\tsongs\tkind\tnetease\n")
	for _, p := range lists {
		kind := "followed"
		switch {
		case p.Owner.ID == user:
			kind = "mine"
		case p.Collaborative:
			kind = "collab"
		}
		linked := "-"
		if l, ok := linkOf(p.ID); ok {
			linked = fmt.Sprint(l.Netease)
		}
		fmt.Fprintf(&b, "%s\t%s\t%d\t%s\t%s\n", p.ID, clean(p.Name), p.Total(), kind, linked)
	}
	return b.String(), nil
}

func spotifyPlaylist(args json.RawMessage) (string, error) {
	var a struct{ ID, Query string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	tracks, err := spotify.PlaylistItems(a.ID)
	if err != nil {
		return "", err
	}
	q := strings.ToLower(a.Query)
	var b strings.Builder
	fmt.Fprintf(&b, "%d songs\nn\ttitle\tartists\tlength\n", len(tracks))
	for i, t := range tracks {
		if q != "" && !strings.Contains(strings.ToLower(t.Name+"\x00"+t.ArtistNames()+"\x00"+t.Album.Name), q) {
			continue
		}
		secs := t.Duration / 1000
		fmt.Fprintf(&b, "%d\t%s\t%s\t%d:%02d\n", i+1, clean(t.Name), clean(t.ArtistNames()), secs/60, secs%60)
	}
	return b.String(), nil
}

func report(r crossplay.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "added %d to NetEase, %d to Spotify\n", r.AddedToNetease, r.AddedToSpotify)
	if len(r.MissingOnNetease) > 0 {
		fmt.Fprintf(&b, "not found on NetEase (%d):\n%s\n", len(r.MissingOnNetease), clean2(r.MissingOnNetease))
	}
	if len(r.MissingOnSpotify) > 0 {
		fmt.Fprintf(&b, "not found on Spotify (%d):\n%s\n", len(r.MissingOnSpotify), clean2(r.MissingOnSpotify))
	}
	return b.String()
}

func clean2(lines []string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = clean(l)
	}
	return strings.Join(out, "\n")
}

// failed says how far a sync got before err.
func failed(r crossplay.Report, err error) error {
	if crossplay.Blocked(err) {
		return fmt.Errorf("NetEase is refusing requests (-460); songs matched so far are remembered, run it again in a few minutes. %s", r)
	}
	return fmt.Errorf("%w (%s)", err, r)
}

func spotifyImport(args json.RawMessage) (string, error) {
	var a struct{ ID, Name string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	if _, err := me(); err != nil {
		return "", err
	}
	if l, ok := linkOf(a.ID); ok {
		return "", fmt.Errorf("already linked to NetEase playlist %d; use sync_playlist", l.Netease)
	}
	if a.Name == "" {
		lists, err := spotify.Playlists()
		if err != nil {
			return "", err
		}
		for _, p := range lists {
			if p.ID == a.ID {
				a.Name = p.Name
			}
		}
		if a.Name == "" {
			return "", fmt.Errorf("no Spotify playlist %s among the user's", a.ID)
		}
	}
	l, r, err := crossplay.ImportToNetease(a.ID, a.Name, nil)
	changed(l.Netease)
	if err != nil {
		return "", failed(r, err)
	}
	return fmt.Sprintf("NetEase playlist %d %s\n%s", l.Netease, clean(a.Name), report(r)), nil
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
	if a.Name == "" {
		pl, err := playlist(int64(a.ID))
		if err != nil {
			return "", err
		}
		a.Name = pl.Name
	}
	l, r, err := crossplay.ExportToSpotify(int64(a.ID), a.Name, nil)
	if err != nil {
		return "", failed(r, err)
	}
	return fmt.Sprintf("Spotify playlist %s %s\n%s", l.Spotify, clean(a.Name), report(r)), nil
}

func syncLinks() (string, error) {
	crossplay.Reload()
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
	if _, err := spotifyUser(); err != nil {
		return "", err
	}
	l, ok := linkOf(a.Spotify)
	if a.Netease != 0 {
		if _, err := own(int64(a.Netease)); err != nil {
			return "", err
		}
		l = crossplay.Link{Spotify: a.Spotify, Netease: int64(a.Netease), Direction: crossplay.Both, Name: l.Name}
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
	r, err := crossplay.Sync(l, nil)
	changed(l.Netease)
	if err != nil {
		return "", failed(r, err)
	}
	return report(r), nil
}
