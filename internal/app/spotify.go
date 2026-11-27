package app

import (
	"context"
	"errors"
	"fmt"
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

	login    *spotify.Login // a connection waiting on the browser
	loginErr string

	lists     []spotify.Playlist
	listsErr  string
	loading   bool
	collapsed bool

	pages map[string]*spotPage
	// busy says, by Spotify playlist, what a copy or sync of it is doing.
	busy map[string]string
	// links are the pairs of playlists kept in step, read off the main
	// thread (crossplay.Links reads a file).
	links []crossplay.Link
}

// spotPage is a Spotify playlist's page: its songs, and those NetEase has.
type spotPage struct {
	loading  bool
	err      string
	progress string
	tracks   []spotify.Track
	matched  []netease.Track // in the playlist's order
	missing  []spotify.Track // those NetEase hasn't
	pending  int             // those not looked for yet
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
	spotifyItems     = spotify.PlaylistItems
	neteaseIDsOf     = crossplay.NeteaseIDs
	songDetails      = netease.SongDetails
	crossSync        = crossplay.Sync
	crossImport      = crossplay.ImportToNetease
	crossExport      = crossplay.ExportToSpotify
)

// bootSpotify reads who is connected to Spotify, off the main thread as
// the keychain may ask first, and loads their playlists.
func (a *app) bootSpotify() {
	go func() {
		id, name := spotifyAccount()
		client := spotify.ClientID()
		links := crossplay.Links()
		a.post(func() {
			a.spot.links = links
			a.spot.user, a.spot.name, a.spot.checked = id, name, true
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
		a.post(func() {
			a.spot.loading = false
			if err != nil {
				a.spot.listsErr = spotErrText(err)
				if errors.Is(err, spotify.ErrNotConnected) {
					a.spot.user, a.spot.name = "", ""
				}
				return
			}
			a.spot.lists = lists
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
		return "Spotify 只允许读取你自己创建或参与协作的歌单"
	case crossplay.Blocked(err):
		return "网易云暂时拒绝了请求，已匹配的歌曲已记下，稍后再试"
	case strings.HasPrefix(err.Error(), "netease:"):
		return "网易云请求失败：" + strings.TrimPrefix(err.Error(), "netease: ")
	case errors.As(err, &e) && e.Reason == "QUOTA_EXCEEDED":
		return "Spotify 的调用额度用完了，稍后再试"
	case errors.As(err, &e) && e.Status == 403:
		return "Spotify 拒绝了请求：开发者模式需要 app 所有者有 Premium，且你的账号在 app 的用户名单里"
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
		a.post(func() {
			if a.spot.login != l {
				return
			}
			a.spot.login = nil
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "closed") {
					a.spot.loginErr = "没有连接上：" + strings.TrimPrefix(err.Error(), "spotify: ")
				}
				return
			}
			a.spot.user, a.spot.name = id, name
			a.loadSpotLists()
		})
	}()
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
	a.spot.user, a.spot.name, a.spot.lists, a.spot.pages = "", "", nil, nil
	if a.route.page == pageSpotify {
		a.open(a.firstPage())
	}
}

// spotSettings is the settings' group that connects Spotify.
func (a *app) spotSettings(c *ui.Context, group func(string, func()), row func(string, string, func())) {
	s := &a.spot
	group("Spotify", func() {
		if s.user != "" {
			row("已连接 "+s.name, "侧边栏会列出你的 Spotify 歌单，歌曲从网易云匹配播放；可以把歌单复制或同步到网易云，也可以反过来", func() {
				if ui.Button(c.Key("spotify-out"), "断开").Clicked() {
					a.disconnectSpotify()
				}
			})
			return
		}
		row("连接 Spotify", "Spotify 只给个人开发者 app 开放接口，需要你自己建一个：在 developer.spotify.com 的 Dashboard 新建 app，Redirect URI 填下面这个地址，API 勾选 Web API，再把 Client ID 粘贴到这里。app 所有者需要 Spotify Premium。Spotify 的音频不能在这里播放，歌曲都从网易云匹配。", func() {})
		row("Redirect URI", spotify.RedirectURI, func() {
			if ui.Button(c.Key("spotify-copy"), "复制").Clicked() {
				copyText(spotify.RedirectURI)
			}
		})
		ui.Row(c).Padding(12, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("spotify-client"), &s.clientID).Placeholder("Client ID").Label("Spotify Client ID").FontSize(13).Grow(1).MinWidth(0)
			if s.login != nil {
				ui.Text(c, "等待浏览器授权…").FontSize(12).TextColor(a.pal.muted)
				if ui.Button(c.Key("spotify-cancel"), "取消").Clicked() {
					a.cancelSpotifyLogin()
				}
			} else if ui.PrimaryButton(c.Key("spotify-in"), "连接").Disabled(len(strings.TrimSpace(s.clientID)) != 32).Clicked() {
				services := c.Services()
				a.startSpotifyLogin(func(u string) { services.OpenURL(u) })
			}
		})
		if s.loginErr != "" {
			ui.Text(c, s.loginErr).FontSize(12).TextColor(ui.Hex("#e5484d")).Padding(0, 16, 12, 16)
		}
	})
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
		b.ContextMenu(a.sideMenu("spotify"))
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
	for i := range s.lists {
		pl := &s.lists[i]
		selected := a.route.page == pageSpotify && a.route.query == pl.ID && !a.nowPlaying
		b := ui.ButtonBase(c.Key("sp-" + pl.ID)).Height(34).PaddingX(8).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pl.Name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		readable := pl.Editable(s.user)
		b.Children(func() {
			a.cover(c, pl.Cover(), 80, 22, 5)
			t := ui.Text(c, pl.Name).FontSize(13).SingleLine().Shrink(1).MinWidth(0)
			if !readable {
				t.TextColor(p.faint)
			}
			// Matching on in the background, after its page was left.
			if pg := s.pages[pl.ID]; pg != nil && pg.loading && pg.total > 0 {
				ui.Box(c).Grow(1)
				ui.Textf(c, "%d%%", pg.done*100/pg.total).FontSize(11).TextColor(p.faint).FontFeatures("tnum")
			}
		})
		if b.Clicked() {
			a.open(route{page: pageSpotify, query: pl.ID})
		}
	}
}

// spotPlaylist returns the listener's Spotify playlist id, or nil.
func (a *app) spotPlaylist(id string) *spotify.Playlist {
	for i := range a.spot.lists {
		if a.spot.lists[i].ID == id {
			return &a.spot.lists[i]
		}
	}
	return nil
}

// loadSpotPage reads the songs of Spotify playlist id and finds them on
// NetEase: the first time slowly, a search a song, spaced out, up to
// it goes on when the page is left, and each song is looked for once:
// opened again, or the playlist changed, only the songs not found before
// are.
func (a *app) loadSpotPage(id string, again bool) {
	if a.spot.pages == nil {
		a.spot.pages = map[string]*spotPage{}
	}
	if pg := a.spot.pages[id]; pg != nil && (pg.loading || !again && pg.err == "") {
		return
	}
	if pl := a.spotPlaylist(id); pl != nil && !pl.Editable(a.spot.user) {
		return // Spotify keeps its songs from the app
	}
	pg := &spotPage{loading: true, progress: "正在读取 Spotify 歌单…"}
	a.spot.pages[id] = pg
	a.loadLinks()
	go func() {
		tracks, err := spotifyItems(id)
		var ids []int64
		looked := 0
		for waits := 0; err == nil || waits < spotWaits; waits++ {
			if err != nil {
				// NetEase asked for fewer searches: wait as long as it
				// said, and go on where it stopped.
				blocked, wait := netease.Blocked(err)
				if !blocked {
					break
				}
				wait = max(wait, 20*time.Second)
				resume := time.Now().Add(wait)
				a.post(func() { pg.resume = resume })
				time.Sleep(time.Until(resume))
				a.post(func() { pg.resume = time.Time{} })
			}
			ids, err = neteaseIDsOf(tracks, crossplay.Budget{
				Progress: func(done, total int) {
					looked = done
					a.post(func() { pg.done, pg.total = done, total })
				},
			})
			if err == nil {
				break
			}
		}
		if err == nil {
			looked = len(tracks)
		}
		var matched []netease.Track
		var missing []spotify.Track
		if err == nil {
			var want []int64
			for i, n := range ids[:looked] {
				if n == 0 {
					missing = append(missing, tracks[i])
				} else {
					want = append(want, n)
				}
			}
			if len(want) > 0 {
				a.post(func() { pg.progress = "正在读取歌曲信息…" })
				matched, err = songDetails(want)
			}
		}
		a.post(func() {
			pg.loading = false
			pg.tracks, pg.matched, pg.missing = tracks, matched, missing
			pg.pending = len(tracks) - looked
			if err != nil {
				pg.err = spotErrText(err)
			}
		})
	}()
}

// spotifyPage is the page of one of the listener's Spotify playlists.
func (a *app) spotifyPage(c *ui.Context) {
	id := a.route.query
	pg := a.spot.pages[id]
	pl := a.spotPlaylist(id)
	if pl != nil && !pl.Editable(a.spot.user) {
		a.centered(c, "Spotify 只允许读取你自己创建或参与协作的歌单。可以在 Spotify 里把它复制成自己的歌单再来。", nil)
		return
	}
	if pg == nil || pg.loading {
		a.spotLoading(c, pg)
		return
	}
	if pg.err != "" && len(pg.matched) == 0 {
		a.centered(c, pg.err, func() { a.loadSpotPage(id, true) })
		return
	}
	var footer []func(c *ui.Context)
	if len(pg.missing) > 0 {
		footer = append(footer, func(c *ui.Context) { a.spotMissing(c, pg.missing) })
	}
	a.trackRows(c, pg.matched, true, func(c *ui.Context) { a.spotHeader(c, id, pl, pg) }, footer...)
}

func (a *app) spotHeader(c *ui.Context, id string, pl *spotify.Playlist, pg *spotPage) {
	p := a.pal
	name, cover, owner := "Spotify 歌单", "", ""
	if pl != nil {
		name, cover, owner = pl.Name, pl.Cover(), pl.Owner.DisplayName
	}
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c.Key("cover-box")).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, cover, 600, 14)
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
			link, linked := a.spotLinkOf(id)
			if busy := a.spot.busy[id]; busy != "" {
				ui.Text(c, busy).FontSize(12.5).TextColor(p.muted)
			} else if linked {
				note := "已和网易云歌单关联"
				if !link.Synced.IsZero() {
					note += " · 上次同步 " + link.Synced.Local().Format("01-02 15:04")
				}
				ui.Text(c, note).FontSize(12.5).TextColor(p.muted)
			}
			if pg.err != "" {
				ui.Text(c, pg.err).FontSize(12.5).TextColor(p.muted)
			}
			if pg.pending > 0 {
				ui.Textf(c, "还有 %d 首没来得及匹配", pg.pending).FontSize(12.5).TextColor(p.muted)
			}
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() && len(pg.matched) > 0 {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.pl.PlayQueue(pg.matched, firstPlayable(pg.matched))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() && len(pg.matched) > 0 {
					a.pl.PlayShuffled(pg.matched)
				}
				busy := a.spot.busy[id] != ""
				if linked {
					b := pillButton(c.Key("sync"), iconReload, "同步", p.hover.Over(p.bg), p.text)
					b.Menu(func(m *ui.Menu) {
						if m.Item("同步到网易云").Disabled(busy).Chosen() {
							a.syncLink(link, crossplay.ToNetease)
						}
						if m.Item("双向同步").Disabled(busy).Chosen() {
							a.syncLink(link, crossplay.Both)
						}
						if m.Item("打开网易云歌单").Chosen() {
							a.open(route{page: pagePlaylist, id: link.Netease})
						}
						m.Separator()
						if m.Item("取消关联").Chosen() {
							go func() {
								crossplay.Unlink(id)
								a.loadLinks()
							}()
						}
					})
				} else if a.profile != nil && pillButton(c.Key("import"), iconPlus, "复制到网易云", p.hover.Over(p.bg), p.text).Disabled(busy).Clicked() {
					a.importSpotify(id, name)
				}
				if pg.pending > 0 && pillButton(c.Key("more"), iconSearch, "继续匹配", p.hover.Over(p.bg), p.text).Clicked() {
					a.loadSpotPage(id, true)
				}
				if iconButton(c.Key("reload"), iconReload, 14, p.muted, "重新读取").Size(36, 36).Radius(999).Clicked() {
					a.loadSpotPage(id, true)
				}
			})
		})
	})
}

// spotMissing lists the songs NetEase hasn't, under those it has.
func (a *app) spotMissing(c *ui.Context, missing []spotify.Track) {
	p := a.pal
	ui.Column(c.Key("spotify-missing")).Padding(24, 12, 8, 12).Gap(6).Children(func() {
		ui.Textf(c, "网易云没有找到的 %d 首", len(missing)).FontSize(12).FontWeight(600).TextColor(p.faint)
		for i := range missing {
			t := &missing[i]
			ui.Text(c, t.Name+" — "+t.ArtistNames()).FontSize(12.5).TextColor(p.muted).SingleLine()
		}
	})
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
// background, saying how it goes on the playlist's page.
func (a *app) crossWork(id string, work func(crossplay.Progress) (crossplay.Link, crossplay.Report, error)) {
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
				a.toast = spotErrText(err)
				a.loadLinks()
				return
			}
			a.toast = syncReport(r)
			a.loadSpotLists()
			a.loadLinks()
		})
	}()
}

func (a *app) importSpotify(id, name string) {
	a.crossWork(id, func(p crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		return crossImport(id, name, crossplay.Budget{}, p)
	})
}

func (a *app) syncLink(l crossplay.Link, d crossplay.Direction) {
	l.Direction = d
	a.crossWork(l.Spotify, func(p crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		r, err := crossSync(l, crossplay.Budget{}, p)
		return l, r, err
	})
}

// spotifyItems is what a NetEase playlist's menu offers for Spotify: copy
// it there, or sync it with the copy.
func (a *app) spotifyMenuItems(m *ui.Menu, pl *netease.Playlist) {
	if a.spot.user == "" {
		return
	}
	if l, ok := a.neteaseLinkOf(pl.ID); ok {
		busy := a.spot.busy[l.Spotify] != ""
		if m.Item("同步到 Spotify").Disabled(busy).Chosen() {
			a.syncLink(l, crossplay.ToSpotify)
			a.toast = "正在同步到 Spotify…"
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
