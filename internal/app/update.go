package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// Updates: the app asks this fork's GitHub releases whether a newer
// version is out, and offers to download it, remind later or skip it.
// It does not install anything itself: the download is the release's DMG,
// opened in the browser. Checks run 10 seconds after launch, then once a
// day, while the settings allow, and whenever the listener asks.

const (
	repoURL        = "https://github.com/yuanruoyu515/duckease-music"
	latestURL      = "https://api.github.com/repos/yuanruoyu515/duckease-music/releases/latest"
	updateFirst    = 10 * time.Second
	updateInterval = 24 * time.Hour
	updateNotesMax = 600 // runes of the release notes the prompt shows
)

// release is a version published on GitHub.
type release struct {
	Version  string // without the v
	Page     string // the release's page
	Download string // its macOS DMG, or else its page
	Notes    string
}

// latestRelease asks GitHub for the newest release; tests replace it.
var latestRelease = func() (*release, error) {
	req, err := http.NewRequest("GET", latestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DuckEase-Music/"+appVersion())
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("GitHub: " + res.Status)
	}
	var r struct {
		Tag    string `json:"tag_name"`
		Page   string `json:"html_url"`
		Body   string `json:"body"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}
	rel := &release{Version: strings.TrimPrefix(r.Tag, "v"), Page: r.Page, Download: r.Page, Notes: r.Body}
	for _, as := range r.Assets {
		if strings.HasSuffix(as.Name, ".dmg") {
			rel.Download = as.URL
		}
	}
	return rel, nil
}

// appVersion is the version running, as the build says.
func appVersion() string {
	defer func() { recover() }() // no app, as in tests
	return mygo.App.Version()
}

// newerVersion reports whether version a is newer than b: their numbers
// first, as 0.1.26 after 0.1.25, then this fork's own, as
// 0.1.25-duckease.3 after 0.1.25-duckease.2.
func newerVersion(a, b string) bool {
	pa, fa := splitVersion(a)
	pb, fb := splitVersion(b)
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return fa > fb
}

// splitVersion returns the numbers of a version, and its fork number,
// -1 for none.
func splitVersion(v string) ([]int, int) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	base, suffix, _ := strings.Cut(v, "-")
	var nums []int
	for _, s := range strings.Split(base, ".") {
		n, _ := strconv.Atoi(s)
		nums = append(nums, n)
	}
	fork := -1
	if i := strings.LastIndexByte(suffix, '.'); i >= 0 {
		if n, err := strconv.Atoi(suffix[i+1:]); err == nil {
			fork = n
		}
	}
	return nums, fork
}

// scheduleUpdates checks for updates after updateFirst, then every
// updateInterval, while the settings allow.
func (a *app) scheduleUpdates(after time.Duration) {
	time.AfterFunc(after, func() {
		a.post(func() {
			if a.settings.UpdateCheck {
				a.checkUpdate(false)
			}
			a.scheduleUpdates(updateInterval)
		})
	})
}

// checkUpdate asks for the newest release. Asked by the listener, it says
// what it found, a skipped version too; else it speaks only of a newer
// version not skipped.
func (a *app) checkUpdate(manual bool) {
	if a.updateChecking {
		return
	}
	a.updateChecking = true
	a.redraw()
	go func() {
		rel, err := latestRelease()
		a.post(func() {
			a.updateChecking = false
			a.redraw()
			current := appVersion()
			switch {
			case err != nil:
				if manual {
					a.message(mygo.MessageWarning, "无法检查更新", "连不上 GitHub，请稍后再试。\n\n"+err.Error())
				}
			case !newerVersion(rel.Version, current):
				if manual {
					a.message(mygo.MessageInfo, "已是最新版本", "DuckEase Music "+current+" 是目前最新的版本。")
				}
			case manual || rel.Version != a.settings.SkippedVersion:
				a.offerUpdate(rel, current)
			}
		})
	}()
}

// offerUpdate shows a newer release, to download, remind later or skip.
func (a *app) offerUpdate(rel *release, current string) {
	detail := "你现在用的是 " + current + "。"
	if notes := trimNotes(rel.Notes); notes != "" {
		detail += "\n\n" + notes
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:        a.win,
			Type:          mygo.MessageInfo,
			Title:         "发现新版本",
			Message:       "DuckEase Music " + rel.Version + " 可以下载了",
			Detail:        detail,
			Buttons:       []string{"下载", "稍后提醒", "忽略此版本"},
			DefaultButton: 0,
			CancelButton:  1,
		})
		if err != nil {
			return
		}
		a.post(func() {
			switch res.Button {
			case 0:
				mygo.Shell.OpenExternal(rel.Download)
			case 2:
				a.settings.SkippedVersion = rel.Version
				a.settings.save()
			}
		})
	}()
}

// trimNotes shortens release notes for the prompt: their first section,
// without Markdown's marks, cut at updateNotesMax.
func trimNotes(notes string) string {
	var lines []string
	sections := 0
	for _, l := range strings.Split(strings.ReplaceAll(notes, "\r", ""), "\n") {
		if strings.HasPrefix(l, "## ") {
			sections++
			if sections > 1 {
				break
			}
			continue
		}
		l = mdLink.ReplaceAllString(strings.NewReplacer("**", "", "`", "").Replace(l), "$1")
		lines = append(lines, l)
	}
	s := strings.TrimSpace(strings.Join(lines, "\n"))
	if r := []rune(s); len(r) > updateNotesMax {
		s = string(r[:updateNotesMax]) + "…"
	}
	return s
}

// mdLink is a Markdown link, whose text the prompt keeps.
var mdLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)

// message shows a note with an OK button, without waiting for it.
func (a *app) message(typ mygo.MessageType, msg, detail string) {
	go mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: typ, Message: msg, Detail: detail, Buttons: []string{"好"}})
}
