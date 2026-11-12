package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// openSettings shows the settings window, opening it the first time.
func (a *app) openSettings() {
	if a.settingsWin != nil {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.measureCache()
		a.checkBackup()
		return
	}
	a.measureCache()
	a.checkBackup()
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           "设置",
		Width:           560,
		Height:          720,
		MinWidth:        480,
		MinHeight:       420,
		StateKey:        "settings",
		DisableMaximize: true,
		DisableMinimize: true,
		// The app's menu has nothing for this window: on Linux and
		// Windows it stays out of sight until Alt shows it.
		AutoHideMenuBar: true,
		BackgroundColor: "light-dark(#f4f4f6, #1c1c1f)",
		Content:         ui.View(a.settingsView),
	})
	a.settingsWin.OnClosed(func() {
		a.settingsWin = nil
		a.syncDock()
	})
	a.syncDock()
}

// changed applies the settings everywhere and keeps them.
func (a *app) changed() {
	a.settings.save()
	a.settings.applyAppearance()
	netease.Fallback.Store(a.settings.Fallback)
	a.redraw()
}

func (a *app) settingsView(c *ui.Context) {
	t := *c.Theme()
	p := paletteFor(&t)
	t.Accent, t.AccentText = p.accent, p.onAccent
	c.SetTheme(&t)
	c.Root().Background(p.sidebar)
	c.OnShortcut(ui.Cmd, ui.KeyW, func() { a.settingsWin.Close() })
	s := &a.settings

	ui.Scroll(c).Fill().Padding(24, 28, 32, 28).Gap(8).Children(func() {
		group := func(title string, rows func()) {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.muted).Margin(14, 4, 4, 4)
			ui.Column(c).Radius(12).Background(p.bg).Border(1, p.hairline).Dividers(1, p.hairline).Children(rows)
		}
		row := func(label, desc string, control func()) {
			ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(48).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, label).FontSize(13.5).FontWeight(500)
					if desc != "" {
						ui.Text(c, desc).FontSize(12).TextColor(p.muted).LineHeight(1.4)
					}
				})
				control()
			})
		}

		// Only on macOS: elsewhere the icon is always in the notification
		// area.
		if runtime.GOOS == "darwin" {
			group("通用", func() {
				row("应用图标", "显示在菜单栏时，关闭窗口后可从菜单栏图标打开", func() {
					labels := make([]string, len(appIcons))
					chosen := appIcons[0].label
					for i, ic := range appIcons {
						labels[i] = ic.label
						if ic.id == s.AppIcon {
							chosen = ic.label
						}
					}
					if ui.Select(c.Key("app-icon"), &chosen, labels).Label("应用图标").Width(150).Changed() {
						for _, ic := range appIcons {
							if ic.label == chosen {
								s.AppIcon = ic.id
							}
						}
						a.changed()
						a.changeAppIcon()
					}
				})
			})
		}

		group("播放", func() {
			row("音质", "无损与 Hi-Res 需要黑胶 VIP，没有时播放所能播放的最高音质；下一首歌曲开始生效", func() {
				labels := make([]string, len(qualityLevels))
				chosen := qualityLevels[2].label
				for i, q := range qualityLevels {
					labels[i] = q.label
					if q.level == s.Quality {
						chosen = q.label
					}
				}
				if ui.Select(c.Key("quality"), &chosen, labels).Label("音质").Width(150).Changed() {
					for _, q := range qualityLevels {
						if q.label == chosen {
							s.Quality = q.level
						}
					}
					a.changed()
				}
			})
			row("备用音源", "网易云加载失败的歌曲和歌单里的灰色歌曲，改从备用音源播放，两边都没有才提示无法播放。音源来自 "+netease.FallbackCredit+"，是第三方免费服务，仅供个人学习，可能随时停止；只在需要时请求，每 5 分钟最多 40 次", func() {
				if ui.Switch(c.Key("fallback"), &s.Fallback).Label("备用音源").Changed() {
					a.changed()
				}
			})
			if s.Fallback {
				a.fallbackQuota(c, p)
			}
			row("显示评论", "在播放页显示歌曲评论；关闭后不再加载评论，间奏热评也一并关闭", func() {
				show := !s.HideComments
				if ui.Switch(c.Key("comments"), &show).Label("显示评论").Changed() {
					s.HideComments = !show
					if s.HideComments && a.npPane == paneComments {
						a.npPane = a.lyricsPaneChosen()
					}
					a.changed()
				}
			})
			row("间奏时显示热评", "前奏、间奏与尾奏时，歌词处换成这首歌的热门评论", func() {
				if ui.Switch(c.Key("hot-comments"), &s.HotComments).Label("间奏时显示热评").Disabled(s.HideComments).Changed() {
					a.changed()
				}
			})
			row("音量均衡", "按网易云音乐测得的响度调整每首歌的音量，让歌曲听起来一样响", func() {
				if ui.Switch(c.Key("normalize"), &s.Normalize).Label("音量均衡").Changed() {
					a.pl.SetVolume(a.volume)
					a.changed()
				}
			})
		})

		group("缓存", func() {
			row("歌曲缓存", "听过的歌曲保存在本机，再次播放无需等待，也不耗流量；"+a.cacheUsage(), func() {
				labels := make([]string, len(cacheLimits))
				chosen := cacheLimits[2].label
				for i, l := range cacheLimits {
					labels[i] = l.label
					if l.mb == s.CacheMB {
						chosen = l.label
					}
				}
				if ui.Select(c.Key("cache"), &chosen, labels).Label("歌曲缓存").Width(150).Changed() {
					for _, l := range cacheLimits {
						if l.label == chosen {
							s.CacheMB = l.mb
						}
					}
					a.changed()
					a.applyCacheLimit()
				}
			})
			row("清除缓存", "删除保存在本机的全部歌曲", func() {
				if ui.Button(c.Key("clear-cache"), "清除").Disabled(a.cacheUsed <= 0).Clicked() {
					a.clearCache()
				}
			})
		})

		group("外观", func() {
			row("外观", "", func() {
				modes := []string{"system", "light", "dark"}
				i := 0
				for k, m := range modes {
					if m == s.Appearance {
						i = k
					}
				}
				if segmented(c.Key("appearance"), &i, "跟随系统", "浅色", "深色") {
					s.Appearance = modes[i]
					a.changed()
				}
			})
		})

		group("实验性功能", func() {
			row("进度条烟雾特效", "播放时进度条化作拖着烟雾的光点，随音乐起伏；会多用一些电量", func() {
				if ui.Switch(c.Key("smoke"), &s.Smoke).Label("进度条烟雾特效").Changed() {
					a.changed()
				}
			})
			row("烟雾背景", "播放页的背景化作封面色调的烟雾，缓缓升起，随低音起伏；仍在打磨，会多用一些电量", func() {
				if ui.Switch(c.Key("fog"), &s.Fog).Label("烟雾背景").Changed() {
					a.changed()
				}
			})
		})

		// iCloud Drive is macOS's.
		if runtime.GOOS == "darwin" {
			group("备份", func() {
				b := &a.backup
				desc := "把本机的设置、侧边栏布局、搜索记录和自定义歌单封面存到 iCloud 云盘的“" + backupFolder + "”文件夹，换电脑或重装后可以恢复。不包含网易云账号和登录信息。"
				switch {
				case b.err != "":
					desc += "\n" + b.err
				case !b.at.IsZero():
					desc += "\n上次备份：" + b.at.Local().Format("2006-01-02 15:04")
				case b.checked:
					desc += "\n还没有备份过"
				}
				row("备份到 iCloud 云盘", desc, func() {
					label := "立即备份"
					if b.busy {
						label = "请稍候…"
					}
					if ui.Button(c.Key("backup"), label).Disabled(b.busy).Clicked() {
						a.backupNow()
					}
				})
				row("从 iCloud 恢复", "用 iCloud 云盘里的备份替换本机的上述设置，恢复前会再确认一次", func() {
					if ui.Button(c.Key("restore"), "恢复…").Disabled(b.busy || b.checked && b.at.IsZero()).Clicked() {
						a.restoreNow()
					}
				})
			})
		}

		group("更新", func() {
			row("自动检查更新", "启动后和每天检查一次 GitHub 上的新版本，有新版本时提示下载，也可以忽略这个版本", func() {
				if ui.Switch(c.Key("update-check"), &s.UpdateCheck).Label("自动检查更新").Changed() {
					a.changed()
				}
			})
			row("当前版本 "+appVersion(), "", func() {
				label := "检查更新"
				if a.updateChecking {
					label = "正在检查…"
				}
				if ui.Button(c.Key("check-update"), label).Disabled(a.updateChecking).Clicked() {
					a.checkUpdate(true)
				}
			})
		})

		group("账号", func() {
			if a.profile == nil {
				row("未登录", "登录后，每日推荐与歌单将根据你的口味生成", func() {
					if ui.PrimaryButton(c.Key("login"), "扫码登录").Clicked() {
						a.showMain()
						a.openLogin()
						a.win.Invalidate()
					}
				})
				return
			}
			prof := a.profile
			ui.Row(c).Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				a.avatar(c, prof.AvatarURL, 36, p.press)
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, prof.Nickname).FontSize(13.5).FontWeight(600)
					ui.Text(c, "已登录网易云音乐").FontSize(12).TextColor(p.muted)
				})
				if ui.Button(c.Key("logout"), "退出登录").Clicked() {
					a.signOut()
				}
			})
		})
	})
}

// fallbackUsage reads the fallback source's allowance; the README's
// screenshots replace it.
var fallbackUsage = netease.FallbackUsage

// fallbackQuota shows how much of the fallback source's five minutes'
// allowance is used: a bar filling toward the limit, and when a request
// frees up again.
func (a *app) fallbackQuota(c *ui.Context, p palette) {
	used, limit, freeIn := fallbackUsage(c.Now())
	if freeIn > 0 {
		// Drawn again each second, for the countdown, until the oldest
		// request stops counting.
		c.After(min(freeIn, time.Second))
	}
	frac := float32(used) / float32(limit)
	color := p.accent
	switch {
	case used >= limit:
		color = ui.Hex("#e5484d")
	case frac >= 0.75:
		color = ui.Hex("#f5a524")
	}
	ui.Column(c.Key("fallback-quota")).Padding(10, 16, 12, 16).Gap(7).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "5 分钟限额").FontSize(12).TextColor(p.muted).Grow(1)
			ui.Textf(c, "%d / %d 次", used, limit).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
		})
		track := p.press.Over(p.bg)
		ui.Box(c).Height(6).Label(fmt.Sprintf("备用音源 5 分钟内已用 %d 次，共 %d 次", used, limit)).
			Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Fill(r, track, r.H/2)
				if used > 0 {
					w := max(r.H, r.W*min(frac, 1))
					pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, color, r.H/2)
				}
			})
		note := "还没有请求过备用音源"
		switch {
		case used >= limit:
			note = "已用完，" + waitText(freeIn) + "后恢复 1 次"
		case used > 0:
			note = "最早的一次请求将在 " + waitText(freeIn) + "后释放"
		}
		ui.Text(c, note).FontSize(11.5).TextColor(p.faint)
	})
}

// waitText says how long d is, to the second under a minute and to the
// minute above it.
func waitText(d time.Duration) string {
	s := int((d + time.Second - 1) / time.Second)
	if s < 60 {
		return fmt.Sprintf("%d 秒", s)
	}
	return fmt.Sprintf("%d 分 %d 秒", s/60, s%60)
}
