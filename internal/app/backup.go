package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// A backup of what the listener set up in the app, kept as a zip in
// iCloud Drive, made and restored only when they ask: the settings, the
// sidebar's layout, the search history and the covers of their own
// playlists. Nothing of their NetEase account: no login, no session, no
// playback where it was left.

// backupFolder and backupFile are where the backup goes in iCloud Drive.
const (
	backupFolder = "DuckEase Music"
	backupFile   = "DuckEase Music 备份.zip"
)

// backupFormat is the version of the backup's layout, which a newer one
// may change.
const backupFormat = 1

// errNoICloud is iCloud Drive turned off, or missing.
var errNoICloud = errors.New("找不到 iCloud 云盘，请在“系统设置 > Apple 账户 > iCloud”中打开 iCloud 云盘")

// errNotDownloaded is a backup iCloud keeps in the cloud only, not yet on
// this Mac.
var errNotDownloaded = errors.New("备份还没从 iCloud 下载到这台 Mac，请在访达中打开 iCloud 云盘的“" + backupFolder + "”文件夹，等它下载完再试")

// errNoBackup is no backup made yet.
var errNoBackup = errors.New("iCloud 云盘里还没有备份")

// iCloudDrive returns the folder of iCloud Drive, as macOS keeps it.
func iCloudDrive() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errNoICloud
	}
	dir := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errNoICloud
	}
	return dir, nil
}

// backupPath returns where the backup is kept in iCloud Drive.
func backupPath() (string, error) {
	drive, err := iCloudDrive()
	if err != nil {
		return "", err
	}
	return filepath.Join(drive, backupFolder, backupFile), nil
}

// backupManifest says what a backup is.
type backupManifest struct {
	App     string    `json:"app"`
	Version string    `json:"version"`
	Format  int       `json:"format"`
	Created time.Time `json:"created"`
}

// writeBackup writes a backup of s and of the covers in coversDir to
// dest, whole or not at all.
func writeBackup(dest string, s settings, coversDir string, now time.Time) error {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name string, data []byte) error {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	manifest, _ := json.MarshalIndent(backupManifest{App: "DuckEase Music", Version: appVersion(), Format: backupFormat, Created: now}, "", "  ")
	if err := add("manifest.json", manifest); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := add("settings.json", b); err != nil {
		return err
	}
	if coversDir != "" {
		entries, _ := os.ReadDir(coversDir)
		for _, e := range entries {
			if !coverFile(e.Name()) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(coversDir, e.Name()))
			if err != nil {
				return err
			}
			if err := add("covers/"+e.Name(), data); err != nil {
				return err
			}
		}
	}
	if err := z.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// coverFile reports whether name is that of a cover kept: <id>.jpg.
func coverFile(name string) bool {
	id, ok := strings.CutSuffix(name, ".jpg")
	_, err := strconv.ParseInt(id, 10, 64)
	return ok && err == nil
}

// backupContents is what a backup holds.
type backupContents struct {
	manifest backupManifest
	settings settings
	covers   map[string][]byte // by file name
}

// readBackup reads the backup at src, over the defaults for settings it
// lacks.
func readBackup(src string) (*backupContents, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			// iCloud leaves a placeholder for a file not downloaded.
			if _, e := os.Stat(filepath.Join(filepath.Dir(src), "."+filepath.Base(src)+".icloud")); e == nil {
				return nil, errNotDownloaded
			}
			return nil, errNoBackup
		}
		return nil, err
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("备份文件已损坏：%w", err)
	}
	out := &backupContents{settings: defaultSettings(), covers: map[string][]byte{}}
	var sawManifest, sawSettings bool
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(r, 32<<20))
		r.Close()
		if err != nil {
			return nil, err
		}
		switch dir, name := path.Split(f.Name); {
		case f.Name == "manifest.json":
			if err := json.Unmarshal(b, &out.manifest); err != nil {
				return nil, fmt.Errorf("备份文件已损坏：%w", err)
			}
			sawManifest = true
		case f.Name == "settings.json":
			if err := json.Unmarshal(b, &out.settings); err != nil {
				return nil, fmt.Errorf("备份文件已损坏：%w", err)
			}
			sawSettings = true
		case dir == "covers/" && coverFile(name):
			out.covers[name] = b
		}
	}
	if !sawManifest || !sawSettings {
		return nil, errors.New("这不是 DuckEase Music 的备份")
	}
	if out.manifest.Format > backupFormat {
		return nil, errors.New("这份备份来自更新版本的 DuckEase Music，请先更新再恢复")
	}
	return out, nil
}

// restoreCovers makes the covers in dir those of the backup.
func restoreCovers(dir string, covers map[string][]byte) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if coverFile(e.Name()) {
			if _, keep := covers[e.Name()]; !keep {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	for name, data := range covers {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// backupState is what the settings show of the backup.
type backupState struct {
	busy    bool
	checked bool      // the backup was looked for
	at      time.Time // when the backup in iCloud Drive was made, zero for none
	err     string    // why it can't be made or found
}

// checkBackup looks for the backup in iCloud Drive, for the settings to
// say when it was made.
func (a *app) checkBackup() {
	go func() {
		var at time.Time
		var msg string
		if p, err := backupPath(); err != nil {
			msg = err.Error()
		} else if info, err := os.Stat(p); err == nil {
			at = info.ModTime()
		}
		a.post(func() {
			a.backup.checked, a.backup.at, a.backup.err = true, at, msg
		})
	}()
}

// backupNow writes the backup to iCloud Drive.
func (a *app) backupNow() {
	if a.backup.busy {
		return
	}
	a.backup.busy = true
	s := a.settings
	dir := coversDir()
	go func() {
		p, err := backupPath()
		if err == nil {
			err = writeBackup(p, s, dir, time.Now())
		}
		a.post(func() {
			a.backup.busy = false
			if err != nil {
				log.Printf("backup: %v", err)
				a.backup.err = err.Error()
				a.notify("备份失败", err.Error())
				return
			}
			a.backup.err = ""
			a.backup.at = time.Now()
			a.notify("已备份到 iCloud 云盘", "备份在 iCloud 云盘的“"+backupFolder+"”文件夹里。")
		})
	}()
}

// restoreNow asks, then replaces what the backup holds with it.
func (a *app) restoreNow() {
	if a.backup.busy {
		return
	}
	a.backup.busy = true
	parent := a.settingsWin
	dir := coversDir()
	go func() {
		fail := func(err error) {
			a.post(func() {
				a.backup.busy = false
				a.notify("无法恢复", err.Error())
			})
		}
		p, err := backupPath()
		if err != nil {
			fail(err)
			return
		}
		b, err := readBackup(p)
		if err != nil {
			fail(err)
			return
		}
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  parent,
			Type:    mygo.MessageQuestion,
			Title:   "从 iCloud 恢复",
			Message: "用 " + b.manifest.Created.Local().Format("2006-01-02 15:04") + " 的备份替换本机的设置？",
			Detail:  "设置、侧边栏、搜索记录和自定义歌单封面会换成备份里的。网易云账号和播放列表不受影响。",
			Buttons: []string{"恢复", "取消"},
		})
		if err != nil || res.Button != 0 {
			a.post(func() { a.backup.busy = false })
			return
		}
		cerr := restoreCovers(dir, b.covers)
		a.post(func() {
			a.backup.busy = false
			a.applyRestored(b.settings, dir)
			if cerr != nil {
				log.Printf("restore covers: %v", cerr)
				a.notify("已恢复设置", "但有些歌单封面没能恢复。")
				return
			}
			a.toast = "已从 iCloud 恢复设置"
		})
	}()
}

// applyRestored makes s the settings, everywhere, and reads the covers
// in dir again.
func (a *app) applyRestored(s settings, dir string) {
	s.path = a.settings.path
	a.settings = s
	a.changed()
	a.changeAppIcon()
	a.applyCacheLimit()
	a.volume = s.Volume
	a.pl.SetVolume(a.volume)
	a.covers = localCovers(dir)
}

// notify tells the listener something over the settings window, which
// has no toasts of its own, or the main window.
func (a *app) notify(title, detail string) {
	parent := a.settingsWin
	if parent == nil {
		parent = a.win
	}
	go mygo.Dialog.Message(mygo.MessageOptions{Parent: parent, Type: mygo.MessageInfo, Message: title, Detail: detail})
}
