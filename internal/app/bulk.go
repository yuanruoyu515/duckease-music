package app

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Working on many songs or playlists at once: songs picked on a
// playlist's page, then added to a playlist of the listener's, moved
// there or taken out; and the listener's playlists picked in the manager,
// then deleted, unsaved, or their songs copied or moved into one.

// playlistDetail loads a playlist with its songs; tests replace it.
var playlistDetail = netease.PlaylistDetail

// bulkPause is the wait between requests of a batch, so many of them
// don't look to NetEase like the bursts its risk control turns away.
var bulkPause = 400 * time.Millisecond

// bulkChunk is how many songs go to NetEase in one request.
const bulkChunk = 200

// trackPick is the songs picked on a playlist's page.
type trackPick struct {
	on     bool
	pid    int64          // the playlist whose songs are picked
	ids    map[int64]bool // the songs picked
	anchor int64          // the song last clicked, which Shift picks from
}

// picking reports whether songs are being picked on playlist pid's page.
func (a *app) picking(pid int64) bool { return a.pick.on && a.pick.pid == pid }

// startPicking starts picking the songs of playlist pid.
func (a *app) startPicking(pid int64) {
	a.pick = trackPick{on: true, pid: pid, ids: map[int64]bool{}}
}

func (a *app) stopPicking() { a.pick = trackPick{} }

// pickClick picks song id or drops it; with shift, it picks those listed
// from the song last clicked to it, as Finder does.
func (a *app) pickClick(listed []int64, id int64, shift bool) {
	from := slices.Index(listed, a.pick.anchor)
	to := slices.Index(listed, id)
	if shift && from >= 0 && to >= 0 {
		if from > to {
			from, to = to, from
		}
		for _, x := range listed[from : to+1] {
			a.pick.ids[x] = true
		}
	} else if a.pick.ids[id] {
		delete(a.pick.ids, id)
	} else {
		a.pick.ids[id] = true
	}
	a.pick.anchor = id
}

// pickedSongs returns the songs picked, in the order tracks lists them.
func (a *app) pickedSongs(tracks []netease.Track) []int64 {
	var ids []int64
	for _, t := range tracks {
		if a.pick.ids[t.ID] {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// listedIDs returns the ids of the tracks at shown, or all with nil.
func listedIDs(all []netease.Track, shown []int) []int64 {
	var ids []int64
	if shown == nil {
		for _, t := range all {
			ids = append(ids, t.ID)
		}
		return ids
	}
	for _, i := range shown {
		ids = append(ids, all[i].ID)
	}
	return ids
}

// addSongs adds songs to the listener's playlist pid, some at a time,
// and says how it went, naming the playlist name.
func (a *app) addSongs(pid int64, name string, songs []int64, then func(err error)) {
	go func() {
		err := addInChunks(pid, songs)
		a.post(func() {
			switch {
			case errors.Is(err, netease.ErrInPlaylist):
				a.toast = "所选歌曲已在“" + name + "”中"
			case err != nil:
				a.toast = "添加失败，请稍后再试"
			default:
				a.toast = fmt.Sprintf("已将 %d 首歌曲添加到“%s”", len(songs), name)
			}
			if err == nil || errors.Is(err, netease.ErrInPlaylist) {
				a.playlistChanged(pid)
			}
			if then != nil {
				then(err)
			}
		})
	}()
}

// addInChunks adds songs to playlist pid, bulkChunk at a time, leaving
// out those it has already: NetEase turns a whole request away for one
// song it has. It returns ErrInPlaylist when it has them all.
func addInChunks(pid int64, songs []int64) error {
	if there, err := playlistDetail(pid); err == nil {
		has := map[int64]bool{}
		for _, t := range there.TrackIDs {
			has[t.ID] = true
		}
		for _, t := range there.Tracks {
			has[t.ID] = true
		}
		songs = slices.DeleteFunc(slices.Clone(songs), func(id int64) bool { return has[id] })
		if len(songs) == 0 {
			return netease.ErrInPlaylist
		}
		time.Sleep(bulkPause)
	}
	added := 0
	for i := 0; i < len(songs); i += bulkChunk {
		if i > 0 {
			time.Sleep(bulkPause)
		}
		chunk := songs[i:min(i+bulkChunk, len(songs))]
		switch err := addToPlaylist(pid, chunk...); {
		case err == nil:
			added += len(chunk)
		case errors.Is(err, netease.ErrInPlaylist):
			// Added since it was looked at: the rest go on.
		default:
			return err
		}
	}
	if added == 0 {
		return netease.ErrInPlaylist
	}
	return nil
}

// removeSongs takes songs out of the listener's playlist pid, at once in
// the app and then with NetEase, which may refuse: they come back then.
// Quiet, it says nothing once done, as when moving them.
func (a *app) removeSongs(pid int64, songs []int64, quiet bool) {
	if a.likedList(pid) {
		a.unlikeSongs(pid, songs, quiet)
		return
	}
	pl := a.playlists[pid]
	var before []netease.Track
	if pl != nil {
		// A new list: the player may be playing the old one.
		before = pl.Tracks
		pl.Tracks = slices.DeleteFunc(slices.Clone(before), func(t netease.Track) bool { return slices.Contains(songs, t.ID) })
	}
	go func() {
		var err error
		for i := 0; i < len(songs) && err == nil; i += bulkChunk {
			if i > 0 {
				time.Sleep(bulkPause)
			}
			err = removeFromPlaylist(pid, songs[i:min(i+bulkChunk, len(songs))]...)
		}
		a.post(func() {
			if err != nil {
				if pl != nil && a.playlists[pid] == pl {
					pl.Tracks = before
				}
				a.toast = "移除失败，请稍后再试"
				return
			}
			if !quiet {
				a.toast = fmt.Sprintf("已移除 %d 首歌曲", len(songs))
			}
			if a.profile != nil {
				a.loadMine(a.profile.UserID)
			}
		})
	}()
}

// likedList reports whether pid is the listener's liked songs, whose
// songs leave it by being unliked.
func (a *app) likedList(pid int64) bool {
	return slices.ContainsFunc(a.mine, func(pl netease.Playlist) bool { return pl.ID == pid && pl.SpecialType == 5 })
}

// unlikeSongs takes songs out of the liked songs, playlist pid, by
// unliking each: at once in the app, then with NetEase one after the
// other, as a heart's click does. Those it refuses come back.
func (a *app) unlikeSongs(pid int64, songs []int64, quiet bool) {
	pl := a.playlists[pid]
	var before []netease.Track
	if pl != nil {
		before = pl.Tracks
		pl.Tracks = slices.DeleteFunc(slices.Clone(before), func(t netease.Track) bool { return slices.Contains(songs, t.ID) })
	}
	if a.liked == nil {
		a.liked = map[int64]bool{}
	}
	for _, id := range songs {
		a.setLiked(id, false)
	}
	go func() {
		var failed []int64
		for i, id := range songs {
			if i > 0 {
				time.Sleep(bulkPause)
			}
			if likeSong(id, false) != nil {
				failed = append(failed, id)
			}
		}
		a.post(func() {
			for _, id := range failed {
				a.setLiked(id, true)
			}
			if len(failed) > 0 {
				if pl != nil && a.playlists[pid] == pl {
					// Back, those NetEase still likes, where they were.
					pl.Tracks = slices.DeleteFunc(slices.Clone(before), func(t netease.Track) bool {
						return slices.Contains(songs, t.ID) && !slices.Contains(failed, t.ID)
					})
				}
				a.toast = fmt.Sprintf("%d 首歌曲没能取消喜欢，请稍后再试", len(failed))
				return
			}
			if !quiet {
				a.toast = fmt.Sprintf("已从我喜欢的音乐中移除 %d 首歌曲", len(songs))
			}
			if a.profile != nil {
				a.loadMine(a.profile.UserID)
			}
		})
	}()
}

// moveSongs adds songs to playlist to, then takes them out of from.
func (a *app) moveSongs(from, to int64, name string, songs []int64) {
	a.addSongs(to, name, songs, func(err error) {
		if err == nil || errors.Is(err, netease.ErrInPlaylist) {
			a.removeSongs(from, songs, true)
			a.toast = fmt.Sprintf("已将 %d 首歌曲移到“%s”", len(songs), name)
		}
	})
}

// targets are the listener's playlists songs may go to: theirs but the
// liked songs, and those in skip.
func (a *app) targets(skip ...int64) []*netease.Playlist {
	var to []*netease.Playlist
	for i := range a.mine {
		if pl := &a.mine[i]; pl.SpecialType != 5 && !slices.Contains(skip, pl.ID) {
			to = append(to, pl)
		}
	}
	return to
}

// pickBar floats over the foot of a playlist's page while its songs are
// picked: how many, and what to do with them.
func (a *app) pickBar(c *ui.Context) {
	if a.route.page != pagePlaylist || !a.picking(a.route.id) || a.nowPlaying {
		return
	}
	pl := a.playlists[a.route.id]
	if pl == nil {
		return
	}
	p := a.pal
	tracks := a.plFilter.tracks(pl)
	ids := a.pickedSongs(tracks)
	// The liked songs too: their songs leave by being unliked.
	own := a.editable(pl.ID) != nil || a.likedList(pl.ID)
	c.OnShortcut(0, ui.KeyEscape, a.stopPicking)
	c.OnShortcut(ui.Cmd, ui.KeyA, func() {
		for _, t := range tracks {
			a.pick.ids[t.ID] = true
		}
	})
	ui.Row(c.Key("pick-bar")).Absolute().Left(0).Right(0).Bottom(playerBarHeight + 14).Justify(ui.Center).PassThrough().Children(func() {
		bar := ui.Row(c).Height(48).Padding(0, 8, 0, 18).Gap(6).AlignItems(ui.Center).Radius(999).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.3)).
			Role(ui.RoleToolbar).Label("所选歌曲").
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Y: 16, Scale: 0.96}, Exit: &ui.Motion{Y: 16}})
		bar.Children(func() {
			ui.Textf(c, "已选 %d 首", len(ids)).FontSize(13).FontWeight(600).FontFeatures("tnum").Margin(0, 6, 0, 0)
			all := len(ids) == len(tracks) && len(tracks) > 0
			label := "全选"
			if all {
				label = "全不选"
			}
			if pillButton(c.Key("all"), nil, label, ui.Transparent, p.text).Clicked() {
				for _, t := range tracks {
					if all {
						delete(a.pick.ids, t.ID)
					} else {
						a.pick.ids[t.ID] = true
					}
				}
			}
			none := len(ids) == 0
			dim := func(b ui.Element) ui.Element {
				if none {
					b.Disabled(true).Opacity(0.4)
				}
				return b
			}
			if a.profile != nil {
				add := dim(pillButton(c.Key("add"), iconPlus, "添加到歌单", p.hover.Over(p.bg), p.text))
				add.Menu(func(m *ui.Menu) {
					if m.Item("新建歌单…").Chosen() {
						a.naming = nameDialog{open: true, addMany: ids, name: ""}
					}
					if to := a.targets(pl.ID); len(to) > 0 {
						m.Separator()
						for _, t := range to {
							if m.Item(t.Name).Chosen() {
								a.addSongs(t.ID, t.Name, ids, nil)
								a.stopPicking()
							}
						}
					}
				})
			}
			if own {
				move := dim(pillButton(c.Key("move"), iconMove, "移到歌单", p.hover.Over(p.bg), p.text))
				move.Menu(func(m *ui.Menu) {
					to := a.targets(pl.ID)
					if len(to) == 0 {
						m.Item("没有别的歌单").Disabled(true)
					}
					for _, t := range to {
						if m.Item(t.Name).Chosen() {
							a.moveSongs(pl.ID, t.ID, t.Name, ids)
							a.stopPicking()
						}
					}
				})
				if dim(pillButton(c.Key("remove"), iconTrash, "移除", p.hover.Over(p.bg), p.text)).Clicked() {
					pid := pl.ID
					a.ask = confirmDialog{open: true, title: "移除歌曲",
						body:   removeBody(pl.Name, len(ids), a.likedList(pid)),
						action: "移除",
						do: func() {
							a.removeSongs(pid, ids, false)
							a.stopPicking()
						}}
				}
			}
			if pillButton(c.Key("done"), nil, "完成", p.accent, p.onAccent).Clicked() {
				a.stopPicking()
			}
		})
	})
}

// pickMark is the circle a row shows in place of its number while songs
// are picked: filled with a check once the song is.
func (a *app) pickMark(c *ui.Context, on bool) {
	p := a.pal
	box := ui.Box(c).Size(18, 18).Radius(999).Center()
	if on {
		box.Background(p.accent)
	} else {
		box.Border(1.5, p.faint)
	}
	box.Children(func() {
		if on {
			ui.Icon(c, iconCheck).Size(12, 12).TextColor(p.onAccent)
		}
	})
}

// confirmDialog asks whether to do something that can't be undone.
type confirmDialog struct {
	open                bool
	title, body, action string
	do                  func()
}

func (a *app) confirmView(c *ui.Context) {
	p := a.pal
	d := &a.ask
	ui.DialogBase(c.Key("confirm"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(380).Padding(24).Radius(18).Gap(8).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Text(c, d.title).FontSize(17).FontWeight(700)
		ui.Text(c, d.body).FontSize(13).TextColor(p.muted).LineHeight(1.45)
		ui.Row(c).Gap(10).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
			if pillButton(c.Key("ok"), nil, d.action, ui.Hex("#e5484d"), ui.RGB(255, 255, 255)).Clicked() {
				d.open = false
				if d.do != nil {
					d.do()
				}
			}
		})
	})
}

// The playlist manager.

// managerState is the manager of the listener's playlists, as it shows.
type managerState struct {
	open  bool
	saved bool           // it lists the playlists saved, not those made
	ids   map[int64]bool // the playlists picked
	busy  string         // what it is doing, while it is
}

// openManager opens the manager on the playlists made, or saved.
func (a *app) openManager(saved bool) {
	a.manager = managerState{open: true, saved: saved, ids: map[int64]bool{}}
}

// managed are the playlists the manager lists.
func (a *app) managed() []netease.Playlist {
	if a.manager.saved {
		return a.saved
	}
	return a.mine
}

// managedPicked returns the playlists picked, in the order listed.
func (a *app) managedPicked() []netease.Playlist {
	var out []netease.Playlist
	for _, pl := range a.managed() {
		if a.manager.ids[pl.ID] {
			out = append(out, pl)
		}
	}
	return out
}

// songsOf loads the songs of the playlists from, one after the other, in
// their order, each song once.
func songsOf(from []netease.Playlist, progress func(i int)) ([]int64, error) {
	var ids []int64
	seen := map[int64]bool{}
	for i, pl := range from {
		if progress != nil {
			progress(i)
		}
		if i > 0 {
			time.Sleep(bulkPause)
		}
		d, err := playlistDetail(pl.ID)
		if err != nil {
			return nil, err
		}
		for _, t := range d.Tracks {
			if !seen[t.ID] {
				seen[t.ID] = true
				ids = append(ids, t.ID)
			}
		}
	}
	return ids, nil
}

// copyInto copies the songs of the playlists picked into the listener's
// playlist to, named name, or a new one named name with to 0; then, if
// move, deletes the playlists they came from.
func (a *app) copyInto(to int64, name string, move bool) {
	from := a.managedPicked()
	if len(from) == 0 || a.manager.busy != "" {
		return
	}
	a.manager.busy = "正在读取歌单…"
	go func() {
		ids, err := songsOf(from, func(i int) {
			a.post(func() { a.manager.busy = fmt.Sprintf("正在读取歌单 %d/%d…", i+1, len(from)) })
		})
		if err == nil && to == 0 {
			var pl *netease.Playlist
			if pl, err = createPlaylist(name); err == nil {
				to = pl.ID
				a.post(func() { a.addMine(*pl) })
			}
		}
		if err == nil && len(ids) > 0 {
			a.post(func() { a.manager.busy = fmt.Sprintf("正在添加 %d 首歌曲…", len(ids)) })
			err = addInChunks(to, ids)
			if errors.Is(err, netease.ErrInPlaylist) {
				err = nil
			}
		}
		a.post(func() {
			a.manager.busy = ""
			if err != nil {
				a.toast = "操作失败，请稍后再试"
				return
			}
			a.playlistChanged(to)
			a.manager.ids = map[int64]bool{}
			a.toast = fmt.Sprintf("已将 %d 首歌曲复制到“%s”", len(ids), name)
			if move {
				var gone []netease.Playlist
				for _, pl := range from {
					if pl.ID != to && pl.SpecialType != 5 && a.editable(pl.ID) != nil {
						gone = append(gone, pl)
					}
				}
				a.deletePlaylists(gone)
				a.toast = fmt.Sprintf("已将 %d 首歌曲迁移到“%s”", len(ids), name)
			}
		})
	}()
}

// copyEach copies each playlist picked into a new playlist of the
// listener's of the same name.
func (a *app) copyEach() {
	from := a.managedPicked()
	if len(from) == 0 || a.manager.busy != "" {
		return
	}
	go func() {
		var err error
		done := 0
		for i, src := range from {
			a.post(func() { a.manager.busy = fmt.Sprintf("正在复制 %d/%d…", i+1, len(from)) })
			var ids []int64
			if ids, err = songsOf([]netease.Playlist{src}, nil); err != nil {
				break
			}
			time.Sleep(bulkPause)
			var pl *netease.Playlist
			if pl, err = createPlaylist(src.Name); err != nil {
				break
			}
			a.post(func() { a.addMine(*pl) })
			if len(ids) > 0 {
				time.Sleep(bulkPause)
				if err = addInChunks(pl.ID, ids); errors.Is(err, netease.ErrInPlaylist) {
					err = nil
				}
				if err != nil {
					break
				}
			}
			done++
			time.Sleep(bulkPause)
		}
		a.post(func() {
			a.manager.busy = ""
			a.manager.ids = map[int64]bool{}
			if a.profile != nil {
				a.loadMine(a.profile.UserID)
			}
			if err != nil {
				a.toast = fmt.Sprintf("复制了 %d 个歌单后失败，请稍后再试", done)
				return
			}
			a.toast = fmt.Sprintf("已复制 %d 个歌单到我的歌单", done)
		})
	}()
}

// addMine lists a playlist the listener just made, under the liked songs,
// as NetEase lists the newest.
func (a *app) addMine(pl netease.Playlist) {
	at := 0
	if len(a.mine) > 0 && a.mine[0].SpecialType == 5 {
		at = 1
	}
	pl.Tracks = nil
	a.mine = slices.Insert(slices.Clone(a.mine), at, pl)
}

// deletePlaylists deletes the listener's playlists: at once from the
// sidebar, and then with NetEase, one after the other; those it refuses
// come back.
func (a *app) deletePlaylists(gone []netease.Playlist) {
	if len(gone) == 0 {
		return
	}
	ids := make([]int64, len(gone))
	for i, pl := range gone {
		ids[i] = pl.ID
	}
	before := a.mine
	a.mine = slices.DeleteFunc(slices.Clone(a.mine), func(pl netease.Playlist) bool { return slices.Contains(ids, pl.ID) })
	a.leavePlaylists(ids)
	go func() {
		var failed []int64
		for i, id := range ids {
			if i > 0 {
				time.Sleep(bulkPause)
			}
			if deletePlaylist(id) != nil {
				failed = append(failed, id)
			}
		}
		a.post(func() {
			for _, id := range ids {
				if !slices.Contains(failed, id) {
					delete(a.playlists, id)
					a.resetCover(id)
				}
			}
			if len(failed) > 0 {
				// Back where they were.
				var back []netease.Playlist
				for _, pl := range before {
					if slices.Contains(failed, pl.ID) || slices.ContainsFunc(a.mine, func(m netease.Playlist) bool { return m.ID == pl.ID }) {
						back = append(back, pl)
					}
				}
				a.mine = back
				a.toast = fmt.Sprintf("%d 个歌单删除失败，请稍后再试", len(failed))
				return
			}
			a.toast = fmt.Sprintf("已删除 %d 个歌单", len(ids))
		})
	}()
}

// leavePlaylists leaves no way back to the pages of playlists ids, and
// leaves the one showing for the page before.
func (a *app) leavePlaylists(ids []int64) {
	gone := func(r route) bool { return r.page == pagePlaylist && slices.Contains(ids, r.id) }
	a.history = slices.DeleteFunc(a.history, gone)
	a.forward = slices.DeleteFunc(a.forward, gone)
	if gone(a.route) {
		r := a.firstPage()
		if n := len(a.history); n > 0 {
			r, a.history = a.history[n-1], a.history[:n-1]
		}
		a.show(r)
	}
}

// unsaveMany unsaves the playlists picked, one after the other.
func (a *app) unsaveMany() {
	from := a.managedPicked()
	if len(from) == 0 || a.manager.busy != "" {
		return
	}
	for _, pl := range from {
		a.setSaved(pl, false, 0)
	}
	a.manager.ids = map[int64]bool{}
	go func() {
		var failed []netease.Playlist
		for i, pl := range from {
			if i > 0 {
				time.Sleep(bulkPause)
			}
			if savePlaylist(pl.ID, false) != nil {
				failed = append(failed, pl)
			}
		}
		a.post(func() {
			for _, pl := range failed {
				a.setSaved(pl, true, len(a.saved))
			}
			if len(failed) > 0 {
				a.toast = fmt.Sprintf("%d 个歌单取消收藏失败，请稍后再试", len(failed))
				return
			}
			a.toast = fmt.Sprintf("已取消收藏 %d 个歌单", len(from))
		})
	}()
}

// managerView is the manager of the listener's playlists: two tabs, of
// those made and those saved, whose playlists are picked by their boxes,
// and the actions on those picked along its foot.
func (a *app) managerView(c *ui.Context) {
	p := a.pal
	m := &a.manager
	ui.DialogBase(c.Key("playlist-manager"), &m.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		_, h := c.Size()
		panel.Width(520).Height(min(640, h-80)).Padding(22, 20, 18, 20).Radius(18).Gap(12).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "管理歌单").FontSize(17).FontWeight(700).Grow(1)
			tab := 0
			if m.saved {
				tab = 1
			}
			if segmented(c.Key("tab"), &tab, "我创建的", "我收藏的") {
				a.openManager(tab == 1)
			}
		})
		lists := a.managed()
		picked := a.managedPicked()
		ui.Row(c).AlignItems(ui.Center).Padding(0, 4).Children(func() {
			ui.Textf(c, "已选 %d 个，共 %d 个", len(picked), len(lists)).FontSize(12.5).TextColor(p.muted).Grow(1)
			all := len(picked) > 0 && len(picked) == len(lists)
			label := "全选"
			if all {
				label = "全不选"
			}
			if pillButton(c.Key("all"), nil, label, ui.Transparent, p.accent).Clicked() {
				for _, pl := range lists {
					if all {
						delete(m.ids, pl.ID)
					} else {
						m.ids[pl.ID] = true
					}
				}
			}
		})
		ui.Scroll(c.Key("lists")).Grow(1).Radius(12).Background(p.hover.Over(p.bg)).Padding(6).Gap(2).Children(func() {
			if len(lists) == 0 {
				ui.Text(c, "这里还没有歌单").FontSize(13).TextColor(p.muted).Padding(24)
			}
			for i := range lists {
				pl := &lists[i]
				on := m.ids[pl.ID]
				row := ui.ButtonBase(c.Key(pl.ID)).Height(48).PaddingX(10).Gap(12).Radius(9).AlignItems(ui.Center).
					Justify(ui.Start).FocusRing(false).Role(ui.RoleCheckBox).Checked(on).Label(pl.Name)
				switch {
				case on:
					row.Background(p.accent.Alpha(0.1))
				case row.Hovered():
					row.Background(p.hover)
				}
				row.Children(func() {
					a.pickMark(c, on)
					a.cover(c, a.plCover(pl), 80, 34, 6)
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, pl.Name).FontSize(13.5).SingleLine()
						sub := fmt.Sprintf("%d 首", pl.TrackCount)
						if m.saved && pl.Creator.Nickname != "" {
							sub += " · " + pl.Creator.Nickname
						}
						ui.Text(c, sub).FontSize(11.5).TextColor(p.muted).SingleLine()
					})
				})
				if row.Clicked() {
					if on {
						delete(m.ids, pl.ID)
					} else {
						m.ids[pl.ID] = true
					}
				}
			}
		})
		if m.busy != "" {
			ui.Text(c, m.busy).FontSize(12.5).TextColor(p.muted).Padding(0, 4)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			none := len(picked) == 0 || m.busy != ""
			act := func(key string, icon *ui.SVG, label string) ui.Element {
				b := pillButton(c.Key(key), icon, label, p.hover.Over(p.bg), p.text)
				if none {
					b.Disabled(true).Opacity(0.4)
				}
				return b
			}
			skip := make([]int64, len(picked))
			for i, pl := range picked {
				skip[i] = pl.ID
			}
			into := func(menu *ui.Menu, move bool) {
				if menu.Item("新建歌单…").Chosen() {
					name := picked[0].Name
					if len(picked) > 1 {
						name += " 等"
					}
					a.naming = nameDialog{open: true, name: name, into: true, move: move}
				}
				if to := a.targets(skip...); len(to) > 0 {
					menu.Separator()
					for _, t := range to {
						if menu.Item(t.Name).Chosen() {
							if move {
								a.askMove(t.ID, t.Name)
							} else {
								a.copyInto(t.ID, t.Name, false)
							}
						}
					}
				}
			}
			if m.saved {
				if act("copy-each", iconCopy, "复制为我的歌单").Clicked() {
					a.copyEach()
				}
				act("merge", iconPlus, "合并到…").Menu(func(menu *ui.Menu) { into(menu, false) })
				if act("unsave", iconX, "取消收藏").Clicked() {
					n := len(picked)
					a.ask = confirmDialog{open: true, title: "取消收藏", action: "取消收藏",
						body: fmt.Sprintf("取消收藏所选的 %d 个歌单？", n), do: a.unsaveMany}
				}
			} else {
				act("copy", iconCopy, "复制到…").Menu(func(menu *ui.Menu) { into(menu, false) })
				act("move", iconMove, "迁移到…").Menu(func(menu *ui.Menu) { into(menu, true) })
				var deletable []netease.Playlist
				for _, pl := range picked {
					if pl.SpecialType != 5 {
						deletable = append(deletable, pl)
					}
				}
				del := act("delete", iconTrash, "删除")
				if len(deletable) == 0 {
					del.Disabled(true).Opacity(0.4)
				}
				if del.Clicked() {
					body := fmt.Sprintf("删除所选的 %d 个歌单？删除后无法恢复。", len(deletable))
					if len(deletable) < len(picked) {
						body += "“我喜欢的音乐”不能删除，会保留。"
					}
					a.ask = confirmDialog{open: true, title: "删除歌单", action: "删除", body: body, do: func() {
						a.deletePlaylists(deletable)
						a.manager.ids = map[int64]bool{}
					}}
				}
			}
			ui.Spacer(c)
			if pillButton(c.Key("close"), nil, "完成", p.accent, p.onAccent).Clicked() {
				m.open = false
			}
		})
	})
}

// askMoveNew asks before moving the songs of the playlists picked into a
// new playlist named name, which deletes them.
func (a *app) askMoveNew(name string) {
	n := 0
	for _, pl := range a.managedPicked() {
		if pl.SpecialType != 5 {
			n++
		}
	}
	a.ask = confirmDialog{open: true, title: "迁移歌单", action: "迁移",
		body: fmt.Sprintf("新建歌单“%s”，把所选歌单的歌曲复制进去，然后删除这 %d 个原歌单？删除后无法恢复，“我喜欢的音乐”会保留。", name, n),
		do:   func() { a.copyInto(0, name, true) }}
}

// askMove asks before moving the songs of the playlists picked into
// playlist to, which deletes them.
func (a *app) askMove(to int64, name string) {
	n := 0
	for _, pl := range a.managedPicked() {
		if pl.SpecialType != 5 && pl.ID != to {
			n++
		}
	}
	a.ask = confirmDialog{open: true, title: "迁移歌单", action: "迁移",
		body: fmt.Sprintf("把所选歌单的歌曲复制到“%s”，然后删除这 %d 个原歌单？删除后无法恢复，“我喜欢的音乐”会保留。", name, n),
		do:   func() { a.copyInto(to, name, true) }}
}

// copyToMine copies playlist src, someone else's, into a new playlist of
// the listener's of the same name.
func (a *app) copyToMine(src netease.Playlist) {
	if a.profile == nil {
		a.showMain()
		a.openLogin()
		return
	}
	a.toast = "正在复制“" + src.Name + "”…"
	go func() {
		ids, err := songsOf([]netease.Playlist{src}, nil)
		var pl *netease.Playlist
		if err == nil {
			pl, err = createPlaylist(src.Name)
		}
		if err == nil && len(ids) > 0 {
			time.Sleep(bulkPause)
			if err = addInChunks(pl.ID, ids); errors.Is(err, netease.ErrInPlaylist) {
				err = nil
			}
		}
		a.post(func() {
			if pl != nil {
				a.addMine(*pl)
				if a.profile != nil {
					a.loadMine(a.profile.UserID)
				}
			}
			if err != nil {
				a.toast = "复制失败，请稍后再试"
				return
			}
			a.toast = fmt.Sprintf("已复制到我的歌单“%s”，共 %d 首", pl.Name, len(ids))
		})
	}()
}

// isMine reports whether playlist id is one the listener made, the liked
// songs' too.
func (a *app) isMine(id int64) bool {
	return slices.ContainsFunc(a.mine, func(pl netease.Playlist) bool { return pl.ID == id })
}

// coverItems are the items of a menu that change the cover of the
// listener's playlist id, kept on this Mac.
func (a *app) coverItems(m *ui.Menu, id int64) {
	if m.Item("更换封面…").Chosen() {
		a.chooseCover(id)
	}
	if _, ok := a.covers[id]; ok && m.Item("恢复默认封面").Chosen() {
		a.resetCover(id)
	}
}

// removeBody asks whether to take n songs out of the playlist name: from
// the liked songs, by unliking them.
func removeBody(name string, n int, liked bool) string {
	if liked {
		return fmt.Sprintf("从“%s”中移除所选的 %d 首歌曲？这些歌会取消喜欢。", name, n)
	}
	return fmt.Sprintf("从“%s”中移除所选的 %d 首歌曲？", name, n)
}
