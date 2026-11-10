package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Versions compare by their numbers, then by this fork's number.
func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b  string
		newer bool
	}{
		{"0.1.25-duckease.3", "0.1.25-duckease.2", true},
		{"0.1.25-duckease.10", "0.1.25-duckease.9", true},
		{"v0.1.26-duckease.1", "0.1.25-duckease.9", true},
		{"0.1.25-duckease.2", "0.1.25-duckease.2", false},
		{"0.1.25-duckease.1", "0.1.25-duckease.2", false},
		{"0.1.25-duckease.1", "0.1.25", true},
		{"0.1.3-duckease.1", "0.1.25-duckease.1", false},
	} {
		if got := newerVersion(c.a, c.b); got != c.newer {
			t.Errorf("%s newer than %s: got %v", c.a, c.b, got)
		}
	}
}

// The prompt shows the release notes' first section, without Markdown's
// marks, and not the English one after it.
func TestTrimNotes(t *testing.T) {
	got := trimNotes("## 新功能\n\n- **歌单排序**：见 [README](https://x/y)。\n\n## What's new\n\n- Sorting")
	if got != "- 歌单排序：见 README。" {
		t.Errorf("got %q", got)
	}
	if long := trimNotes(strings.Repeat("长", 2000)); len([]rune(long)) != updateNotesMax+1 {
		t.Errorf("cut to %d runes", len([]rune(long)))
	}
}

// The about window's badges open the maintainer's GitHub and Threads.
func TestAboutBadges(t *testing.T) {
	var opened []string
	openURL = func(_ *ui.Context, url string) { opened = append(opened, url) }
	defer func() { openURL = func(c *ui.Context, url string) { c.OpenURL(url) } }()
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.aboutView, 380, 420)
	tt.Frame()
	for _, label := range []string{"GitHub yuanruoyu515", "Threads @realtimyry"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if len(opened) != 2 || opened[0] != githubURL || opened[1] != threadsURL {
		t.Fatalf("opened %v", opened)
	}
}
