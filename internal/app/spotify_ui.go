package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/crossplay"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/spotify"
)

// The settings' guide to connecting Spotify, the dialogs that edit the
// listener's Spotify playlists, and what is kept of their pages on disk.

// spotDashboard is where the listener makes their Spotify app.
const spotDashboard = "https://developer.spotify.com/dashboard"

// spotSettings is the settings' group that connects Spotify: a guide,
// step by step, until it is.
func (a *app) spotSettings(c *ui.Context, group func(string, func()), row func(string, string, func())) {
	s := &a.spot
	p := paletteFor(c.Theme())
	services := c.Services()
	group("Spotify", func() {
		if s.user != "" {
			row("已连接 "+s.name, "侧边栏会列出你的 Spotify 歌单和喜欢的歌曲，歌从网易云匹配播放。可以在这里编辑、删除 Spotify 歌单，复制或关联到网易云歌单", func() {
				if ui.Button(c.Key("spotify-out"), "断开").Clicked() {
					a.disconnectSpotify()
				}
			})
			if s.reconnect {
				row("需要重新授权一次", "新版本要读取和修改“喜欢的歌曲”，Spotify 需要你再点一次同意。Client ID 不用重填", func() {
					if s.login != nil {
						ui.Text(c, "等待浏览器授权…").FontSize(12).TextColor(p.muted)
						return
					}
					if ui.PrimaryButton(c.Key("spotify-again"), "重新连接").Clicked() {
						a.startSpotifyLogin(func(u string) { services.OpenURL(u) })
					}
				})
			}
			if s.loginErr != "" {
				ui.Text(c, s.loginErr).FontSize(12).TextColor(ui.Hex("#e5484d")).Padding(0, 16, 12, 16)
			}
			return
		}
		ui.Column(c).Padding(14, 16).Gap(4).Children(func() {
			ui.Text(c, "连接 Spotify").FontSize(13.5).FontWeight(500)
			ui.Text(c, "Spotify 不给第三方软件统一的接口，每个人要建一个自己的“开发者 app”，拿到它的 Client ID。全程免费，大约 3 分钟，照下面做就行。").FontSize(12).TextColor(p.muted).LineHeight(1.5)
		})
		step := func(n int, title, desc string, action func()) {
			ui.Row(c).Padding(12, 16).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Size(22, 22).Radius(999).Center().Background(p.accent.Alpha(0.14)).Children(func() {
					ui.Textf(c, "%d", n).FontSize(12).FontWeight(700).TextColor(p.accent)
				})
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, title).FontSize(13).FontWeight(500)
					if desc != "" {
						ui.Text(c, desc).FontSize(12).TextColor(p.muted).LineHeight(1.5)
					}
				})
				if action != nil {
					action()
				}
			})
		}
		step(1, "打开 Spotify 开发者后台", "用你平时听歌的 Spotify 账号登录。第一次进会让你同意开发者条款，勾上就行", func() {
			if ui.Button(c.Key("spotify-dash"), "打开").Clicked() {
				services.OpenURL(spotDashboard)
			}
		})
		step(2, "点右上角 Create app", "App name 和 App description 随便写，比如 DuckEase。Website 留空", nil)
		step(3, "在 Redirect URIs 里粘贴这个地址，点 Add", spotify.RedirectURI+"\n一个字都不能差，开头是 http 不是 https", func() {
			if ui.Button(c.Key("spotify-copy"), "复制地址").Clicked() {
				copyText(spotify.RedirectURI)
				a.toast = "已复制 Redirect URI"
			}
		})
		step(4, "API 那一栏勾 Web API，勾同意条款，点 Save", "", nil)
		step(5, "复制 Client ID，粘贴到下面，点连接", "在刚建好的 app 页面点 Settings 就能看到 Client ID，是 32 位字母和数字。不需要 Client Secret。点连接后浏览器会打开 Spotify 的授权页，点 Agree，再回到这里", nil)
		ui.Row(c).Padding(4, 16, 12, 50).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("spotify-client"), &s.clientID).Placeholder("粘贴 Client ID").Label("Spotify Client ID").FontSize(13).Grow(1).MinWidth(0)
			if s.login != nil {
				ui.Text(c, "等待浏览器授权…").FontSize(12).TextColor(p.muted)
				if ui.Button(c.Key("spotify-cancel"), "取消").Clicked() {
					a.cancelSpotifyLogin()
				}
			} else if ui.PrimaryButton(c.Key("spotify-in"), "连接").Disabled(len(strings.TrimSpace(s.clientID)) != 32).Clicked() {
				s.clientID = strings.TrimSpace(s.clientID)
				a.startSpotifyLogin(func(u string) { services.OpenURL(u) })
			}
		})
		if n := len(strings.TrimSpace(s.clientID)); n > 0 && n != 32 {
			ui.Textf(c, "现在是 %d 位，Client ID 应该是 32 位。复制时别带上空格或别的字", n).FontSize(12).TextColor(p.muted).Padding(0, 16, 12, 50)
		}
		if s.loginErr != "" {
			ui.Text(c, s.loginErr).FontSize(12).TextColor(ui.Hex("#e5484d")).Padding(0, 16, 12, 50).LineHeight(1.45)
		}
		ui.Column(c).Padding(12, 16).Gap(4).Children(func() {
			ui.Text(c, "要知道的几件事").FontSize(12).FontWeight(600).TextColor(p.muted)
			for _, note := range []string{
				"建 app 的那个 Spotify 账号需要 Premium，这是 Spotify 2026 年起的规定。",
				"只给自己用不用管名单。给家人朋友用，要在 app 的 User Management 里加上他们的 Spotify 邮箱，最多 5 人。",
				"Spotify 的音频不能在这里播放，歌都从网易云匹配来放。",
				"登录信息存在钥匙串里，和网易云的放在一起。",
			} {
				ui.Text(c, "· "+note).FontSize(12).TextColor(p.muted).LineHeight(1.5)
			}
		})
	})
}

// spotEditDialog names a new Spotify playlist, or renames one and sets
// its description.
type spotEditDialog struct {
	open       bool
	id         string // the playlist edited, or "" to make one
	name, desc string
	busy       bool
	err        string
}

func (a *app) submitSpotEdit() {
	d := &a.spot.edit
	name, desc := strings.TrimSpace(d.name), strings.TrimSpace(d.desc)
	if name == "" || d.busy {
		return
	}
	d.busy, d.err = true, ""
	id := d.id
	go func() {
		var made *spotify.Playlist
		var err error
		if id == "" {
			made, err = spotifyCreate(name, desc)
		} else {
			err = spotifyDetails(id, name, desc)
		}
		a.post(func() {
			a.spot.edit.busy = false
			if err != nil {
				a.spot.edit.err = spotErrText(err)
				return
			}
			a.spot.edit.open = false
			if id == "" {
				a.spot.lists = append([]spotify.Playlist{*made}, a.spot.lists...)
				a.open(route{page: pageSpotify, query: made.ID})
				a.loadSpotLists()
				return
			}
			if pl := a.spotPlaylist(id); pl != nil {
				pl.Name, pl.Description = name, desc
			}
			a.toast = "已保存"
		})
	}()
}

// spotImportDialog copies a Spotify playlist to NetEase, as a new
// playlist named as the listener likes, or links it with one of theirs.
type spotImportDialog struct {
	open   bool
	id     string // the Spotify playlist
	spName string
	mode   int    // 0 a new NetEase playlist, 1 one the listener has
	name   string // the new one's name
	target string // the label of the one chosen
	both   bool   // songs go both ways, not only to NetEase
	err    string
}

// openSpotImport asks how to bring Spotify playlist id to NetEase;
// existing starts on linking one the listener has.
func (a *app) openSpotImport(id, name string, existing bool) {
	d := spotImportDialog{open: true, id: id, spName: name, name: name}
	if existing {
		d.mode = 1
	}
	if l, ok := a.spotLinkOf(id); ok {
		for _, t := range a.importTargets() {
			if t.id == l.Netease {
				d.target = t.label
			}
		}
		d.both = l.Direction == crossplay.Both
	}
	a.spot.importing = d
}

type importTarget struct {
	id    int64
	label string
}

// importTargets are the listener's NetEase playlists a Spotify one may be
// linked with, their liked songs first, each labelled once.
func (a *app) importTargets() []importTarget {
	var out []importTarget
	seen := map[string]bool{}
	for _, pl := range a.mine {
		label := pl.Name
		if pl.SpecialType == 5 {
			label = "我喜欢的音乐"
		}
		if seen[label] {
			label = fmt.Sprintf("%s（%d 首）", label, pl.TrackCount)
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, importTarget{pl.ID, label})
	}
	return out
}

func (a *app) submitSpotImport() {
	d := a.spot.importing
	id := d.id
	if d.mode == 0 {
		name := strings.TrimSpace(d.name)
		if name == "" {
			return
		}
		a.spot.importing.open = false
		a.crossWork(id, func(p crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
			return crossImport(id, name, crossplay.Budget{}, p)
		}, func(err error) {
			// Nothing was made: ask again, keeping what was typed.
			d.open = true
			d.err = "网易云没有建成这个歌单：" + spotErrText(err) + "。网易云不接受部分符号、表情和敏感词，换个名字试试"
			a.spot.importing = d
		})
		return
	}
	var ne int64
	for _, t := range a.importTargets() {
		if t.label == d.target {
			ne = t.id
		}
	}
	if ne == 0 {
		return
	}
	dir := crossplay.ToNetease
	if d.both && a.spotEditable(id) {
		dir = crossplay.Both
	}
	a.spot.importing.open = false
	name := d.spName
	a.crossWork(id, func(p crossplay.Progress) (crossplay.Link, crossplay.Report, error) {
		return crossLink(id, ne, name, dir, crossplay.Budget{}, p)
	})
}

// spotChooseDialog chooses which Spotify playlists the sidebar shows.
type spotChooseDialog struct {
	open bool
	rows []spotChooseRow
}

type spotChooseRow struct {
	id, name string
	on       bool
}

func (a *app) openSpotChooser() {
	d := spotChooseDialog{open: true}
	for _, pl := range a.spotAll() {
		d.rows = append(d.rows, spotChooseRow{pl.ID, pl.Name, !a.spotHidden(pl.ID)})
	}
	a.spot.choosing = d
}

// spotDialogs are the dialogs of the listener's Spotify playlists.
func (a *app) spotDialogs(c *ui.Context) {
	p := a.pal
	panel := func(panel ui.Element, w float32) {
		panel.Width(w).Padding(24).Radius(18).Gap(14).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
	}
	buttons := func(action string, ok bool, cancel, submit func()) {
		ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				cancel()
			}
			b := pillButton(c.Key("ok"), nil, action, p.accent, p.onAccent)
			if !ok {
				b.Disabled(true).Opacity(0.5)
			}
			if b.Clicked() {
				submit()
			}
		})
	}
	errText := func(err string) {
		if err != "" {
			ui.Text(c, err).FontSize(12).TextColor(ui.Hex("#e5484d")).LineHeight(1.45)
		}
	}

	e := &a.spot.edit
	ui.DialogBase(c.Key("spotify-edit"), &e.open, func(backdrop, pn ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel(pn, 380)
		title, action := "新建 Spotify 歌单", "创建"
		if e.id != "" {
			title, action = "编辑 Spotify 歌单", "保存"
		}
		if e.busy {
			action = "正在保存…"
		}
		ui.Text(c, title).FontSize(17).FontWeight(700)
		in := ui.TextInput(c.Key("name"), &e.name).Placeholder("歌单名称").Label("歌单名称").FontSize(14).AutoFocus()
		if in.Submitted() {
			a.submitSpotEdit()
		}
		ui.TextArea(c.Key("desc"), &e.desc).Placeholder("简介（可以不填）").Label("歌单简介").FontSize(13).Height(72)
		errText(e.err)
		buttons(action, strings.TrimSpace(e.name) != "" && !e.busy, func() { e.open = false }, a.submitSpotEdit)
	})

	d := &a.spot.importing
	ui.DialogBase(c.Key("spotify-import"), &d.open, func(backdrop, pn ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel(pn, 420)
		ui.Text(c, "把“"+d.spName+"”带到网易云").FontSize(17).FontWeight(700).MaxLines(2)
		ui.Segmented(c.Key("mode"), &d.mode, "新建网易云歌单", "关联已有歌单")
		targets := a.importTargets()
		ok := false
		if d.mode == 0 {
			in := ui.TextInput(c.Key("name"), &d.name).Placeholder("网易云歌单名称").Label("网易云歌单名称").FontSize(14).AutoFocus()
			if in.Submitted() {
				a.submitSpotImport()
			}
			ui.Text(c, "默认用 Spotify 上的名字。网易云不接受部分符号、表情和敏感词，被拒的话这里改一下就行").FontSize(12).TextColor(p.muted).LineHeight(1.45)
			ok = strings.TrimSpace(d.name) != ""
		} else {
			labels := make([]string, len(targets))
			for i, t := range targets {
				labels[i] = t.label
			}
			if d.target == "" && len(labels) > 0 {
				d.target = labels[0]
			}
			if len(labels) == 0 {
				ui.Text(c, "你还没有自己的网易云歌单").FontSize(13).TextColor(p.muted)
			} else {
				ui.Select(c.Key("target"), &d.target, labels).Label("网易云歌单")
				ok = true
			}
			if a.spotEditable(d.id) {
				ui.Checkbox(c.Key("both"), &d.both, "双向同步：网易云歌单里多出来的歌也加到 Spotify")
			}
			ui.Text(c, "关联后会马上同步一次，只加不删：Spotify 上有、网易云歌单里没有的歌加进去，已经有的不会重复加").FontSize(12).TextColor(p.muted).LineHeight(1.45)
		}
		errText(d.err)
		action := "复制"
		if d.mode == 1 {
			action = "关联并同步"
		}
		buttons(action, ok, func() { d.open = false }, a.submitSpotImport)
	})

	ch := &a.spot.choosing
	ui.DialogBase(c.Key("spotify-choose"), &ch.open, func(backdrop, pn ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel(pn, 380)
		ui.Text(c, "侧边栏显示哪些 Spotify 歌单").FontSize(17).FontWeight(700)
		ui.Row(c).Gap(8).Children(func() {
			if pillButton(c.Key("all"), nil, "全部显示", p.hover.Over(p.bg), p.text).Clicked() {
				for i := range ch.rows {
					ch.rows[i].on = true
					a.setSpotHidden(ch.rows[i].id, false)
				}
			}
			if pillButton(c.Key("none"), nil, "全部隐藏", p.hover.Over(p.bg), p.text).Clicked() {
				for i := range ch.rows {
					ch.rows[i].on = false
					a.setSpotHidden(ch.rows[i].id, true)
				}
			}
		})
		ui.Scroll(c.Key("rows")).MaxHeight(360).Gap(10).Children(func() {
			for i := range ch.rows {
				r := &ch.rows[i]
				if ui.Checkbox(c.Key("row-"+r.id), &r.on, r.name).Changed() {
					a.setSpotHidden(r.id, !r.on)
				}
			}
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			if pillButton(c.Key("done"), nil, "完成", p.accent, p.onAccent).Clicked() {
				ch.open = false
			}
		})
	})
}

// spotCache is what is kept of a Spotify playlist's page, on disk, so
// that opening it again shows it without reading Spotify or searching
// NetEase: the listener updates it when they like.
type spotCache struct {
	Snapshot string          `json:"snapshot,omitempty"`
	Tracks   []spotify.Track `json:"tracks"`
	Matched  []netease.Track `json:"matched"`
	From     []string        `json:"from"`
	Missing  []spotify.Track `json:"missing"`
	Pending  int             `json:"pending,omitempty"`
	At       time.Time       `json:"at"`
}

func spotCacheOf(pg *spotPage) spotCache {
	return spotCache{Snapshot: pg.snapshot, Tracks: pg.tracks, Matched: pg.matched, From: pg.from, Missing: pg.missing, Pending: pg.pending}
}

// restore shows what was kept on pg.
func (k spotCache) restore(pg *spotPage) {
	pg.tracks, pg.matched, pg.from, pg.missing, pg.pending, pg.snapshot = k.Tracks, k.Matched, k.From, k.Missing, k.Pending, k.Snapshot
	pg.err = ""
}

// without returns what was kept, less the songs of the URIs gone. New
// slices: the player may be playing the old ones.
func (k spotCache) without(gone map[string]bool) spotCache {
	out := k
	out.Tracks = slices.DeleteFunc(slices.Clone(k.Tracks), func(t spotify.Track) bool { return gone[t.URI] })
	out.Missing = slices.DeleteFunc(slices.Clone(k.Missing), func(t spotify.Track) bool { return gone[t.URI] })
	out.Matched, out.From = nil, nil
	for i, t := range k.Matched {
		if i < len(k.From) && gone[k.From[i]] {
			continue
		}
		out.Matched = append(out.Matched, t)
		if i < len(k.From) {
			out.From = append(out.From, k.From[i])
		}
	}
	return out
}

// spotCachePath is where the page of Spotify playlist id is kept.
func spotCachePath(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return "", fmt.Errorf("bad playlist id %q", id)
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "spotify-pages", id+".json"), nil
}

// What is kept of pages, read, written and forgotten; tests replace them.
var (
	readSpotCache = func(id string) (spotCache, bool) {
		var k spotCache
		p, err := spotCachePath(id)
		if err != nil {
			return k, false
		}
		b, err := os.ReadFile(p)
		if err != nil || json.Unmarshal(b, &k) != nil {
			return spotCache{}, false
		}
		return k, true
	}
	writeSpotCache = func(id string, k spotCache) {
		p, err := spotCachePath(id)
		if err != nil {
			return
		}
		k.At = time.Now()
		b, _ := json.Marshal(k)
		os.MkdirAll(filepath.Dir(p), 0o755)
		tmp := fmt.Sprintf("%s.%d.tmp", p, os.Getpid())
		if os.WriteFile(tmp, b, 0o644) == nil {
			os.Rename(tmp, p)
		}
	}
	removeSpotCache = func(id string) {
		if p, err := spotCachePath(id); err == nil {
			os.Remove(p)
		}
	}
)
