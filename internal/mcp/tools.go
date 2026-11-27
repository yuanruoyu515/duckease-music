package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/match"
	"netease-music-mygo/internal/netease"
)

// The NetEase calls the tools make; tests replace them.
var (
	account         = netease.Account
	restoreSession  = netease.RestoreSession
	userPlaylists   = netease.UserPlaylists
	playlistDetail  = netease.PlaylistDetail
	searchSongs     = netease.Search
	createPlaylist  = netease.CreatePlaylist
	renamePlaylist  = netease.RenamePlaylist
	deletePlaylist  = netease.DeletePlaylist
	addToPlaylist   = netease.AddToPlaylist
	removeFromPlist = netease.RemoveFromPlaylist
	likeSong        = netease.Like
)

// pause is the wait between requests of a batch, as the app's bulk edits
// wait, so that NetEase's risk control doesn't see a burst. netease's
// guard also spaces edits and searches, across calls and processes.
var pause = 400 * time.Millisecond

// Most songs one call may take: an agent splits more into several calls,
// which netease's guard spaces out.
const (
	maxMatch = 100
	maxAdd   = 1000
	maxEdit  = 200 // removals and likes, one request each or in one go
)

// tooMany says a call asked for more than it may.
func tooMany(n, most int, what string) error {
	if n > most {
		return fmt.Errorf("%d %s; send at most %d per call and do the rest in later calls", n, what, most)
	}
	return nil
}

// chunk is how many songs go to NetEase in one request.
const chunk = 200

// cacheFor is how long a playlist read stays good: an agent reads a
// playlist, then works on it, and the app may change it meanwhile.
const cacheFor = time.Minute

type tool struct {
	name, title, description string
	params                   map[string]any // properties of the input schema
	required                 []string
	readOnly, destructive    bool
	run                      func(args json.RawMessage) (string, error)
}

func (t tool) describe() map[string]any {
	schema := map[string]any{"type": "object", "properties": t.params}
	if len(t.required) > 0 {
		schema["required"] = t.required
	}
	if t.params == nil {
		schema["properties"] = map[string]any{}
	}
	return map[string]any{
		"name":        t.name,
		"title":       t.title,
		"description": t.description,
		"inputSchema": schema,
		"annotations": map[string]any{
			"readOnlyHint":    t.readOnly,
			"destructiveHint": t.destructive,
			"idempotentHint":  t.readOnly,
			"openWorldHint":   true,
		},
	}
}

// id is a NetEase id an agent may send as a number or a string.
type id int64

func (i *id) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	// A link, as music.163.com/#/playlist?id=123&userid=4: its id
	// parameter, not userid's.
	if _, query, ok := strings.Cut(s, "?"); ok {
		if v, err := url.ParseQuery(query); err == nil && v.Get("id") != "" {
			s = v.Get("id")
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("not an id: %s", b)
	}
	*i = id(n)
	return nil
}

func ids(in []id) []int64 {
	out := make([]int64, len(in))
	for i, x := range in {
		out[i] = int64(x)
	}
	return out
}

var (
	intParam = func(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }
	strParam = func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	idsParam = func(d string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": d}
	}
	boolParam   = func(d string) map[string]any { return map[string]any{"type": "boolean", "description": d} }
	playlistArg = intParam("playlist id")
)

func newTools() []tool {
	return append(neteaseTools(), spotifyTools()...)
}

func neteaseTools() []tool {
	return []tool{
		{
			name: "playlists", title: "List playlists", readOnly: true,
			description: "The user's playlists. Lines: id, name, songs, kind (liked = 我喜欢的音乐, mine, saved = made by others, read-only).",
			run:         func(json.RawMessage) (string, error) { return listPlaylists() },
		},
		{
			name: "playlist", title: "Read a playlist", readOnly: true,
			description: "Songs of a playlist, in order (first 1000 at most). Pass query to get only songs whose title, artist or album contains it.",
			params: map[string]any{
				"id":     playlistArg,
				"query":  strParam("optional filter, case-insensitive"),
				"offset": intParam("skip this many songs (default 0)"),
				"limit":  intParam("at most this many songs (default 300)"),
			},
			required: []string{"id"},
			run:      readPlaylist,
		},
		{
			name: "search", title: "Search songs", readOnly: true,
			description: "Search NetEase for songs.",
			params: map[string]any{
				"query": strParam("keywords, e.g. title and artist"),
				"limit": intParam("results, default 8, at most 30"),
			},
			required: []string{"query"},
			run:      search,
		},
		{
			name: "match", title: "Find songs by name", readOnly: true,
			description: "Find the NetEase song for each of many songs at once, e.g. to import a list. Each line is \"title - artist[, artist]\". Lines out: n (1-based), id, score (0-1), title, artists; id \"-\" when nothing matched well. Feed the ids to add_songs.",
			params: map[string]any{
				"songs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "\"title - artist\" lines, up to 100"},
			},
			required: []string{"songs"},
			run:      matchSongs,
		},
		{
			name: "duplicates", title: "Find duplicate songs", readOnly: true,
			description: "Songs of a playlist that appear more than once, by id or by the same title (versions such as Live count apart) and first artist. Lines: id, title, artists; groups split by blank lines, the first of each group is the one listed first.",
			params:      map[string]any{"id": playlistArg},
			required:    []string{"id"},
			run:         duplicates,
		},
		{
			name: "create_playlist", title: "Create a playlist",
			description: "Create a playlist of the user's, optionally with songs. Returns its id.",
			params: map[string]any{
				"name":     strParam("name"),
				"song_ids": idsParam("songs to add, optional"),
			},
			required: []string{"name"},
			run:      create,
		},
		{
			name: "add_songs", title: "Add songs to a playlist",
			description: "Add songs to one of the user's own playlists (they go to its top). Songs it already has are skipped.",
			params:      map[string]any{"id": playlistArg, "song_ids": idsParam("songs to add")},
			required:    []string{"id", "song_ids"},
			run:         func(a json.RawMessage) (string, error) { return edit(a, true) },
		},
		{
			name: "remove_songs", title: "Remove songs from a playlist", destructive: true,
			description: "Remove songs from one of the user's own playlists.",
			params:      map[string]any{"id": playlistArg, "song_ids": idsParam("songs to remove")},
			required:    []string{"id", "song_ids"},
			run:         func(a json.RawMessage) (string, error) { return edit(a, false) },
		},
		{
			name: "rename_playlist", title: "Rename a playlist",
			description: "Rename one of the user's own playlists.",
			params:      map[string]any{"id": playlistArg, "name": strParam("new name")},
			required:    []string{"id", "name"},
			run:         rename,
		},
		{
			name: "delete_playlist", title: "Delete a playlist", destructive: true,
			description: "Delete one of the user's own playlists, for good. 我喜欢的音乐 can't be deleted.",
			params:      map[string]any{"id": playlistArg},
			required:    []string{"id"},
			run:         remove,
		},
		{
			name: "like_songs", title: "Like or unlike songs",
			description: "Add songs to or take them out of 我喜欢的音乐.",
			params: map[string]any{
				"song_ids": idsParam("songs"),
				"like":     boolParam("true to like (default), false to unlike"),
			},
			required: []string{"song_ids"},
			run:      like,
		},
	}
}

// cache keeps what was read of NetEase for cacheFor.
var cache struct {
	sync.Mutex
	uid       int64
	nick      string
	uidAt     time.Time
	lists     []netease.Playlist // made, then saved
	made      map[int64]bool
	listsAt   time.Time
	playlists map[int64]cached
}

type cached struct {
	pl *netease.Playlist
	at time.Time
}

// changed forgets what was read of playlist pid, and of the list of
// playlists, whose counts it changes.
func changed(pid int64) {
	cache.Lock()
	defer cache.Unlock()
	delete(cache.playlists, pid)
	cache.listsAt = time.Time{}
}

func me() (int64, error) {
	cache.Lock()
	uid, at := cache.uid, cache.uidAt
	cache.Unlock()
	if uid != 0 && time.Since(at) < 5*cacheFor {
		return uid, nil
	}
	p, err := account()
	if err == nil && p == nil {
		// Signed in, or out, in the app since this server started.
		restoreSession()
		p, err = account()
	}
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, errors.New("not signed in: open DuckEase Music and sign in to NetEase first")
	}
	cache.Lock()
	if cache.uid != p.UserID { // another account: forget the other's
		cache.listsAt, cache.playlists = time.Time{}, nil
	}
	cache.uid, cache.nick, cache.uidAt = p.UserID, p.Nickname, time.Now()
	cache.Unlock()
	return p.UserID, nil
}

func playlists() ([]netease.Playlist, map[int64]bool, error) {
	cache.Lock()
	if time.Since(cache.listsAt) < cacheFor {
		defer cache.Unlock()
		return cache.lists, cache.made, nil
	}
	cache.Unlock()
	uid, err := me()
	if err != nil {
		return nil, nil, err
	}
	made, saved, err := userPlaylists(uid)
	if err != nil {
		return nil, nil, err
	}
	mine := map[int64]bool{}
	for _, p := range made {
		mine[p.ID] = true
	}
	cache.Lock()
	defer cache.Unlock()
	cache.lists = append(made, saved...)
	cache.made = mine
	cache.listsAt = time.Now()
	return cache.lists, mine, nil
}

func playlist(pid int64) (*netease.Playlist, error) {
	cache.Lock()
	if c, ok := cache.playlists[pid]; ok && time.Since(c.at) < cacheFor {
		cache.Unlock()
		return c.pl, nil
	}
	cache.Unlock()
	pl, err := playlistDetail(pid)
	if err != nil {
		return nil, err
	}
	cache.Lock()
	defer cache.Unlock()
	if cache.playlists == nil {
		cache.playlists = map[int64]cached{}
	}
	cache.playlists[pid] = cached{pl, time.Now()}
	return pl, nil
}

// own returns the user's playlist pid, or an error saying it isn't theirs.
func own(pid int64) (netease.Playlist, error) {
	lists, mine, err := playlists()
	if err != nil {
		return netease.Playlist{}, err
	}
	for _, p := range lists {
		if p.ID == pid {
			if !mine[pid] {
				return p, fmt.Errorf("playlist %d (%s) was made by someone else; only the user's own playlists can be changed", pid, p.Name)
			}
			return p, nil
		}
	}
	return netease.Playlist{}, fmt.Errorf("playlist %d isn't one of the user's; call playlists for their ids", pid)
}

// clean keeps a field to one line without tabs.
func clean(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", "").Replace(s)
}

func songLine(b *strings.Builder, t *netease.Track) {
	if !t.Playable() {
		b.WriteByte('!')
	}
	secs := t.Duration / 1000
	fmt.Fprintf(b, "%d\t%s\t%s\t%s\t%d:%02d\n", t.ID, clean(t.Name), clean(t.ArtistNames()), clean(t.Album.Name), secs/60, secs%60)
}

func listPlaylists() (string, error) {
	lists, mine, err := playlists()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	cache.Lock()
	fmt.Fprintf(&b, "user %d %s\nid\tname\tsongs\tkind\n", cache.uid, clean(cache.nick))
	cache.Unlock()
	for _, p := range lists {
		kind := "saved"
		switch {
		case p.SpecialType == 5:
			kind = "liked"
		case mine[p.ID]:
			kind = "mine"
		}
		fmt.Fprintf(&b, "%d\t%s\t%d\t%s\n", p.ID, clean(p.Name), p.TrackCount, kind)
	}
	return b.String(), nil
}

func readPlaylist(args json.RawMessage) (string, error) {
	var a struct {
		ID     id     `json:"id"`
		Query  string `json:"query"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Limit <= 0 {
		a.Limit = 300
	}
	pl, err := playlist(int64(a.ID))
	if err != nil {
		return "", err
	}
	q := strings.ToLower(strings.TrimSpace(a.Query))
	var picked []*netease.Track
	for i := range pl.Tracks {
		t := &pl.Tracks[i]
		if q == "" || strings.Contains(strings.ToLower(t.Name+"\x00"+t.ArtistNames()+"\x00"+t.Album.Name+"\x00"+strings.Join(t.Alias, " ")), q) {
			picked = append(picked, t)
		}
	}
	total := len(picked)
	from := min(max(a.Offset, 0), total)
	to := min(from+a.Limit, total)
	var b strings.Builder
	fmt.Fprintf(&b, "playlist %d %s: %d songs", pl.ID, clean(pl.Name), pl.TrackCount)
	if q != "" {
		fmt.Fprintf(&b, ", %d match %q", total, a.Query)
	}
	if from > 0 || to < total {
		fmt.Fprintf(&b, ", showing %d-%d", from+1, to)
	}
	if len(pl.Tracks) >= 1000 && pl.TrackCount > len(pl.Tracks) {
		fmt.Fprintf(&b, " (only the first %d can be read)", len(pl.Tracks))
	}
	b.WriteString("\nid\ttitle\tartists\talbum\tlength\n")
	for _, t := range picked[from:to] {
		songLine(&b, t)
	}
	return b.String(), nil
}

func search(args json.RawMessage) (string, error) {
	var a struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Query) == "" {
		return "", errors.New("empty query")
	}
	if a.Limit <= 0 {
		a.Limit = 8
	}
	songs, err := searchSongs(a.Query, min(a.Limit, 30))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("id\ttitle\tartists\talbum\tlength\n")
	for i := range songs {
		songLine(&b, &songs[i])
	}
	if len(songs) == 0 {
		b.WriteString("(nothing found)\n")
	}
	return b.String(), nil
}

func matchSongs(args json.RawMessage) (string, error) {
	var a struct {
		Songs []string `json:"songs"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if err := tooMany(len(a.Songs), maxMatch, "songs"); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("n\tid\tscore\ttitle\tartists\n")
	found := 0
	for i, line := range a.Songs {
		t, score, err := crossplay.FindNetease(match.Parse(line))
		switch {
		case err != nil:
			fmt.Fprintf(&b, "%d\t-\t\terror: %s\n", i+1, clean(err.Error()))
			if blocked, _ := netease.Blocked(err); blocked {
				fmt.Fprintf(&b, "stopped at line %d: %s\n", i+1, coolingNote(err))
				return b.String(), nil
			}
		case t == nil:
			fmt.Fprintf(&b, "%d\t-\t\tnot found\n", i+1)
		default:
			found++
			fmt.Fprintf(&b, "%d\t%d\t%.2f\t%s\t%s\n", i+1, t.ID, score, clean(t.Name), clean(t.ArtistNames()))
		}
	}
	fmt.Fprintf(&b, "matched %d/%d\n", found, len(a.Songs))
	return b.String(), nil
}

func duplicates(args json.RawMessage) (string, error) {
	var a struct {
		ID id `json:"id"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	pl, err := playlist(int64(a.ID))
	if err != nil {
		return "", err
	}
	groups := map[string][]*netease.Track{}
	var order []string
	for i := range pl.Tracks {
		t := &pl.Tracks[i]
		artist := ""
		if len(t.Artists) > 0 {
			artist = match.Clean(t.Artists[0].Name)
		}
		// The whole title, versions too: a live take isn't a duplicate.
		key := match.Clean(t.Name) + "\x00" + artist
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], t)
	}
	var b strings.Builder
	n := 0
	for _, k := range order {
		g := groups[k]
		if len(g) < 2 {
			continue
		}
		n++
		if n > 1 {
			b.WriteByte('\n')
		}
		for _, t := range g {
			fmt.Fprintf(&b, "%d\t%s\t%s\n", t.ID, clean(t.Name), clean(t.ArtistNames()))
		}
	}
	if n == 0 {
		return fmt.Sprintf("no duplicates in %d songs\n", len(pl.Tracks)), nil
	}
	return fmt.Sprintf("%d groups of duplicates\n%s", n, b.String()), nil
}

// inChunks sends songs to NetEase chunk at a time, pause apart.
// It returns how many went, and how many a chunk NetEase turned away as
// already there held.
func inChunks(songs []int64, send func([]int64) error) (sent, refused int, err error) {
	for i := 0; i < len(songs); i += chunk {
		if i > 0 {
			time.Sleep(pause)
		}
		c := songs[i:min(i+chunk, len(songs))]
		switch err := send(c); {
		case errors.Is(err, netease.ErrInPlaylist):
			refused += len(c)
		case err != nil:
			return sent, refused, err
		default:
			sent += len(c)
		}
	}
	return sent, refused, nil
}

// fresh drops the songs pid already has, so that adding them doesn't fail
// the chunk.
func fresh(pid int64, songs []int64) ([]int64, error) {
	pl, err := playlist(pid)
	if err != nil {
		return nil, err
	}
	// All its songs, past the first 1000 too, by id: NetEase turns a
	// whole request away for one song the playlist has.
	has := map[int64]bool{}
	for _, t := range pl.TrackIDs {
		has[t.ID] = true
	}
	for _, t := range pl.Tracks {
		has[t.ID] = true
	}
	var out []int64
	seen := map[int64]bool{}
	for _, s := range songs {
		if !has[s] && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out, nil
}

// unique drops repeats from songs, keeping the first of each.
func unique(songs []int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, s := range songs {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// reversed returns songs last first: NetEase puts each song it adds at
// the top, those of one request too, so they go last first to keep the
// order given.
func reversed(songs []int64) []int64 {
	out := make([]int64, len(songs))
	for i, s := range songs {
		out[len(songs)-1-i] = s
	}
	return out
}

func create(args json.RawMessage) (string, error) {
	var a struct {
		Name  string `json:"name"`
		Songs []id   `json:"song_ids"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Name) == "" {
		return "", errors.New("empty name")
	}
	if err := tooMany(len(a.Songs), maxAdd, "songs"); err != nil {
		return "", err
	}
	pl, err := createPlaylist(strings.TrimSpace(a.Name))
	if err != nil {
		return "", err
	}
	changed(pl.ID)
	if len(a.Songs) == 0 {
		return fmt.Sprintf("created %d %s\n", pl.ID, clean(pl.Name)), nil
	}
	songs := unique(ids(a.Songs))
	time.Sleep(pause)
	sent, _, err := inChunks(reversed(songs), func(c []int64) error { return addToPlaylist(pl.ID, c...) })
	changed(pl.ID)
	if err != nil {
		return "", fmt.Errorf("created %d %s, but only %d of %d songs went in before: %w", pl.ID, pl.Name, sent, len(songs), err)
	}
	return fmt.Sprintf("created %d %s with %d songs\n", pl.ID, clean(pl.Name), sent), nil
}

func edit(args json.RawMessage, add bool) (string, error) {
	var a struct {
		ID    id   `json:"id"`
		Songs []id `json:"song_ids"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	pid := int64(a.ID)
	p, err := own(pid)
	if err != nil {
		return "", err
	}
	songs := unique(ids(a.Songs))
	if len(songs) == 0 {
		return "", errors.New("no song_ids")
	}
	most := maxEdit
	if add {
		most = maxAdd
	}
	if err := tooMany(len(songs), most, "songs"); err != nil {
		return "", err
	}
	if !add {
		sent, _, err := inChunks(songs, func(c []int64) error { return removeFromPlist(pid, c...) })
		changed(pid)
		if err != nil {
			return "", fmt.Errorf("removed %d of %d songs before: %w", sent, len(songs), err)
		}
		return fmt.Sprintf("removed %d songs from %s\n", sent, clean(p.Name)), nil
	}
	asked := len(songs)
	if songs, err = fresh(pid, songs); err != nil {
		return "", err
	}
	if len(songs) == 0 {
		return fmt.Sprintf("%s has all %d songs already\n", clean(p.Name), asked), nil
	}
	sent, refused, err := inChunks(reversed(songs), func(c []int64) error { return addToPlaylist(pid, c...) })
	changed(pid)
	if err != nil {
		return "", fmt.Errorf("added %d of %d songs before: %w", sent, len(songs), err)
	}
	text := fmt.Sprintf("added %d songs to %s (%d were there already)\n", sent, clean(p.Name), asked-len(songs))
	if refused > 0 {
		text += fmt.Sprintf("NetEase turned away %d songs as already there\n", refused)
	}
	return text, nil
}

func rename(args json.RawMessage) (string, error) {
	var a struct {
		ID   id     `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := own(int64(a.ID))
	if err != nil {
		return "", err
	}
	if p.SpecialType == 5 {
		return "", errors.New("我喜欢的音乐 can't be renamed")
	}
	if strings.TrimSpace(a.Name) == "" {
		return "", errors.New("empty name")
	}
	if err := renamePlaylist(p.ID, strings.TrimSpace(a.Name)); err != nil {
		return "", err
	}
	changed(p.ID)
	return fmt.Sprintf("renamed %s to %s\n", clean(p.Name), clean(a.Name)), nil
}

func remove(args json.RawMessage) (string, error) {
	var a struct {
		ID id `json:"id"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := own(int64(a.ID))
	if err != nil {
		return "", err
	}
	if p.SpecialType == 5 {
		return "", errors.New("我喜欢的音乐 can't be deleted")
	}
	if err := deletePlaylist(p.ID); err != nil {
		return "", err
	}
	changed(p.ID)
	return fmt.Sprintf("deleted %d %s (%d songs)\n", p.ID, clean(p.Name), p.TrackCount), nil
}

func like(args json.RawMessage) (string, error) {
	var a struct {
		Songs []id  `json:"song_ids"`
		Like  *bool `json:"like"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	on := a.Like == nil || *a.Like
	if err := tooMany(len(a.Songs), maxEdit, "songs"); err != nil {
		return "", err
	}
	done := 0
	for i, s := range a.Songs {
		if i > 0 {
			time.Sleep(pause)
		}
		if err := likeSong(int64(s), on); err != nil {
			return "", fmt.Errorf("after %d of %d songs: %w", done, len(a.Songs), err)
		}
		done++
	}
	cache.Lock()
	cache.listsAt = time.Time{}
	cache.playlists = nil
	cache.Unlock()
	verb := "liked"
	if !on {
		verb = "unliked"
	}
	return fmt.Sprintf("%s %d songs\n", verb, done), nil
}
