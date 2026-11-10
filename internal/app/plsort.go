package app

import (
	"cmp"
	"math/rand/v2"
	"slices"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"netease-music-mygo/internal/netease"
)

// The orders a playlist's songs may be listed and played in.
const (
	sortDefault = iota
	sortReverse
	sortName
	sortArtist
	sortAlbum
	sortShortest
	sortLongest
	sortLucky // shuffled anew each time it is chosen
)

// plSorts name the orders, as the sort button's menu lists them; the
// lucky one has a button of its own.
var plSorts = []string{"默认排序", "倒序", "按歌名", "按歌手", "按专辑", "时长从短到长", "时长从长到短", "手气顺序"}

// collator orders names as the listener reads them: Chinese by pinyin,
// among Latin letters, any case. Only the UI thread uses it.
var collator = collate.New(language.Chinese, collate.IgnoreCase, collate.Loose)

// plOrder keeps a playlist's songs in the order chosen, worked out once
// for each order and each list of songs, not on every frame.
type plOrder struct {
	by     int
	luck   uint64 // the lucky order's seed
	of     *netease.Track
	n      int
	sorted []netease.Track
}

// tracks returns tracks in the order chosen: tracks itself by default.
func (o *plOrder) tracks(tracks []netease.Track, by int, luck uint64) []netease.Track {
	if by == sortDefault || len(tracks) < 2 {
		return tracks
	}
	if o.sorted != nil && o.by == by && o.luck == luck && o.of == &tracks[0] && o.n == len(tracks) {
		return o.sorted
	}
	sorted := slices.Clone(tracks)
	switch by {
	case sortReverse:
		slices.Reverse(sorted)
	case sortName:
		sortBy(sorted, func(t *netease.Track) string { return t.Name })
	case sortArtist:
		sortBy(sorted, func(t *netease.Track) string { return t.ArtistNames() })
	case sortAlbum:
		sortBy(sorted, func(t *netease.Track) string { return t.Album.Name })
	case sortShortest, sortLongest:
		slices.SortStableFunc(sorted, func(a, b netease.Track) int {
			if by == sortLongest {
				return cmp.Compare(b.Duration, a.Duration)
			}
			return cmp.Compare(a.Duration, b.Duration)
		})
	case sortLucky:
		r := rand.New(rand.NewPCG(luck, luck>>1|1))
		r.Shuffle(len(sorted), func(i, j int) { sorted[i], sorted[j] = sorted[j], sorted[i] })
	}
	*o = plOrder{by: by, luck: luck, of: &tracks[0], n: len(tracks), sorted: sorted}
	return sorted
}

// sortBy sorts tracks by the key of each, as collator orders them, those
// of the same key in the order they were.
func sortBy(tracks []netease.Track, key func(*netease.Track) string) {
	keys := make(map[int64][]byte, len(tracks))
	var buf collate.Buffer
	for i := range tracks {
		k := collator.KeyFromString(&buf, key(&tracks[i]))
		keys[tracks[i].ID] = slices.Clone(k)
		buf.Reset()
	}
	slices.SortStableFunc(tracks, func(a, b netease.Track) int {
		return slices.Compare(keys[a.ID], keys[b.ID])
	})
}
