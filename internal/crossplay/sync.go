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
	// Removed is how many songs the listener took out of one side since
	// the last sync, which weren't put back.
	Removed int
	// Refused is how many songs NetEase turned away, as some it says the
	// playlist has.
	Refused int
	// Truncated says the NetEase playlist has more than the
	// netease.MaxTracks songs a sync can read, whose rest didn't go to Spotify.
	Truncated bool
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
// says. It only adds, never deletes; and a song the listener took out of
// one side since the last sync stays out. It finds every song first and
// adds none until it has, so that a sync cut short by its Budget
// (ErrIncomplete) adds nothing, and the next, finding what this one
// found, picks up there and adds in order.
func Sync(l Link, b Budget, progress Progress) (Report, error) {
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
	// All its songs, by id, those whose details didn't load too; NetEase turns a whole request away for one song the
	// playlist has.
	onNetease := map[int64]bool{}
	for _, t := range ne.TrackIDs {
		onNetease[t.ID] = true
	}
	for _, t := range ne.Tracks {
		onNetease[t.ID] = true
	}
	r.Truncated = len(ne.Tracks) >= netease.MaxTracks && len(ne.TrackIDs) > len(ne.Tracks)
	onSpotify := map[string]bool{}
	for _, t := range sp {
		onSpotify[t.URI] = true
	}
	seenNE := map[int64]bool{}
	for _, id := range l.SeenNetease {
		seenNE[id] = true
	}
	seenSP := map[string]bool{}
	for _, u := range l.SeenSpotify {
		seenSP[u] = true
	}
	toNetease := l.Direction == ToNetease || l.Direction == Both
	toSpotify := l.Direction == ToSpotify || l.Direction == Both

	// Find the songs, both ways, before adding any.
	// Both ways, the NetEase ids of Spotify's songs also tell which of
	// NetEase's songs Spotify has already, as another release than the
	// one its search finds.
	var fromSpotify []int64
	if toNetease {
		nb := b
		nb.Progress = func(done, total int) { progress.say("在网易云找歌", done, total) }
		if fromSpotify, err = NeteaseIDs(sp, nb); err != nil {
			return r, err
		}
	}
	var todo []netease.Track
	var uris []string
	if toSpotify {
		covered := map[int64]bool{}
		for _, id := range fromSpotify {
			covered[id] = true
		}
		for _, t := range ne.Tracks {
			if !covered[t.ID] {
				todo = append(todo, t)
			}
		}
		sb := b
		sb.Progress = func(done, total int) { progress.say("在 Spotify 找歌", done, total) }
		if uris, err = SpotifyURIs(todo, sb); err != nil {
			return r, err
		}
	}

	var addNE []int64
	if toNetease {
		seen := map[int64]bool{}
		for i, id := range fromSpotify {
			switch {
			case id == 0:
				r.MissingOnNetease = append(r.MissingOnNetease, sp[i].Name+" - "+sp[i].ArtistNames())
			case onNetease[id] || seen[id]:
			case seenNE[id]:
				r.Removed++
			default:
				seen[id] = true
				addNE = append(addNE, id)
			}
		}
	}
	var addSP []string
	if toSpotify {
		for i, u := range uris {
			switch {
			case u == "":
				r.MissingOnSpotify = append(r.MissingOnSpotify, todo[i].Name+" - "+todo[i].ArtistNames())
			case onSpotify[u]:
			case seenSP[u]:
				r.Removed++
			default:
				onSpotify[u] = true
				addSP = append(addSP, u)
			}
		}
	}

	if len(addNE) > 0 {
		progress.say("添加到网易云", 0, len(addNE))
		added, refused, err := addToNetease(l.Netease, addNE)
		r.AddedToNetease, r.Refused = len(added), refused
		for _, id := range added {
			onNetease[id] = true
		}
		if err != nil {
			l.SeenNetease = keys(onNetease)
			SetLink(l)
			return r, err
		}
	}
	if len(addSP) > 0 {
		progress.say("添加到 Spotify", 0, len(addSP))
		if err := spotifyAdd(l.Spotify, addSP); err != nil {
			return r, err
		}
		r.AddedToSpotify = len(addSP)
	}
	l.Synced = time.Now()
	l.SeenNetease = keys(onNetease)
	l.SeenSpotify = keys(onSpotify)
	return r, SetLink(l)
}

func keys[K comparable](m map[K]bool) []K {
	out := make([]K, 0, len(m))
	for k, on := range m {
		if on {
			out = append(out, k)
		}
	}
	return out
}

// addToNetease adds songs to playlist pid in the order given: NetEase
// puts each song it adds at the top, one request's too, so they go last
// first, a chunk at a time. It returns those added, and how many a chunk
// NetEase turned away as already there held.
func addToNetease(pid int64, songs []int64) (added []int64, refused int, err error) {
	rev := make([]int64, len(songs))
	for i, s := range songs {
		rev[len(songs)-1-i] = s
	}
	for i := 0; i < len(rev); i += neteaseChunk {
		if i > 0 {
			time.Sleep(neteaseAddPause)
		}
		chunk := rev[i:min(i+neteaseChunk, len(rev))]
		switch err := neteaseAdd(pid, chunk...); {
		case errors.Is(err, netease.ErrInPlaylist):
			refused += len(chunk)
		case err != nil:
			return added, refused, err
		default:
			added = append(added, chunk...)
		}
	}
	return added, refused, nil
}

// ImportToNetease makes a NetEase playlist of the songs of Spotify
// playlist sp, named name, and pairs the two. It reads the Spotify
// playlist first, so that one it can't read leaves nothing behind.
func ImportToNetease(sp, name string, b Budget, progress Progress) (Link, Report, error) {
	if _, err := spotifyItems(sp); err != nil {
		return Link{}, Report{}, err
	}
	pl, err := neteaseCreate(name)
	if err != nil {
		return Link{}, Report{}, err
	}
	l := Link{Spotify: sp, Netease: pl.ID, Name: name, Direction: ToNetease}
	if err := SetLink(l); err != nil {
		return l, Report{}, err
	}
	r, err := Sync(l, b, progress)
	return l, r, err
}

// LinkTo pairs Spotify playlist sp with the listener's NetEase playlist
// ne, which they had already, and syncs them the way d says.
func LinkTo(sp string, ne int64, name string, d Direction, b Budget, progress Progress) (Link, Report, error) {
	l := Link{Spotify: sp, Netease: ne, Name: name, Direction: d}
	if old, ok := linkOf(sp); ok && old.Netease == ne {
		// The same pair again: what was seen of it still holds.
		old.Direction = d
		l = old
	}
	if err := SetLink(l); err != nil {
		return l, Report{}, err
	}
	r, err := Sync(l, b, progress)
	return l, r, err
}

func linkOf(sp string) (Link, bool) {
	for _, l := range Links() {
		if l.Spotify == sp {
			return l, true
		}
	}
	return Link{}, false
}

// ExportToSpotify makes a Spotify playlist of the songs of NetEase
// playlist id, named name, and pairs the two.
func ExportToSpotify(id int64, name string, b Budget, progress Progress) (Link, Report, error) {
	if _, err := neteaseDetail(id); err != nil {
		return Link{}, Report{}, err
	}
	pl, err := spotifyCreate(name, "From NetEase playlist "+strconv.FormatInt(id, 10)+", by DuckEase Music")
	if err != nil {
		return Link{}, Report{}, err
	}
	l := Link{Spotify: pl.ID, Netease: id, Name: name, Direction: ToSpotify}
	if err := SetLink(l); err != nil {
		return l, Report{}, err
	}
	r, err := Sync(l, b, progress)
	return l, r, err
}
