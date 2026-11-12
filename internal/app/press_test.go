package app

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// With NM_PRESS_SHOTS set, renders the real views with invented songs and
// locally drawn covers, without a listener's account or network requests.
func TestPressScreenshots(t *testing.T) {
	dir := os.Getenv("NM_PRESS_SHOTS")
	if dir == "" {
		t.Skip("set NM_PRESS_SHOTS to render README screenshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fallbackUsage = func(time.Time) (int, int, time.Duration) { return 12, 40, 3*time.Minute + 12*time.Second }
	defer func() { fallbackUsage = netease.FallbackUsage }()
	for _, scene := range []string{"home", "now-playing", "stage", "queue", "playlist", "settings", "picking", "manager", "cover-picker", "sidebar", "backup", "wall", "stage-partita", "stage-tilt", "fog", "daily"} {
		for _, dark := range []bool{false, true} {
			a := pressApp()
			view, w, h := a.view, 1200, 800
			switch scene {
			case "playlist":
				pl := a.featured[len(a.featured)-1]
				a.playlists[pl.ID] = &pl
				a.route = route{page: pagePlaylist, id: pl.ID}
				a.plFilter = plFilter{id: pl.ID, sort: sortName}
			case "settings", "backup":
				view, w, h = a.settingsView, 560, 1500
				a.settings.Fallback = true
				a.backup = backupState{checked: true, at: time.Date(2026, 11, 12, 21, 30, 0, 0, time.Local)}
			case "now-playing":
				a.nowPlaying = true
				// A song the fallback source served, as its badges say.
				a.pl.SetStream(&netease.Stream{Level: "lossless", Type: "flac", Fallback: true})
			case "wall":
				a.nowPlaying, a.npPane = true, paneWall
			case "stage-partita", "stage-tilt":
				a.nowPlaying, a.npPane = true, paneStage
				a.settings.StageStyle = scene[len("stage-"):]
			case "fog":
				a.nowPlaying = true
				a.settings.Fog = true
			case "daily":
				a.route = route{page: pageDaily}
			case "picking":
				pl := a.playlists[a.mine[1].ID]
				a.route = route{page: pagePlaylist, id: pl.ID}
				a.startPicking(pl.ID)
				for _, i := range []int{1, 2, 4} {
					a.pick.ids[pl.Tracks[i].ID] = true
				}
			case "manager":
				a.route = route{page: pagePlaylist, id: a.mine[1].ID}
				a.openManager(false)
				a.manager.ids[a.mine[1].ID], a.manager.ids[a.mine[3].ID] = true, true
			case "cover-picker":
				a.route = route{page: pagePlaylist, id: a.mine[1].ID}
				a.openCoverPicker(a.mine[1].ID)
			case "sidebar":
				a.customizing = true
				a.settings.Sidebar = []sideItem{{ID: "home"}, {ID: "daily"}, {ID: "mine"}, {ID: "fm"}, {ID: "charts", Hidden: true}, {ID: "podcasts", Hidden: true}, {ID: "search"}, {ID: "saved"}}
			case "stage":
				a.nowPlaying, a.npPane = true, paneStage
			case "queue":
				a.nowPlaying, a.npPane = true, paneQueue
			}
			tt := ui.NewTester(view, w, h)
			tt.SetDark(dark)
			tt.Advance(2 * time.Second)
			tt.Frame()
			img := tt.Image()
			// The settings' groups the newest switches are in.
			if from, to := map[string]string{"settings": "播放", "backup": "备份"}[scene], map[string]string{"settings": "缓存", "backup": "更新"}[scene]; from != "" {
				f, ok := tt.Find(from)
				e, ok2 := tt.Find(to)
				if !ok || !ok2 {
					t.Fatalf("no %s group", from)
				}
				img = img.SubImage(image.Rect(0, int(f.Y-14), w, int(e.Y-10))).(*image.RGBA)
			}
			name := scene + "-light.png"
			if dark {
				name = scene + "-dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, img)
			closeErr := f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}
	}
}

func pressApp() *app {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.StageStyle = ""
	a.profile = &netease.Profile{Nickname: "Demo"}
	var songs []netease.Track
	for i, name := range []string{"晚风经过", "慢一点也没关系", "沿着海岸走", "窗边的星期天", "最后一班电车", "天亮之前", "一小段晴天", "回家的路上"} {
		url := fmt.Sprintf("press://cover/%d", i)
		img := pressCover(i)
		art := &artwork{bmp: ui.NewBitmap(img), backdrop: ui.NewBitmap(blurImage(img)), tint: dominantColor(img)}
		for _, size := range []int{64, 80, 96, 120, 160, 200, 240, 300, 320, 400, 480, 600, 800} {
			key := netease.Sized(url, size)
			a.art.art[key], a.art.art[key+"#backdrop"] = art, art
		}
		a.art.art[url], a.art.art[url+"#backdrop"] = art, art
		songs = append(songs, netease.Track{
			ID: int64(i + 1), Name: name, Artists: []netease.Artist{{Name: "DuckEase Demo"}},
			Album: netease.Album{ID: int64(i + 20), Name: "给自己一点时间", PicURL: url}, Duration: int64(218000 + i*13000),
		})
		p := netease.Playlist{ID: int64(i + 100), Name: []string{"傍晚散步", "专心做点事", "周末不赶路", "留一盏灯", "把时间放慢", "雨天备忘录", "车窗外", "给今天收个尾"}[i], CoverURL: url, TrackCount: 24 + i*3, Tracks: songs}
		a.featured = append(a.featured, p)
		if i < 4 {
			a.recPlaylists = append(a.recPlaylists, p)
		}
	}
	a.daily = songs
	// The listener's own playlists, the liked songs first, and those saved.
	a.mine = []netease.Playlist{{ID: 200, Name: "Demo喜欢的音乐", SpecialType: 5, CoverURL: "press://cover/0", TrackCount: 8}}
	for i, name := range []string{"通勤路上", "睡前听", "跑步歌单", "写作业"} {
		a.mine = append(a.mine, netease.Playlist{ID: int64(201 + i), Name: name, CoverURL: fmt.Sprintf("press://cover/%d", (i+2)%8), TrackCount: 12 + i*7})
	}
	a.saved = []netease.Playlist{a.featured[0], a.featured[3]}
	a.saved[0].Creator.Nickname, a.saved[1].Creator.Nickname = "晚风", "留灯的人"
	a.playlists[201] = &netease.Playlist{ID: 201, Name: "通勤路上", CoverURL: "press://cover/2", Tracks: songs, Creator: a.featured[0].Creator}
	a.playlists[201].Creator.Nickname = "Demo"
	a.profile.UserID = 1
	a.playlists[201].Creator.UserID = 1
	a.pl.Cue(songs)
	a.pl.SetLyrics(lyrics.Build("[00:00.00]晚风经过，灯一盏盏亮了\n[00:18.00]把没做完的事留给明天\n[00:36.00]今天就走到这里吧\n[00:54.00]慢一点，也能到家\n[01:12.00]耳机里还有一小段晴天\n[01:30.00]陪我沿着海岸走远", "", ""))
	a.pl.Hold(40 * time.Second)
	return a
}

// pressCover draws abstract sleeve art owned by this project, rather than
// redistributing an artist's album cover in the README.
func pressCover(seed int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 600, 600))
	palettes := [][3]float64{{218, 112, 86}, {47, 120, 139}, {82, 119, 88}, {109, 93, 151}, {197, 146, 63}, {84, 111, 164}, {183, 95, 121}, {68, 135, 119}}
	p := palettes[seed%len(palettes)]
	for y := range 600 {
		for x := range 600 {
			dx, dy := float64(x-300), float64(y-280)
			r := math.Hypot(dx, dy)
			light := 0.5 + 0.5*float64(600-y)/600
			if r > 100 && r < 230 {
				light += 0.27 + 0.09*math.Cos(r*0.24)
			}
			if r < 44 {
				light = 1.5
			}
			img.SetRGBA(x, y, color.RGBA{uint8(min(255, p[0]*light)), uint8(min(255, p[1]*light)), uint8(min(255, p[2]*light)), 255})
		}
	}
	return img
}
