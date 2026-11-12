package app

import (
	"log"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The picker of a playlist's cover: the album covers of its songs, found
// by a search, or a picture from disk.

// coverPicker is the picker as it shows.
type coverPicker struct {
	open   bool
	pid    int64  // the playlist whose cover is chosen
	query  string // what its songs are searched for
	saving string // the cover being kept, while it is
}

// fetchSongCover downloads an album cover; tests replace it.
var fetchSongCover = fetchArt

// coverPickerMax is how many covers the picker shows at once: a search
// finds the others.
const coverPickerMax = 120

// openCoverPicker opens the picker for playlist id, loading its songs.
func (a *app) openCoverPicker(id int64) {
	a.picker = coverPicker{open: true, pid: id}
	a.loadPlaylist(id, false)
}

// pickerCovers returns the album covers of the songs of tracks that query
// finds, each once, in the order of the songs, with the song each is of.
func pickerCovers(tracks []netease.Track, query string) (urls []string, of []*netease.Track) {
	q := strings.ToLower(strings.TrimSpace(query))
	seen := map[string]bool{}
	for i := range tracks {
		tr := &tracks[i]
		u := tr.Album.PicURL
		if u == "" || seen[u] {
			continue
		}
		if q != "" {
			hay := strings.ToLower(tr.Name + "\n" + tr.ArtistNames() + "\n" + tr.Album.Name)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		seen[u] = true
		urls = append(urls, u)
		of = append(of, tr)
	}
	return urls, of
}

// useSongCover makes the album cover at url playlist id's cover, kept on
// this Mac as one from disk is.
func (a *app) useSongCover(id int64, url string) {
	dir := coversDir()
	if dir == "" || a.picker.saving != "" {
		return
	}
	a.picker.saving = url
	go func() {
		data, err := fetchSongCover(netease.Sized(url, coverSide))
		var u string
		if err == nil {
			u, err = saveCover(dir, id, data)
		}
		a.post(func() {
			a.picker.saving = ""
			if err != nil {
				log.Printf("cover of %d from %s: %v", id, url, err)
				a.toast = "封面下载失败，请稍后再试"
				return
			}
			if a.covers == nil {
				a.covers = map[int64]string{}
			}
			a.covers[id] = u
			a.picker.open = false
			a.toast = "已更换封面"
		})
	}()
}

// coverPickerView is the picker's dialog.
func (a *app) coverPickerView(c *ui.Context) {
	p := a.pal
	d := &a.picker
	ui.DialogBase(c.Key("cover-picker"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		_, h := c.Size()
		panel.Width(600).Height(min(640, h-80)).Padding(22, 20, 18, 20).Radius(18).Gap(12).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "更换封面").FontSize(17).FontWeight(700)
			ui.Text(c, "选一首歌的专辑封面，或用本机的图片。封面只保存在这台电脑上").FontSize(12.5).TextColor(p.muted)
		})
		field := ui.Row(c).Height(34).PaddingX(12).Gap(7).Radius(999).AlignItems(ui.Center).Background(p.hover).Border(1, p.hairline)
		field.Children(func() {
			ui.Icon(c, iconSearch).Size(14, 14).TextColor(p.muted)
			ui.TextInputBase(c.Key("cover-search"), &d.query).Grow(1).MinWidth(0).AutoFocus().
				Placeholder("搜索歌名、歌手或专辑").FontSize(13).Label("搜索歌单内歌曲的封面")
			if d.query != "" {
				iconButton(c.Key("cover-search-clear"), iconX, 12, p.muted, "清除").KeepFocus().OnClick(func() { d.query = "" })
			}
		})
		pl := a.playlists[d.pid]
		ui.Scroll(c.Key("covers")).Grow(1).Radius(12).Background(p.hover.Over(p.bg)).Padding(12).Children(func() {
			switch {
			case pl == nil && a.loadingPl[d.pid]:
				ui.Text(c, "正在加载歌单…").FontSize(13).TextColor(p.muted).Padding(24)
				return
			case pl == nil:
				ui.Text(c, "歌单加载失败").FontSize(13).TextColor(p.muted).Padding(24)
				return
			}
			urls, of := pickerCovers(pl.Tracks, d.query)
			if len(urls) == 0 {
				msg := "歌单里没有歌曲封面"
				if strings.TrimSpace(d.query) != "" {
					msg = "没有找到“" + strings.TrimSpace(d.query) + "”"
				}
				ui.Text(c, msg).FontSize(13).TextColor(p.muted).Padding(24)
				return
			}
			more := len(urls) - coverPickerMax
			if more > 0 {
				urls, of = urls[:coverPickerMax], of[:coverPickerMax]
			}
			ui.Grid(c).Columns(5).Gap(10).Children(func() {
				for i, u := range urls {
					tr := of[i]
					b := ui.ButtonBase(c.Key(u)).AspectRatio(1).Radius(8).FocusRing(false).
						Label(tr.Name + " · " + tr.Album.Name).Tooltip(tr.Name + " — " + tr.ArtistNames())
					b.Children(func() {
						a.coverFill(c, u, 200, 8)
						if b.Hovered() || d.saving == u {
							ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(8).PassThrough().
								Border(2.5, p.accent).Background(ui.RGBA(0, 0, 0, 0.12))
						}
					})
					if b.Clicked() {
						a.useSongCover(d.pid, u)
					}
				}
			})
			if more > 0 {
				ui.Textf(c, "还有 %d 张，输入关键词缩小范围", more).FontSize(12).TextColor(p.muted).Margin(12, 0, 0, 0)
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if pillButton(c.Key("local"), iconPlus, "从本机选择图片…", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
				a.chooseCover(d.pid)
			}
			if _, ok := a.covers[d.pid]; ok && pillButton(c.Key("reset"), nil, "恢复默认", p.hover.Over(p.bg), p.text).Clicked() {
				a.resetCover(d.pid)
			}
			ui.Spacer(c)
			if d.saving != "" {
				ui.Text(c, "正在保存…").FontSize(12.5).TextColor(p.muted)
			}
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
		})
	})
}
