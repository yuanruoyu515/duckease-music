package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

// The listener's Spotify playlists: connecting in the settings, the
// sidebar's section of them, and their pages, which play their songs as
// NetEase has them. Spotify's audio stays in Spotify.

// spotState is what the app knows of Spotify.
type spotState struct {
	user, name string // who is connected; "" for no one
	checked    bool   // the keychain was read
	clientID   string // what the settings' field holds
	// reconnect says the connection lacks scopes the app asks now, as
	// Liked Songs': connecting again asks the listener for them.
	reconnect bool

	login    *spotify.Login // a connection waiting on the browser
	loginErr string

	lists     []spotify.Playlist
	liked     int // how many songs the listener liked, -1 unknown
	listsErr  string
	loading   bool
	collapsed bool

	pages map[string]*spotPage
	// busy says, by Spotify playlist, what a copy or sync of it is doing.
	busy map[string]string
	// links are the pairs of playlists kept in step, read off the main
	// thread (crossplay.Links reads a file).
	links []crossplay.Link

	edit      spotEditDialog
	importing spotImportDialog
	choosing  spotChooseDialog
	matching  spotMatchDialog
}

// spotPage is a Spotify playlist's page: its songs, and those NetEase has.
// Opened again it shows what was kept of it, on disk too: only Update
// reads Spotify again and looks for songs not found yet.
type spotPage struct {
	loading  bool // reading or matching, in the background
	matching bool // looking for songs on NetEase, which Update starts
	err      string
	progress string
	tracks   []spotify.Track
	matched  []netease.Track // in the playlist's order
	from     []string        // the Spotify URI of each matched
	missing  []spotify.Track // those NetEase hasn't
	pending  int             // those not looked for yet
	snapshot string          // the playlist's version when read
	// How far finding them on NetEase is, while it goes; resume is when
	// it goes on, after NetEase asked it to wait.
	done, total int
	resume      time.Time
}

// spotWaits is how many times a page's matching waits out NetEase's
// asking for fewer searches, before it gives up and offers to retry.
const spotWaits = 8

// The Spotify calls the app makes; tests replace them.
var (
	spotifyAccount   = spotify.Account
	spotifyPlaylists = spotify.Playlists
	spotifyLiked     = spotify.LikedCount
	spotifyItems     = spotify.PlaylistItems
	spotifyAdd       = spotify.AddItems
	spotifyRemove    = spotify.RemoveItems
	spotifyDetails   = spotify.ChangeDetails
	spotifyDrop      = spotify.RemovePlaylist
	spotifyCreate    = spotify.CreatePlaylist
	neteaseIDsOf     = crossplay.NeteaseIDs
	knownIDsOf       = crossplay.KnownNeteaseIDs
	spotifyURIsOf    = crossplay.SpotifyURIs
	songDetails      = netease.SongDetails
	crossSync        = crossplay.Sync
	crossImport      = crossplay.ImportToNetease
	crossExport      = crossplay.ExportToSpotify
	crossLink        = crossplay.LinkTo
)

// bootSpotify reads who is connected to Spotify, off the main thread as
// the keychain may ask first, and loads their playlists.
func (a *app) bootSpotify() {
	go func() {
		id, name := spotifyAccount()
		client := spotify.ClientID()
		again := spotify.NeedsReconnect()
		links := crossplay.Links()
		a.post(func() {
			a.spot.links = links
			a.spot.user, a.spot.name, a.spot.checked = id, name, true
			a.spot.reconnect, a.spot.liked = again, -1
			if a.spot.clientID == "" {
				a.spot.clientID = client
			}
			if id != "" {
				a.loadSpotLists()
			}
		})
	}()
}

func (a *app) loadSpotLists() {
	if a.spot.loading {
		return
	}
	a.spot.loading, a.spot.listsErr = true, ""
	go func() {
		lists, err := spotifyPlaylists()
		liked := -1
		if err == nil && !spotify.NeedsReconnect() {
			if n, err := spotifyLiked(); err == nil {
				liked = n
			}
		}
		a.post(func() {
			a.spot.loading = false
			if err != nil {
				a.spot.listsErr = spotErrText(err)
				if errors.Is(err, spotify.ErrNotConnected) {
					a.spot.user, a.spot.name = "", ""
				}
				return
			}
			a.spot.lists, a.spot.liked = lists, liked
		})
	}()
}

// spotErrText says what went wrong with Spotify, for the listener.
func spotErrText(err error) string {
	var e *spotify.APIError
	switch {
	case errors.Is(err, spotify.ErrNotConnected):
		return "Spotify 已断开，请在设置里重新连接"
	case errors.Is(err, spotify.ErrNotReadable):
		return "读不到这个歌单：Spotify 只把你自己创建或协作的歌单开放给开发者 app，别人的歌单只能从公开页面读前 100 首，私密的读不到"
	case crossplay.Blocked(err):
		return "网易云暂时拒绝了请求，已匹配的歌曲已记下，稍后再试"
	case strings.HasPrefix(err.Error(), "netease:"):
		return "网易云请求失败：" + strings.TrimPrefix(err.Error(), "netease: ")
	case errors.As(err, &e) && e.Reason == "QUOTA_EXCEEDED":
		return "Spotify 的调用额度用完了，稍后再试"
	case errors.As(err, &e) && e.Status == 401:
		return "Spotify 登录过期了，请在设置里重新连接"
	case errors.As(err, &e) && e.Status == 403:
		return "Spotify 拒绝了请求：可能是没有这个歌单的编辑权限，或者开发者模式要求 app 所有者有 Premium、你的账号在 app 的用户名单里"
	}
	return "Spotify 请求失败：" + err.Error()
}

// startSpotifyLogin opens Spotify's page that allows the listener's app,
// and waits for the browser to come back, for up to five minutes.
func (a *app) startSpotifyLogin(open func(string)) {
	l, err := spotify.StartLogin(a.spot.clientID)
	if err != nil {
		a.spot.loginErr = strings.TrimPrefix(err.Error(), "spotify: ")
		return
	}
	a.spot.login, a.spot.loginErr = l, ""
	open(l.URL)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := l.Wait(ctx)
		id, name := spotifyAccount()
		again := spotify.NeedsReconnect()
		a.post(func() {
			if a.spot.login != l {
				return
			}
			a.spot.login = nil
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "closed") {
					a.spot.loginErr = "没有连接上：" + spotLoginHint(err)
				}
				return
			}
			a.spot.user, a.spot.name, a.spot.reconnect = id, name, again
			delete(a.spot.pages, spotify.LikedID)
			a.loadSpotLists()
		})
	}()
}

// spotLoginHint says what a failed connection most likely needs.
func spotLoginHint(err error) string {
	msg := strings.TrimPrefix(err.Error(), "spotify: ")
	switch {
	case strings.Contains(msg, "INVALID_CLIENT") || strings.Contains(msg, "invalid_client"):
		return "Client ID 不对，回 Dashboard 再复制一次"
	case strings.Contains(msg, "redirect"):
		return "Redirect URI 不一致，确认 Dashboard 里填的是 " + spotify.RedirectURI
	case strings.Contains(msg, "access_denied"):
		return "在浏览器里点了取消"
	case strings.Contains(msg, "403"):
		return "Spotify 拒绝了：app 所有者需要 Premium，别人用要先加进 app 的 User Management"
	}
	return msg
}

func (a *app) cancelSpotifyLogin() {
	if a.spot.login != nil {
		a.spot.login.Cancel()
		a.spot.login = nil
	}
}

func (a *app) disconnectSpotify() {
	// The keychain may ask, and a refresh under way holds the session.
	go spotify.Disconnect()
	a.spot.user, a.spot.name, a.spot.lists, a.spot.pages, a.spot.reconnect, a.spot.liked = "", "", nil, nil, false, -1
	if a.route.page == pageSpotify {
		a.open(a.firstPage())
	}
}

// loadLinks reads the pairs of playlists again, as the MCP server may
// have made one.
func (a *app) loadLinks() {
	go func() {
		links := crossplay.Links()
		a.post(func() { a.spot.links = links })
	}()
}

// spotLinkOf returns the pair of Spotify playlist id, if it has one.
func (a *app) spotLinkOf(id string) (crossplay.Link, bool) {
	for _, l := range a.spot.links {
		if l.Spotify == id {
			return l, true
		}
	}
	return crossplay.Link{}, false
}

// neteaseLinkOf returns the pair of NetEase playlist id, if it has one.
func (a *app) neteaseLinkOf(id int64) (crossplay.Link, bool) {
	for _, l := range a.spot.links {
		if l.Netease == id {
			return l, true
		}
	}
	return crossplay.Link{}, false
}

// spotLikedList stands for the listener's Liked Songs among their
// playlists: theirs, and editable.
func (a *app) spotLikedList() spotify.Playlist {
	p := spotify.Playlist{ID: spotify.LikedID, Name: "Spotify 喜欢的歌曲", Owner: spotify.User{ID: a.spot.user, DisplayName: a.spot.name}}
	if a.spot.liked >= 0 {
		p.Items = &struct{ Total int }{a.spot.liked}
	}
	return p
}

// spotAll returns the listener's Liked Songs, then their playlists.
func (a *app) spotAll() []spotify.Playlist {
	return append([]spotify.Playlist{a.spotLikedList()}, a.spot.lists...)
}

// spotHidden reports whether the listener chose not to show playlist id
// in the sidebar.
func (a *app) spotHidden(id string) bool { return slices.Contains(a.settings.SpotifyHidden, id) }

func (a *app) setSpotHidden(id string, hidden bool) {
	h := slices.DeleteFunc(slices.Clone(a.settings.SpotifyHidden), func(x string) bool { return x == id })
	if hidden {
		h = append(h, id)
	}
	a.settings.SpotifyHidden = h
	a.keepSettings()
}

// spotPlaylists is the sidebar's section of the listener's Spotify
// playlists, while Spotify is connected.
func (a *app) spotPlaylists(c *ui.Context, title string) {
	s := &a.spot
	if s.user == "" {
		return
	}
	p := a.pal
	ui.Row(c.Key("spotify-head")).Margin(14, 0, 4, 0).Gap(2).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c.Key("toggle")).Grow(1).Height(26).PaddingX(4).Gap(6).Radius(6).
			AlignItems(ui.Center).Justify(ui.Start).Role(ui.RoleDisclosure).Label(title)
		if b.Clicked() {
			s.collapsed = !s.collapsed
		}
		b.ContextMenu(a.spotSectionMenu())
		b.Expanded(!s.collapsed)
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(p.faint).Grow(1)
			arrow := iconDown
			if s.collapsed {
				arrow = iconForward
			}
			ui.Icon(c, arrow).Size(12, 12).TextColor(p.faint)
		})
		iconButton(c.Key("more"), iconMore, 13, p.faint, "Spotify 歌单选项").Size(26, 26).Radius(6).Menu(a.spotSectionMenu())
		if iconButton(c.Key("refresh"), iconReload, 13, p.faint, "刷新 Spotify 歌单").Size(26, 26).Radius(6).Disabled(s.loading).Clicked() {
			a.loadSpotLists()
		}
	})
	if s.collapsed {
		return
	}
	if s.listsErr != "" {
		ui.Text(c, s.listsErr).FontSize(12).TextColor(p.faint).Padding(4, 8).LineHeight(1.4)
	}
	for _, pl := range a.spotAll() {
		if a.spotHidden(pl.ID) {
			continue
		}
		id := pl.ID
		selected := a.route.page == pageSpotify && a.route.query == id && !a.nowPlaying
		b := ui.ButtonBase(c.Key("sp-" + id)).Height(34).PaddingX(8).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pl.Name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		b.ContextMenu(func(m *ui.Menu) {
			if m.Item("在侧边栏隐藏").Chosen() {
				a.setSpotHidden(id, true)
			}
			if m.Item("选择显示的歌单…").Chosen() {
				a.openSpotChooser()
			}
		})
		cover := pl.Cover()
		b.Children(func() {
			if spotify.IsLiked(id) {
				ui.Box(c).Size(22, 22).Radius(5).Center().Background(p.accent.Alpha(0.14)).Children(func() {
					ui.Icon(c, iconHeartFill).Size(12, 12).TextColor(p.accent)
				})
			} else {
				a.cover(c, cover, 80, 22, 5)
			}
			ui.Text(c, pl.Name).FontSize(13).SingleLine().Shrink(1).MinWidth(0)
			// Matching on in the background, after its page was left.
			if pg := s.pages[id]; pg != nil && pg.matching && pg.total > 0 {
				ui.Box(c).Grow(1)
				ui.Textf(c, "%d%%", pg.done*100/pg.total).FontSize(11).TextColor(p.faint).FontFeatures("tnum")
			}
		})
		if b.Clicked() {
			a.open(route{page: pageSpotify, query: id})
		}
	}
}

// spotSectionMenu is the menu of the sidebar's Spotify section.
func (a *app) spotSectionMenu() func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("新建 Spotify 歌单…").Chosen() {
			a.spot.edit = spotEditDialog{open: true}
		}
		if m.Item("选择显示的歌单…").Chosen() {
			a.openSpotChooser()
		}
		if m.Item("刷新").Disabled(a.spot.loading).Chosen() {
			a.loadSpotLists()
		}
		m.Separator()
		a.sideMenu("spotify")(m)
	}
}

// spotPlaylist returns the listener's Spotify playlist id, their Liked
// Songs too, or nil.
func (a *app) spotPlaylist(id string) *spotify.Playlist {
	if spotify.IsLiked(id) {
		pl := a.spotLikedList()
		return &pl
	}
	for i := range a.spot.lists {
		if a.spot.lists[i].ID == id {
			return &a.spot.lists[i]
		}
	}
	return nil
}

// spotEditable reports whether the listener may change playlist id's
// songs: their Liked Songs, their own and collaborative playlists.
func (a *app) spotEditable(id string) bool {
	pl := a.spotPlaylist(id)
	return pl != nil && pl.Editable(a.spot.user)
}

// spotOwn reports whether playlist id is the listener's own, which they
// may rename and delete: not their Liked Songs.
func (a *app) spotOwn(id string) bool {
	pl := a.spotPlaylist(id)
	return pl != nil && !spotify.IsLiked(id) && pl.Owner.ID == a.spot.user
}

// loadSpotPage shows the page of Spotify playlist id: as kept, when it
// was read before, without asking anyone; else read off Spotify, with
// the songs found before, searching for none.
func (a *app) loadSpotPage(id string) {
	if a.spot.pages == nil {
		a.spot.pages = map[string]*spotPage{}
	}
	if a.spot.pages[id] != nil {
		return
	}
	if spotify.IsLiked(id) && a.spot.reconnect {
		return // the page offers to connect again
	}
	pg := &spotPage{loading: true, progress: "正在读取 Spotify 歌单…"}
	a.spot.pages[id] = pg
	a.loadLinks()
	snapshot := a.spotSnapshot(id)
	go func() {
		if kept, ok := readSpotCache(id); ok {
			a.post(func() {
				if a.spot.pages[id] == pg {
					kept.restore(pg)
					pg.loading = false
				}
			})
			return
		}
		a.readSpotPage(id, pg, snapshot, false)
	}()
}

// spotSnapshot returns the version of playlist id the list says.
func (a *app) spotSnapshot(id string) string {
	if pl := a.spotPlaylist(id); pl != nil {
		return pl.SnapshotID
	}
	return ""
}

// updateSpotPage reads Spotify playlist id again and looks on NetEase
// for its songs not found yet, a search a song, spaced out: it goes on
// when the page is left.
func (a *app) updateSpotPage(id string) {
	if a.spot.pages == nil {
		a.spot.pages = map[string]*spotPage{}
	}
	old := a.spot.pages[id]
	if old != nil && old.loading {
		return
	}
	pg := &spotPage{loading: true, matching: true, progress: "正在读取 Spotify 歌单…"}
	if old != nil {
		// What it showed stays while it updates.
		spotCacheOf(old).restore(pg)
	}
	a.spot.pages[id] = pg
	a.loadLinks()
	if spotify.IsLiked(id) {
		a.loadSpotLists() // its count
	}
	go a.readSpotPage(id, pg, a.spotSnapshot(id), true)
}

// readSpotPage reads the songs of Spotify playlist id into pg, off the
// main thread, finding on NetEase those not found before if search.
func (a *app) readSpotPage(id string, pg *spotPage, snapshot string, search bool) {
	tracks, err := spotifyItems(id)
	if err != nil {
		a.post(func() {
			pg.loading, pg.matching = false, false
			pg.err = spotErrText(err)
		})
		return
	}
	var searchErr error
	if search {
		for waits := 0; ; waits++ {
			_, searchErr = neteaseIDsOf(tracks, crossplay.Budget{
				Progress: func(done, total int) {
					a.post(func() { pg.done, pg.total = done, total })
				},
			})
			if searchErr == nil || waits >= spotWaits {
				break
			}
			// NetEase asked for fewer searches: wait as long as it said,
			// and go on where it stopped.
			blocked, wait := netease.Blocked(searchErr)
			if !blocked {
				break
			}
			wait = max(wait, 20*time.Second)
			resume := time.Now().Add(wait)
			a.post(func() { pg.resume = resume })
			time.Sleep(time.Until(resume))
			a.post(func() { pg.resume = time.Time{} })
		}
	}
	// Whatever the search got to, the songs found before show.
	ids, unknown := knownIDsOf(tracks)
	kept := spotCache{Snapshot: snapshot, Tracks: tracks}
	var want []int64
	for _, n := range ids {
		if n != 0 {
			want = append(want, n)
		}
	}
	var details []netease.Track
	if len(want) > 0 {
		a.post(func() { pg.progress = "正在读取歌曲信息…" })
		details, err = songDetails(want)
	}
	if err == nil {
		byID := make(map[int64]netease.Track, len(details))
		for _, t := range details {
			byID[t.ID] = t
		}
		for i, n := range ids {
			switch t, ok := byID[n]; {
			case ok:
				kept.Matched = append(kept.Matched, t)
				kept.From = append(kept.From, tracks[i].URI)
			case unknown[i]:
				kept.Pending++
			default:
				kept.Missing = append(kept.Missing, tracks[i])
			}
		}
		writeSpotCache(id, kept)
	}
	if err == nil {
		err = searchErr
	}
	a.post(func() {
		if a.spot.pages[id] != pg {
			return
		}
		if kept.Matched != nil || kept.Missing != nil || kept.Pending > 0 || len(tracks) == 0 {
			kept.restore(pg)
		}
		pg.loading, pg.matching = false, false
		if err != nil {
			pg.err = spotErrText(err)
		}
	})
}

// spotifyPage is the page of one of the listener's Spotify playlists.
func (a *app) spotifyPage(c *ui.Context) {
	id := a.route.query
	pg := a.spot.pages[id]
	pl := a.spotPlaylist(id)
	if spotify.IsLiked(id) && a.spot.reconnect {
		services := c.Services()
		a.centered(c, "读取 Spotify 喜欢的歌曲需要多一项授权，重新连接一次 Spotify 就好", func() {
			a.startSpotifyLogin(func(u string) { services.OpenURL(u) })
		})
		return
	}
	if pg == nil || pg.loading && len(pg.tracks) == 0 {
		if pg != nil && pg.matching {
			a.spotLoading(c, pg)
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, true, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	if pg.err != "" && len(pg.tracks) == 0 {
		a.centered(c, pg.err, func() {
			delete(a.spot.pages, id)
			a.loadSpotPage(id)
		})
		return
	}
	// Sorted and searched as a NetEase playlist's page is.
	f := &a.plFilter
	if f.id != 0 || f.spot != id {
		*f = plFilter{spot: id}
	}
	tracks := a.spotTracks(pg)
	shown := f.match(tracks)
	header := func(c *ui.Context) {
		a.spotHeader(c, id, pl, pg)
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(0, 12, 14, 12).Children(func() {
			a.playlistSort(c)
			ui.Box(c).Grow(1)
			a.playlistSearch(c)
		})
	}
	var footer []func(c *ui.Context)
	if shown != nil && len(shown) == 0 {
		footer = append(footer, func(c *ui.Context) { a.playlistNoMatch(c, strings.TrimSpace(f.query)) })
	}
	if len(pg.missing) > 0 && shown == nil {
		footer = append(footer, func(c *ui.Context) { a.spotMissing(c, id, pg.missing) })
	}
	a.trackRowsOf(c, tracks, shown, true, header, footer...)
}

// spotTracks returns the songs NetEase has of a Spotify playlist, in the
// order chosen on its page.
func (a *app) spotTracks(pg *spotPage) []netease.Track {
	f := &a.plFilter
	if f.spot == "" || f.id != 0 {
		return pg.matched
	}
	if f.sort == sortNotLiked {
		return f.unliked.tracks(pg.matched, a.liked)
	}
	return f.order.tracks(pg.matched, f.sort, f.luck)
}

// spotStale reports whether Spotify has a newer version of the playlist
// than its page shows.
func (a *app) spotStale(id string, pl *spotify.Playlist, pg *spotPage) bool {
	if pl == nil || pg.loading {
		return false
	}
	if spotify.IsLiked(id) {
		return a.spot.liked >= 0 && a.spot.liked != len(pg.tracks)
	}
	return pl.SnapshotID != "" && pg.snapshot != "" && pl.SnapshotID != pg.snapshot
}

func (a *app) spotHeader(c *ui.Context, id string, pl *spotify.Playlist, pg *spotPage) {
	p := a.pal
	name, cover, owner, desc := "Spotify 歌单", "", "", ""
	if pl != nil {
		name, cover, owner, desc = pl.Name, pl.Cover(), pl.Owner.DisplayName, pl.Description
	}
	editable := a.spotEditable(id)
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c.Key("cover-box")).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			if spotify.IsLiked(id) {
				ui.Box(c).Fill().Radius(14).Center().Background(p.accent.Alpha(0.14)).Children(func() {
					ui.Icon(c, iconHeartFill).Size(72, 72).TextColor(p.accent)
				})
			} else {
				a.coverFill(c, cover, 600, 14)
			}
			a.coverMoreButton(c).Menu(a.spotPageMenu(id, name))
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "Spotify 歌单").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			meta := []string{}
			if owner != "" {
				meta = append(meta, owner)
			}
			meta = append(meta, fmt.Sprintf("%d 首", len(pg.tracks)), fmt.Sprintf("网易云有 %d 首", len(pg.matched)))
			ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted)
			if d := strings.TrimSpace(desc); d != "" {
				ui.Text(c, strings.ReplaceAll(d, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45)
			}
			if !editable && pl != nil && pl.Total() > len(pg.tracks) {
				ui.Textf(c, "别人的歌单 Spotify 只开放前 %d 首，这里读到了 %d 首", spotify.EmbedLimit, len(pg.tracks)).FontSize(12.5).TextColor(p.muted)
			}
			link, linked := a.spotLinkOf(id)
			if busy := a.spot.busy[id]; busy != "" {
				ui.Text(c, busy).FontSize(12.5).TextColor(p.muted)
			} else if linked {
				note := "已关联网易云歌单"
				if ne := a.mineName(link.Netease); ne != "" {
					note += "「" + ne + "」"
				}
				if !link.Synced.IsZero() {
					note += " · 上次同步 " + link.Synced.Local().Format("01-02 15:04")
				}
				ui.Text(c, note).FontSize(12.5).TextColor(p.muted)
			}
			switch {
			case pg.matching && !pg.resume.IsZero():
				left := max(time.Until(pg.resume), 0)
				ui.Textf(c, "网易云说搜得太快了，%d 秒后接着匹配", int(left.Seconds())+1).FontSize(12.5).TextColor(p.muted).FontFeatures("tnum")
				c.After(time.Second)
			case pg.matching && pg.total > 0:
				ui.Textf(c, "正在网易云匹配 %d / %d…", pg.done, pg.total).FontSize(12.5).TextColor(p.muted).FontFeatures("tnum")
			case pg.loading:
				ui.Text(c, pg.progress).FontSize(12.5).TextColor(p.muted)
			case pg.err != "":
				ui.Text(c, pg.err).FontSize(12.5).TextColor(p.muted)
			case a.spotStale(id, pl, pg):
				ui.Text(c, "Spotify 上这个歌单有变化，点「更新」读取").FontSize(12.5).TextColor(p.accent)
			case pg.pending > 0:
				ui.Textf(c, "还有 %d 首没有匹配，点「更新」在网易云找", pg.pending).FontSize(12.5).TextColor(p.muted)
			}
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() && len(pg.matched) > 0 {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					tracks := a.spotTracks(pg)
					a.pl.PlayQueue(tracks, firstPlayable(tracks))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() && len(pg.matched) > 0 {
					a.pl.PlayShuffled(a.spotTracks(pg))
				}
				label := "更新"
				if pg.loading {
					label = "更新中…"
				}
				if pillButton(c.Key("update"), iconReload, label, p.hover.Over(p.bg), p.text).Disabled(pg.loading).Clicked() {
					a.updateSpotPage(id)
				}
				busy := a.spot.busy[id] != ""
				if linked {
					b := pillButton(c.Key("sync"), iconReload, "同步", p.hover.Over(p.bg), p.text)
					b.Menu(func(m *ui.Menu) {
						if m.Item("同步到网易云").Disabled(busy).Chosen() {
							a.syncLink(link, crossplay.ToNetease)
						}
						if m.Item("双向同步").Disabled(busy || !editable).Chosen() {
							a.syncLink(link, crossplay.Both)
						}
						if m.Item("打开网易云歌单").Chosen() {
							a.open(route{page: pagePlaylist, id: link.Netease})
						}
						m.Separator()
						if m.Item("改关联到别的网易云歌单…").Disabled(busy).Chosen() {
							a.openSpotImport(id, name, true)
						}
						if m.Item("取消关联").Chosen() {
							go func() {
								crossplay.Unlink(id)
								a.loadLinks()
							}()
						}
					})
				} else if a.profile != nil && pillButton(c.Key("import"), iconPlus, "复制到网易云", p.hover.Over(p.bg), p.text).Disabled(busy).Clicked() {
					a.openSpotImport(id, name, false)
				}
			})
		})
	})
}

// mineName returns the name of the listener's NetEase playlist id, or "".
func (a *app) mineName(id int64) string {
	for _, pl := range a.mine {
		if pl.ID == id {
			return pl.Name
		}
	}
	return ""
}

// spotPageMenu is the menu behind the dots on a Spotify playlist's cover.
func (a *app) spotPageMenu(id, name string) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		own := a.spotOwn(id)
		if own && m.Item("编辑名称和简介…").Chosen() {
			pl := a.spotPlaylist(id)
			a.spot.edit = spotEditDialog{open: true, id: id, name: pl.Name, desc: pl.Description}
		}
		if a.profile != nil {
			if _, linked := a.spotLinkOf(id); !linked {
				if m.Item("复制到网易云…").Chosen() {
					a.openSpotImport(id, name, false)
				}
				if m.Item("关联已有网易云歌单…").Chosen() {
					a.openSpotImport(id, name, true)
				}
			}
		}
		if m.Item("更新").Chosen() {
			a.updateSpotPage(id)
		}
		if !spotify.IsLiked(id) && m.Item("复制 Spotify 链接").Chosen() {
			copyText("https://open.spotify.com/playlist/" + id)
		}
		if m.Item("在侧边栏隐藏").Chosen() {
			a.setSpotHidden(id, true)
		}
		if !spotify.IsLiked(id) {
			m.Separator()
			label := "从资料库移除…"
			if own {
				label = "删除歌单…"
			}
			if m.Item(label).Chosen() {
				a.confirmSpotDelete(id, name, own)
			}
		}
	}
}

// spotMissing lists the songs NetEase hasn't, under those it has: a click
// matches one by hand, and their menus take them out of the Spotify
// playlist id.
func (a *app) spotMissing(c *ui.Context, id string, missing []spotify.Track) {
	p := a.pal
	editable := a.spotEditable(id)
	ui.Column(c.Key("spotify-missing")).Padding(24, 12, 8, 12).Gap(2).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(0, 0, 4, 0).Children(func() {
			ui.Textf(c, "网易云没有找到的 %d 首", len(missing)).FontSize(12).FontWeight(600).TextColor(p.faint)
			ui.Text(c, "点一首可以手动匹配").FontSize(12).TextColor(p.faint)
		})
		for i := range missing {
			t := missing[i]
			label := t.Name + " — " + t.ArtistNames()
			b := ui.ButtonBase(c.Key(fmt.Sprintf("miss-%d-%s", i, t.ID))).Height(30).PaddingX(8).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(label)
			hovered := b.Hovered()
			if hovered {
				b.Background(p.hover)
			}
			b.ContextMenu(func(m *ui.Menu) {
				if m.Item("手动匹配…").Chosen() {
					a.openSpotMatch(id, t)
				}
				m.Separator()
				if m.Item("复制歌名和歌手").Chosen() {
					copyText(t.Name + " - " + t.ArtistNames())
				}
				if m.Item("在网易云搜索").Chosen() {
					a.query = t.Name + " " + t.ArtistNames()
					a.searchFor(route{page: pageSearch, query: a.query, kind: netease.SearchSongs})
				}
				if editable {
					m.Separator()
					if m.Item("从这个 Spotify 歌单移除").Chosen() {
						a.removeFromSpot(id, []string{t.URI})
					}
				}
			})
			b.Children(func() {
				ui.Text(c, label).FontSize(12.5).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
				ui.Box(c).Grow(1)
				if hovered {
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconSearch).Size(12, 12).TextColor(p.accent)
						ui.Text(c, "手动匹配").FontSize(12).FontWeight(600).TextColor(p.accent)
					})
				}
			})
			if b.Clicked() {
				a.openSpotMatch(id, t)
			}
		}
	})
}

// removeFromSpot takes songs, by URI, out of Spotify playlist id: at once
// on its page, and then with Spotify, which may refuse: they come back
// then.
func (a *app) removeFromSpot(id string, uris []string) {
	pg := a.spot.pages[id]
	var before spotCache
	if pg != nil {
		before = spotCacheOf(pg)
		gone := map[string]bool{}
		for _, u := range uris {
			gone[u] = true
		}
		after := before.without(gone)
		after.restore(pg)
	}
	go func() {
		err := spotifyRemove(id, uris)
		if kept, ok := readSpotCache(id); err == nil && ok {
			gone := map[string]bool{}
			for _, u := range uris {
				gone[u] = true
			}
			writeSpotCache(id, kept.without(gone))
		}
		a.post(func() {
			if err != nil {
				if pg != nil && a.spot.pages[id] == pg {
					before.restore(pg)
				}
				a.toast = spotErrText(err)
				return
			}
			a.toast = "已从 Spotify 歌单移除"
			if spotify.IsLiked(id) && a.spot.liked > 0 {
				a.spot.liked -= len(uris)
			}
			if pl := a.spotPlaylist(id); pl != nil && !spotify.IsLiked(id) {
				a.loadSpotLists() // its count and version changed
			}
		})
	}()
}

// addToSpot finds song tr on Spotify and adds it to playlist id.
func (a *app) addToSpot(id, name string, tr *netease.Track) {
	t := *tr
	a.toast = "正在 Spotify 上找这首歌…"
	go func() {
		uris, err := spotifyURIsOf([]netease.Track{t}, crossplay.Budget{})
		if err == nil && (len(uris) == 0 || uris[0] == "") {
			a.post(func() { a.toast = "Spotify 上没有找到《" + t.Name + "》" })
			return
		}
		if err == nil && !spotify.IsLiked(id) {
			// Already in it, as the page last read it: not again.
			if kept, ok := readSpotCache(id); ok && slices.ContainsFunc(kept.Tracks, func(s spotify.Track) bool { return s.URI == uris[0] }) {
				a.post(func() { a.toast = "《" + t.Name + "》已经在「" + name + "」里了" })
				return
			}
		}
		if err == nil {
			err = spotifyAdd(id, uris[:1])
		}
		a.post(func() {
			if err != nil {
				a.toast = spotErrText(err)
				return
			}
			a.toast = "已添加到「" + name + "」"
			if pg := a.spot.pages[id]; pg != nil {
				// It shows after the next update.
				pg.snapshot = "changed"
			}
			if spotify.IsLiked(id) && a.spot.liked >= 0 {
				a.spot.liked++
			} else {
				a.loadSpotLists()
			}
		})
	}()
}

// spotSongItems are a song's menu items for Spotify: adding it to one of
// the listener's Spotify playlists, and on a Spotify playlist's page,
// taking it out of that one.
func (a *app) spotSongItems(m *ui.Menu, tr *netease.Track) {
	if a.spot.user == "" || tr.ProgramID != 0 {
		return
	}
	here := ""
	if a.route.page == pageSpotify {
		here = a.route.query
	}
	m.Separator()
	m.Submenu("添加到 Spotify 歌单", func(m *ui.Menu) {
		for _, pl := range a.spotAll() {
			if pl.ID == here || !pl.Editable(a.spot.user) || (spotify.IsLiked(pl.ID) && a.spot.reconnect) {
				continue
			}
			id, name := pl.ID, pl.Name
			if m.Item(name).Chosen() {
				a.addToSpot(id, name, tr)
			}
		}
	})
	if here == "" {
		return
	}
	pg := a.spot.pages[here]
	if pg == nil {
		return
	}
	var uris []string
	for i, t := range pg.matched {
		if t.ID == tr.ID && i < len(pg.from) {
			uris = append(uris, pg.from[i])
		}
	}
	if len(uris) == 0 {
		return
	}
	if m.Item("匹配错了，手动改…").Chosen() {
		for _, t := range pg.tracks {
			if t.URI == uris[0] {
				a.openSpotMatch(here, t)
				break
			}
		}
	}
	if a.spotEditable(here) && m.Item("从这个 Spotify 歌单移除").Chosen() {
		a.removeFromSpot(here, uris)
	}
}

// confirmSpotDelete asks whether to delete the listener's Spotify
// playlist id, or take someone else's out of their library.
func (a *app) confirmSpotDelete(id, name string, own bool) {
	title, body, action := "从资料库移除", "确定把“"+name+"”从你的 Spotify 资料库移除吗？之后可以在 Spotify 里重新收藏。", "移除"
	if own {
		title, body, action = "删除 Spotify 歌单", "确定删除 Spotify 歌单“"+name+"”吗？Spotify 会把它从你的资料库移除，关联的网易云歌单不受影响。", "删除"
	}
	a.ask = confirmDialog{open: true, title: title, body: body, action: action, do: func() { a.deleteSpot(id) }}
}

func (a *app) deleteSpot(id string) {
	i := slices.IndexFunc(a.spot.lists, func(p spotify.Playlist) bool { return p.ID == id })
	if i < 0 {
		return
	}
	pl := a.spot.lists[i]
	a.spot.lists = slices.Delete(slices.Clone(a.spot.lists), i, i+1)
	if a.route.page == pageSpotify && a.route.query == id {
		a.open(a.firstPage())
	}
	go func() {
		err := spotifyDrop(id)
		if err == nil {
			crossplay.Unlink(id)
			removeSpotCache(id)
		}
		a.post(func() {
			if err != nil {
				a.spot.lists = slices.Insert(slices.Clone(a.spot.lists), min(i, len(a.spot.lists)), pl)
				a.toast = spotErrText(err)
				return
			}
			delete(a.spot.pages, id)
			a.toast = "已移除 Spotify 歌单"
			a.loadLinks()
		})
	}()
}

// syncReport says what a sync did, in a toast.
func syncReport(r crossplay.Report) string {
	parts := []string{}
	if r.AddedToNetease > 0 {
		parts = append(parts, fmt.Sprintf("网易云新增 %d 首", r.AddedToNetease))
	}
	if r.AddedToSpotify > 0 {
		parts = append(parts, fmt.Sprintf("Spotify 新增 %d 首", r.AddedToSpotify))
	}
	if len(parts) == 0 {
		parts = append(parts, "两边已经一致")
	}
	if n := len(r.MissingOnNetease) + len(r.MissingOnSpotify); n > 0 {
		parts = append(parts, fmt.Sprintf("%d 首另一边没有", n))
	}
	return "同步完成：" + strings.Join(parts, "，")
}

// crossWork runs a copy or sync for Spotify playlist id in the
// background, saying how it goes on the playlist's page. failed, if set,
// hears an error before anything was linked, as NetEase refusing a
// playlist's name.
func (a *app) crossWork(id string, work func(crossplay.Progress) (crossplay.Link, crossplay.Report, error), failed ...func(error)) {
	if a.spot.busy == nil {
		a.spot.busy = map[string]string{}
	}
	if a.spot.busy[id] != "" {
		return
	}
	a.spot.busy[id] = "正在准备…"
	go func() {
		l, r, err := work(func(step string, done, total int) {
			text := step + "…"
			if total > 0 {
				text = fmt.Sprintf("%s %d/%d…", step, done, total)
			}
			a.post(func() { a.spot.busy[id] = text })
		})
		a.post(func() {
			delete(a.spot.busy, id)
			if l.Netease != 0 {
				delete(a.playlists, l.Netease)
				if a.profile != nil {
					a.loadMine(a.profile.UserID)
				}
			}
			if err != nil {
				if l.Netease == 0 && len(failed) > 0 {
					failed[0](err)
				} else {
					a.toast = spotErrText(err)
				}
				a.loadLinks()
				return
			}
			a.toast = syncReport(r)
			if r.AddedToSpotify > 0 {
				if pg := a.spot.pages[l.Spotify]; pg != nil {
					pg.snapshot = "changed"
				}
			}
			a.loadSpotLists()
			a.loadLinks()
		})
	}()
}

func (a *app) syncLink(l crossplay.Link, d crossplay.Direction) {
	l.Direction = d
	a.crossWork(l.Spotify, func(p crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		r, err := crossSync(l, crossplay.Budget{}, p)
		return l, r, err
	})
}

// spotifyMenuItems is what a NetEase playlist's menu offers for Spotify: copy
// it there, or sync it with the copy.
func (a *app) spotifyMenuItems(m *ui.Menu, pl *netease.Playlist) {
	if a.spot.user == "" {
		return
	}
	if l, ok := a.neteaseLinkOf(pl.ID); ok {
		busy := a.spot.busy[l.Spotify] != ""
		if m.Item("同步到 Spotify").Disabled(busy || !a.spotEditable(l.Spotify)).Chosen() {
			a.syncLink(l, crossplay.ToSpotify)
			a.toast = "正在同步到 Spotify…"
		}
		if m.Item("打开 Spotify 歌单").Chosen() {
			a.open(route{page: pageSpotify, query: l.Spotify})
		}
		return
	}
	key := fmt.Sprintf("netease-%d", pl.ID)
	if m.Item("复制到 Spotify").Disabled(a.spot.busy[key] != "").Chosen() {
		id, name := pl.ID, pl.Name
		a.toast = "正在复制到 Spotify，歌多时要几分钟…"
		a.crossWork(key, func(p crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
			return crossExport(id, name, crossplay.Budget{}, p)
		})
	}
}
