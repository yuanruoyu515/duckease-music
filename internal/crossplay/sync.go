package crossplay

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

// The calls a sync makes to either service; tests replace them.
var (
	spotifyItems    = spotify.PlaylistItems
	spotifyAdd      = spotify.AddItems
	spotifyCreate   = spotify.CreatePlaylist
	neteaseDetail   = netease.PlaylistDetail
	neteaseAdd      = netease.AddToPlaylist
	neteaseCreate   = netease.CreatePlaylist
	neteaseChunk    = 200
	neteaseAddPause = 400 * time.Millisecond
)

// Report is what a sync did.
type Report struct {
	AddedToNetease int
	AddedToSpotify int
	// Missing are the songs found on one side only: "title - artists".
	MissingOnNetease []string
	MissingOnSpotify []string
}

func (r Report) String() string {
	return fmt.Sprintf("+%d NetEase, +%d Spotify; %d not on NetEase, %d not on Spotify",
		r.AddedToNetease, r.AddedToSpotify, len(r.MissingOnNetease), len(r.MissingOnSpotify))
}

// Progress hears how a sync goes: what it does, and how far.
type Progress func(step string, done, total int)

func (p Progress) say(step string, done, total int) {
	if p != nil {
		p(step, done, total)
	}
}

// Sync brings the songs of l's playlists across, the way l.Direction
// says. It only adds: a song taken out of one side stays on the other,
// so that a mistake or a match gone wrong never deletes anything.
func Sync(l Link, progress Progress) (Report, error) {
	var r Report
	progress.say("读取 Spotify 歌单", 0, 0)
	sp, err := spotifyItems(l.Spotify)
	if err != nil {
		return r, err
	}
	progress.say("读取网易云歌单", 0, 0)
	ne, err := neteaseDetail(l.Netease)
	if err != nil {
		return r, err
	}
	onNetease := map[int64]bool{}
	for _, t := range ne.Tracks {
		onNetease[t.ID] = true
	}

	// The NetEase ids of Spotify's songs: needed to bring them to NetEase,
	// and, both ways, to know which of NetEase's songs Spotify has
	// already, though as another release than the one search finds.
	var fromSpotify []int64
	if l.Direction == ToNetease || l.Direction == Both {
		fromSpotify, err = NeteaseIDs(sp, func(done, total int) { progress.say("在网易云找歌", done, total) })
		if err != nil {
			return r, err
		}
		var add []int64
		seen := map[int64]bool{}
		for i, id := range fromSpotify {
			switch {
			case id == 0:
				r.MissingOnNetease = append(r.MissingOnNetease, sp[i].Name+" - "+sp[i].ArtistNames())
			case !onNetease[id] && !seen[id]:
				seen[id] = true
				add = append(add, id)
			}
		}
		if len(add) > 0 {
			progress.say("添加到网易云", 0, len(add))
			if err := addToNetease(l.Netease, add); err != nil {
				return r, err
			}
			r.AddedToNetease = len(add)
		}
	}

	if l.Direction == ToSpotify || l.Direction == Both {
		onSpotify := map[string]bool{}
		for _, t := range sp {
			onSpotify[t.URI] = true
		}
		covered := map[int64]bool{}
		for _, id := range fromSpotify {
			covered[id] = true
		}
		var todo []netease.Track
		for _, t := range ne.Tracks {
			if !covered[t.ID] {
				todo = append(todo, t)
			}
		}
		uris, err := SpotifyURIs(todo, func(done, total int) { progress.say("在 Spotify 找歌", done, total) })
		if err != nil {
			return r, err
		}
		var add []string
		for i, u := range uris {
			switch {
			case u == "":
				r.MissingOnSpotify = append(r.MissingOnSpotify, todo[i].Name+" - "+todo[i].ArtistNames())
			case !onSpotify[u]:
				onSpotify[u] = true
				add = append(add, u)
			}
		}
		if len(add) > 0 {
			progress.say("添加到 Spotify", 0, len(add))
			if err := spotifyAdd(l.Spotify, add); err != nil {
				return r, err
			}
			r.AddedToSpotify = len(add)
		}
	}
	l.Synced = time.Now()
	SetLink(l)
	return r, Save()
}

// addToNetease adds songs to playlist pid in the order given: NetEase
// puts what it adds at the top, so they go last first, a chunk at a time.
func addToNetease(pid int64, songs []int64) error {
	rev := make([]int64, len(songs))
	for i, s := range songs {
		rev[len(songs)-1-i] = s
	}
	for i := 0; i < len(rev); i += neteaseChunk {
		if i > 0 {
			time.Sleep(neteaseAddPause)
		}
		err := neteaseAdd(pid, rev[i:min(i+neteaseChunk, len(rev))]...)
		if err != nil && !errors.Is(err, netease.ErrInPlaylist) {
			return err
		}
	}
	return nil
}

// ImportToNetease makes a NetEase playlist of the songs of Spotify
// playlist sp, named name, and pairs the two.
func ImportToNetease(sp, name string, progress Progress) (Link, Report, error) {
	pl, err := neteaseCreate(name)
	if err != nil {
		return Link{}, Report{}, err
	}
	l := Link{Spotify: sp, Netease: pl.ID, Name: name, Direction: ToNetease}
	SetLink(l)
	r, err := Sync(l, progress)
	return l, r, err
}

// ExportToSpotify makes a Spotify playlist of the songs of NetEase
// playlist id, named name, and pairs the two.
func ExportToSpotify(id int64, name string, progress Progress) (Link, Report, error) {
	pl, err := spotifyCreate(name, "From NetEase playlist "+strconv.FormatInt(id, 10)+", by DuckEase Music")
	if err != nil {
		return Link{}, Report{}, err
	}
	l := Link{Spotify: pl.ID, Netease: id, Name: name, Direction: ToSpotify}
	SetLink(l)
	r, err := Sync(l, progress)
	return l, r, err
}
