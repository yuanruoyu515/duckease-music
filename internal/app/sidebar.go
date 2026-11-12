package app

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// The sidebar's sections, which the listener may hide and put in another
// order (settings.Sidebar), and the editor that does it.

// sideItem is one of the sidebar's sections as the settings keep it.
type sideItem struct {
	ID     string `json:"id"`
	Hidden bool   `json:"hidden,omitempty"`
}

// sideSection is what a section of the sidebar is.
type sideSection struct {
	id    string
	label string
	icon  *ui.SVG
	page  pageKind // the page a link opens, for those that are links
}

// sideSections are the sidebar's sections, in their first order.
var sideSections = []sideSection{
	{"home", "为你推荐", iconHome, pageHome},
	{"daily", "每日推荐", iconDaily, pageDaily},
	{"fm", "私人 FM", iconRadio, pageFM},
	{"charts", "排行榜", iconCharts, pageCharts},
	{"podcasts", "播客", iconPodcast, pagePodcasts},
	{"search", "搜索结果", iconSearch, pageSearch},
	{"mine", "我的歌单", iconNote, pagePlaylist},
	{"saved", "收藏的歌单", iconHeart, pagePlaylist},
}

func sideSectionOf(id string) (sideSection, bool) {
	i := slices.IndexFunc(sideSections, func(s sideSection) bool { return s.id == id })
	if i < 0 {
		return sideSection{}, false
	}
	return sideSections[i], true
}

// sidebarOrder returns the sections in the order kept, each known one
// once: those the settings don't know of yet, as from a newer version,
// at the end, shown.
func sidebarOrder(kept []sideItem) []sideItem {
	out := make([]sideItem, 0, len(sideSections))
	for _, it := range kept {
		if _, ok := sideSectionOf(it.ID); ok && !slices.ContainsFunc(out, func(o sideItem) bool { return o.ID == it.ID }) {
			out = append(out, it)
		}
	}
	for _, s := range sideSections {
		if !slices.ContainsFunc(out, func(o sideItem) bool { return o.ID == s.id }) {
			out = append(out, sideItem{ID: s.id})
		}
	}
	return out
}

// sideShown reports whether the section id shows in the sidebar.
func (a *app) sideShown(id string) bool {
	for _, it := range a.settings.Sidebar {
		if it.ID == id {
			return !it.Hidden
		}
	}
	return true
}

// firstPage is the page the app opens on: the first link the sidebar
// shows, or the home page with none shown.
func (a *app) firstPage() route {
	for _, it := range sidebarOrder(a.settings.Sidebar) {
		if s, _ := sideSectionOf(it.ID); !it.Hidden && s.page != pagePlaylist && s.page != pageSearch {
			return route{page: s.page}
		}
	}
	return route{page: pageHome}
}

// moveSide moves the sections at rows of the order to before row to, as a
// list's Reorder says, and keeps it.
func (a *app) moveSide(rows []int, to int) {
	order := sidebarOrder(a.settings.Sidebar)
	var moved []sideItem
	for _, r := range rows {
		moved = append(moved, order[r])
	}
	var rest []sideItem
	at := len(order)
	for i, it := range order {
		if i == to {
			at = len(rest)
		}
		if !slices.Contains(rows, i) {
			rest = append(rest, it)
		}
	}
	if to >= len(order) {
		at = len(rest)
	}
	a.settings.Sidebar = slices.Insert(rest, at, moved...)
	a.keepSettings()
}

// setSideHidden hides the section id or shows it again, and keeps it.
func (a *app) setSideHidden(id string, hidden bool) {
	order := sidebarOrder(a.settings.Sidebar)
	for i := range order {
		if order[i].ID == id {
			order[i].Hidden = hidden
		}
	}
	a.settings.Sidebar = order
	a.keepSettings()
}

// sidebarSections builds the sidebar's sections, in the order and with
// those the listener chose.
func (a *app) sidebarSections(c *ui.Context) {
	for _, it := range sidebarOrder(a.settings.Sidebar) {
		if it.Hidden {
			continue
		}
		s, _ := sideSectionOf(it.ID)
		switch it.ID {
		case "search":
			if a.searched.query != "" {
				a.navItem(c, s.icon, s.label, a.searched)
			}
		case "mine":
			a.myPlaylists(c, s.label, a.mine, true, &a.mineCollapsed)
		case "saved":
			a.myPlaylists(c, s.label, a.saved, false, &a.savedCollapsed)
		default:
			a.navItem(c, s.icon, s.label, route{page: s.page})
		}
	}
	a.customizeItem(c)
}

// sideMenu is the context menu of a section of the sidebar: it hides it,
// or opens the editor.
func (a *app) sideMenu(id string) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if s, ok := sideSectionOf(id); ok && m.Item("在侧边栏隐藏“"+s.label+"”").Chosen() {
			a.setSideHidden(id, true)
		}
		if m.Item("自定义侧边栏…").Chosen() {
			a.customizing = true
		}
	}
}

// customizeItem is the quiet link at the end of the sidebar's sections
// that opens the editor.
func (a *app) customizeItem(c *ui.Context) {
	p := a.pal
	b := ui.ButtonBase(c.Key("customize-sidebar")).Height(30).PaddingX(10).Gap(10).Radius(8).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label("自定义侧边栏").Margin(14, 0, 4, 0)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, iconSliders).Size(15, 15).TextColor(p.faint)
		ui.Text(c, "自定义侧边栏").FontSize(12.5).TextColor(p.faint)
	})
	if b.Clicked() {
		a.customizing = true
	}
}

// sideRowHeight is the height of a row of the sidebar's editor.
const sideRowHeight = 44

// sidebarEditor is the dialog that turns the sidebar's sections on and
// off and puts them in order: dragged, or moved with the arrows.
func (a *app) sidebarEditor(c *ui.Context) {
	p := a.pal
	ui.DialogBase(c.Key("sidebar-editor"), &a.customizing, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(400).Padding(22, 20, 20, 20).Radius(18).Gap(12).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Column(c).Gap(4).Padding(0, 4).Children(func() {
			ui.Text(c, "自定义侧边栏").FontSize(17).FontWeight(700)
			ui.Text(c, "打开或关闭各项，拖动或用箭头调整顺序").FontSize(12.5).TextColor(p.muted)
		})
		order := sidebarOrder(a.settings.Sidebar)
		a.sideList.Reorder = a.moveSide
		a.sideList.Key = func(i int) any { return order[i].ID }
		ui.List(c.Key("sections"), &a.sideList, len(order), func(i int) {
			it := order[i]
			s, _ := sideSectionOf(it.ID)
			ui.Row(c.Key(it.ID)).Height(sideRowHeight).PaddingX(8).Gap(10).AlignItems(ui.Center).Radius(10).Children(func() {
				ui.Icon(c, iconGrip).Size(14, 14).TextColor(p.faint)
				ui.Icon(c, s.icon).Size(16, 16).TextColor(p.muted)
				label := ui.Text(c, s.label).FontSize(13.5).Grow(1).Shrink(1).MinWidth(0).SingleLine()
				if it.Hidden {
					label.TextColor(p.faint)
				}
				up := iconButton(c.Key("up"), iconChevronUp, 13, p.muted, "上移“"+s.label+"”").Size(26, 26).Radius(6)
				if i == 0 {
					up.Disabled(true).Opacity(0.3)
				}
				if up.Clicked() {
					a.moveSide([]int{i}, i-1)
				}
				down := iconButton(c.Key("down"), iconDown, 13, p.muted, "下移“"+s.label+"”").Size(26, 26).Radius(6)
				if i == len(order)-1 {
					down.Disabled(true).Opacity(0.3)
				}
				if down.Clicked() {
					a.moveSide([]int{i}, i+2)
				}
				shown := !it.Hidden
				if ui.Switch(c.Key("shown"), &shown).Label("显示“" + s.label + "”").Changed() {
					a.setSideHidden(it.ID, !shown)
				}
			})
		}).Height(float32(len(order)) * sideRowHeight).Radius(12).Background(p.hover.Over(p.bg))
		ui.Row(c).Padding(4, 8).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "底部的正在播放").FontSize(13.5)
				ui.Text(c, "在账户上方显示正在播放的歌曲").FontSize(12).TextColor(p.muted)
			})
			shown := !a.settings.HideSideNowPlaying
			if ui.Switch(c.Key("np-card"), &shown).Label("底部的正在播放").Changed() {
				a.settings.HideSideNowPlaying = !shown
				a.keepSettings()
			}
		})
		ui.Row(c).Gap(10).Justify(ui.End).Margin(4, 0, 0, 0).Children(func() {
			reset := pillButton(c.Key("reset"), nil, "恢复默认", p.hover.Over(p.bg), p.text)
			if a.settings.Sidebar == nil && !a.settings.HideSideNowPlaying {
				reset.Disabled(true).Opacity(0.5)
			}
			if reset.Clicked() {
				a.settings.Sidebar, a.settings.HideSideNowPlaying = nil, false
				a.keepSettings()
			}
			if pillButton(c.Key("done"), nil, "完成", p.accent, p.onAccent).Clicked() {
				a.customizing = false
			}
		})
	})
}

// keepSettings keeps the settings and redraws, for a change nothing else
// needs to hear of.
func (a *app) keepSettings() {
	a.settings.save()
	a.redraw()
}
