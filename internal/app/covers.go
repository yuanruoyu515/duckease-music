package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"golang.org/x/image/draw"

	"netease-music-mygo/internal/netease"
)

// Covers the listener chose for their own playlists, kept on this Mac
// only, in covers/<id>.jpg in the user data directory: NetEase never
// sees them.

// coversDir returns where the chosen covers are kept, or "" without a
// user data directory; tests replace it.
var coversDir = userCoversDir

func userCoversDir() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "covers")
}

// localCovers reads the covers chosen in dir, by playlist: each as a
// file URL that changes as the file does, so a new one loads.
func localCovers(dir string) map[int64]string {
	out := map[int64]string{}
	if dir == "" {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		id, err := strconv.ParseInt(strings.TrimSuffix(e.Name(), ".jpg"), 10, 64)
		if err != nil || !strings.HasSuffix(e.Name(), ".jpg") {
			continue
		}
		if info, err := e.Info(); err == nil {
			out[id] = coverURL(filepath.Join(dir, e.Name()), info.ModTime().UnixNano())
		}
	}
	return out
}

// coverURL is the URL of the cover at path, as of version.
func coverURL(path string, version int64) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "v=" + strconv.FormatInt(version, 10)}
	return u.String()
}

// fetchArt returns the data of a cover: from disk for the listener's own,
// else from the image CDN.
func fetchArt(raw string) ([]byte, error) {
	if strings.HasPrefix(raw, "file://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(filepath.FromSlash(u.Path))
	}
	return netease.Fetch(raw)
}

// plCover returns the cover a playlist shows: the listener's own, if they
// chose one, else NetEase's.
func (a *app) plCover(pl *netease.Playlist) string {
	if u, ok := a.covers[pl.ID]; ok {
		return u
	}
	return pl.Cover()
}

// coverSide is the side of a cover kept, in pixels: as large as any place
// shows one.
const coverSide = 1000

// squareCover crops img to a square about its middle and scales it down
// to coverSide at most, as JPEG.
func squareCover(img image.Image) ([]byte, error) {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	if side < 1 {
		return nil, errors.New("cover: empty image")
	}
	crop := image.Rect(0, 0, side, side).Add(image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2))
	out := min(side, coverSide)
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// saveCover keeps the picture in data as playlist id's cover in dir, and
// returns its URL.
func saveCover(dir string, id int64, data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cover: %w", err)
	}
	jpg, err := squareCover(img)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, strconv.FormatInt(id, 10)+".jpg")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, jpg, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return coverURL(path, info.ModTime().UnixNano()), nil
}

// chooseCover asks the listener for a picture, and makes it playlist id's
// cover.
func (a *app) chooseCover(id int64) {
	dir := coversDir()
	if dir == "" {
		return
	}
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      win,
			Title:       "选择歌单封面",
			ButtonLabel: "设为封面",
			Message:     "封面只保存在这台电脑上，不会上传到网易云",
			Filters:     []mygo.FileFilter{{Name: "图片", Extensions: []string{"jpg", "jpeg", "png", "webp", "gif"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		var u string
		if err == nil {
			u, err = saveCover(dir, id, data)
		}
		a.post(func() {
			if err != nil {
				log.Printf("cover of %d: %v", id, err)
				a.toast = "无法使用这张图片"
				return
			}
			if a.covers == nil {
				a.covers = map[int64]string{}
			}
			a.covers[id] = u
			a.toast = "已更换封面"
		})
	}()
}

// resetCover drops the cover the listener chose for playlist id, for
// NetEase's.
func (a *app) resetCover(id int64) {
	if _, ok := a.covers[id]; !ok {
		return
	}
	if dir := coversDir(); dir != "" {
		os.Remove(filepath.Join(dir, strconv.FormatInt(id, 10)+".jpg"))
	}
	delete(a.covers, id)
}
