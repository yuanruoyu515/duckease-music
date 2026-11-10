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
	for _, scene := range []string{"home", "now-playing", "stage", "queue", "playlist", "settings"} {
		for _, dark := range []bool{false, true} {
			a := pressApp()
			view, w, h := a.view, 1200, 800
			switch scene {
			case "playlist":
				pl := a.featured[len(a.featured)-1]
				a.playlists[pl.ID] = &pl
				a.route = route{page: pagePlaylist, id: pl.ID}
				a.plFilter = plFilter{id: pl.ID, sort: sortName}
			case "settings":
				view, w, h = a.settingsView, 560, 1100
			case "now-playing":
				a.nowPlaying = true
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
			if scene == "settings" {
				// The playback group, where the newest switches are.
				from, ok := tt.Find("播放")
				to, ok2 := tt.Find("缓存")
				if !ok || !ok2 {
					t.Fatal("no playback group")
				}
				img = img.SubImage(image.Rect(0, int(from.Y-14), w, int(to.Y-10))).(*image.RGBA)
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
