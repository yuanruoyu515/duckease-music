package app

import (
	"bytes"
	_ "embed"
	"image/png"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The about window: the app's icon, name and version, where it comes from,
// and links to its maintainer, as badges.

//go:embed about-icon.png
var aboutIconPNG []byte

var aboutIcon = func() *ui.Bitmap {
	img, err := png.Decode(bytes.NewReader(aboutIconPNG))
	if err != nil {
		return nil
	}
	return ui.NewBitmap(img)
}()

const (
	githubURL  = "https://github.com/yuanruoyu515"
	threadsURL = "https://www.threads.com/@realtimyry"
)

var (
	logoGitHub  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.070 3.633 17.700 3.633 17.700c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.070 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.930 0-1.310.465-2.380 1.235-3.220-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.230.96-.267 1.980-.399 3-.405 1.020.006 2.040.138 3 .405 2.280-1.552 3.285-1.230 3.285-1.230.645 1.653.24 2.873.12 3.176.765.84 1.230 1.910 1.230 3.220 0 4.610-2.805 5.625-5.475 5.920.42.36.81 1.096.81 2.220 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>`))
	logoThreads = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12.186 24h-.007c-3.581-.024-6.334-1.205-8.184-3.509C2.35 18.44 1.5 15.586 1.472 12.01v-.017c.03-3.579.879-6.43 2.525-8.482C5.845 1.205 8.6.024 12.18 0h.014c2.746.02 5.043.725 6.826 2.098 1.677 1.29 2.858 3.13 3.509 5.467l-2.04.569c-1.104-3.96-3.898-5.984-8.304-6.015-2.91.022-5.11.936-6.54 2.717C4.307 6.504 3.616 8.914 3.589 12c.027 3.086.718 5.496 2.057 7.164 1.43 1.783 3.631 2.698 6.54 2.717 2.623-.02 4.358-.631 5.8-2.045 1.647-1.613 1.618-3.593 1.09-4.798-.31-.71-.873-1.3-1.634-1.75-.192 1.352-.622 2.446-1.284 3.272-.886 1.102-2.14 1.704-3.730 1.790-1.202.065-2.361-.218-3.259-.801-1.063-.689-1.685-1.740-1.752-2.964-.065-1.190.408-2.285 1.330-3.082.880-.760 2.119-1.207 3.583-1.291a13.853 13.853 0 0 1 3.020.142c-.126-.742-.375-1.332-.750-1.757-.513-.586-1.308-.883-2.359-.890h-.029c-.844 0-1.992.232-2.721 1.320L7.734 7.847c.980-1.454 2.568-2.256 4.478-2.256h.044c3.194.020 5.097 1.975 5.287 5.388.108.046.216.094.321.142 1.490.700 2.580 1.761 3.154 3.070.797 1.820.871 4.790-1.548 7.158-1.850 1.810-4.094 2.628-7.277 2.650Zm1.003-11.690c-.242 0-.487.007-.739.021-1.836.103-2.980.946-2.916 2.143.067 1.256 1.452 1.839 2.784 1.767 1.224-.065 2.818-.543 3.086-3.710a10.5 10.5 0 0 0-2.215-.221z"/></svg>`))
)

// openURL opens a link in the browser; tests replace it.
var openURL = func(c *ui.Context, url string) { c.OpenURL(url) }

// openAbout shows the about window, opening it the first time.
func (a *app) openAbout() {
	if a.aboutWin != nil {
		a.aboutWin.Show()
		a.aboutWin.Focus()
		return
	}
	a.aboutWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           "关于 DuckEase Music",
		Width:           380,
		Height:          420,
		DisableResize:   true,
		DisableMaximize: true,
		DisableMinimize: true,
		AutoHideMenuBar: true,
		BackgroundColor: "light-dark(#ffffff, #1c1c1f)",
		Content:         ui.View(a.aboutView),
	})
	a.aboutWin.OnClosed(func() {
		a.aboutWin = nil
		a.syncDock()
	})
	a.syncDock()
}

func (a *app) aboutView(c *ui.Context) {
	t := *c.Theme()
	p := paletteFor(&t)
	t.Accent, t.AccentText = p.accent, p.onAccent
	c.SetTheme(&t)
	c.Root().Background(p.bg)
	c.OnShortcut(ui.Cmd, ui.KeyW, func() { a.aboutWin.Close() })
	ui.Column(c).Fill().Padding(36, 28, 28, 28).AlignItems(ui.Center).Gap(6).Children(func() {
		if aboutIcon != nil {
			ui.Image(c, aboutIcon).Size(112, 112).Margin(0, 0, 8, 0)
		}
		ui.Text(c, "DuckEase Music").FontSize(20).FontWeight(700)
		ui.Text(c, "版本 "+appVersion()).FontSize(12.5).TextColor(p.muted).FontFeatures("tnum")
		ui.Text(c, "MyGo 云音乐的个人维护版，原作 egoist 及原项目贡献者").FontSize(12).TextColor(p.muted).TextAlign(ui.Center).LineHeight(1.45).Margin(10, 0, 0, 0)
		ui.Column(c).Gap(10).AlignItems(ui.Center).Margin(18, 0, 0, 0).Children(func() {
			badge(c.Key("github"), c.Theme().Dark, p.accent, logoGitHub, "GitHub", "yuanruoyu515", githubURL)
			badge(c.Key("threads"), c.Theme().Dark, p.accent, logoThreads, "Threads", "@realtimyry", threadsURL)
		})
		ui.Box(c).Grow(1)
		label := "检查更新"
		if a.updateChecking {
			label = "正在检查…"
		}
		if ui.Button(c.Key("check"), label).Disabled(a.updateChecking).Clicked() {
			a.checkUpdate(true)
		}
	})
}

// badge is a link drawn as a pill of two parts, as shields.io draws them:
// a dark one with the site's logo and name, and one of red with
// who it is, rounded at both ends.
func badge(c *ui.Context, darkMode bool, red ui.Color, logo *ui.SVG, site, who, url string) {
	b := ui.ButtonBase(c).Height(30).Radius(999).Clip().FocusRing(false).Label(site + " " + who).Row().AlignItems(ui.Stretch)
	lift := 0.0
	if b.Hovered() {
		lift = 0.12
	}
	white := ui.RGB(255, 255, 255)
	// Dark on light, and a lighter grey on dark, to stand off it.
	dark := ui.RGB(24, 24, 27)
	if darkMode {
		dark = ui.RGB(58, 58, 64)
	}
	b.Children(func() {
		ui.Row(c).PaddingX(12).Gap(7).AlignItems(ui.Center).Background(dark.Mix(white, float32(lift))).Children(func() {
			ui.Icon(c, logo).Size(14, 14).TextColor(white)
			ui.Text(c, site).FontSize(12.5).FontWeight(600).TextColor(white)
		})
		ui.Row(c).PaddingX(12).AlignItems(ui.Center).Background(red.Mix(white, float32(lift))).Children(func() {
			ui.Text(c, who).FontSize(12.5).FontWeight(600).TextColor(white)
		})
	})
	if b.Clicked() {
		openURL(c, url)
	}
}
