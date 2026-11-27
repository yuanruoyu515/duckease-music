package app

import (
	"image/png"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Connecting Codex adds the server's table once, replacing one there,
// and keeps the rest of its settings; removing takes only it out.
func TestCodexConfigKeepsTheRest(t *testing.T) {
	old := "model = \"gpt-5\"\n\n[mcp_servers.duckease]\ncommand = \"/old\"\nargs = [\"mcp\"]\n\n[mcp_servers.other]\ncommand = \"x\"\n"
	got := tomlWithServer(old, `/Applications/DuckEase Music.app/Contents/MacOS/DuckEase Music`)
	if strings.Count(got, "[mcp_servers.duckease]") != 1 || strings.Contains(got, "/old") ||
		!strings.Contains(got, "[mcp_servers.other]") || !strings.Contains(got, `model = "gpt-5"`) ||
		!strings.Contains(got, `command = "/Applications/DuckEase Music.app/Contents/MacOS/DuckEase Music"`) {
		t.Errorf("got:\n%s", got)
	}
	if back := tomlWithoutServer(got); strings.Contains(back, "duckease") || !strings.Contains(back, "[mcp_servers.other]") {
		t.Errorf("removed:\n%s", back)
	}
}

// Connecting an agent with a JSON file keeps its other servers and
// settings, and refuses a file that isn't JSON instead of emptying it.
func TestJSONConfigKeepsTheRest(t *testing.T) {
	got, err := jsonWithServer(`{"globalShortcut":"Cmd+K","mcpServers":{"fs":{"command":"npx"}}}`, "/a/b")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		GlobalShortcut string
		MCPServers     map[string]struct {
			Command string
			Args    []string
		}
	}
	json.Unmarshal([]byte(got), &v)
	if v.GlobalShortcut != "Cmd+K" || v.MCPServers["fs"].Command != "npx" || v.MCPServers["duckease"].Command != "/a/b" {
		t.Errorf("got %s", got)
	}
	if _, err := jsonWithServer(`{"mcpServers": {`, "/a/b"); err == nil {
		t.Error("a broken file was written over")
	}
}

// The dialog, opened from the account menu, connects Codex with a click,
// writing its settings and keeping a copy of the old ones.
func TestConnectCodexFromAccountMenu(t *testing.T) {
	home := t.TempDir()
	h := homeDir
	defer func() { homeDir = h }()
	homeDir = func() (string, error) { return home, nil }
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	os.WriteFile(filepath.Join(home, ".codex/config.toml"), []byte("model = \"x\"\n"), 0o644)

	a := spotApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("me"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("连接 AI Agent…"); err != nil {
		t.Fatal(err)
	}
	a.agents.installed, a.agents.connected = map[string]bool{"codex": true}, map[string]bool{}
	tt.Frame()
	if err := tt.Click("一键接入 Codex"); err != nil {
		t.Fatal(err)
	}
	// The write runs off the main thread.
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if b, _ := os.ReadFile(filepath.Join(home, ".codex/config.toml")); strings.Contains(string(b), "[mcp_servers.duckease]") {
			break
		}
	}
	b, _ := os.ReadFile(filepath.Join(home, ".codex/config.toml"))
	if !strings.Contains(string(b), "[mcp_servers.duckease]") || !strings.Contains(string(b), `model = "x"`) {
		t.Errorf("config:\n%s", b)
	}
	if bak, _ := os.ReadFile(filepath.Join(home, ".codex/config.toml.duckease-backup")); string(bak) != "model = \"x\"\n" {
		t.Errorf("backup %q", bak)
	}
}

// With NM_FRAMES set, saves the dialog that connects agents.
func TestAgentsFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("NM_FRAMES not set")
	}
	for _, dark := range []bool{false, true} {
		a := spotApp()
		a.agents = agentsState{open: true, exe: "/Applications/DuckEase Music.app/Contents/MacOS/DuckEase Music",
			installed: map[string]bool{"codex": true, "claude-code": true, "claude-desktop": true},
			connected: map[string]bool{"claude-code": true}}
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(dark)
		for range 20 {
			tt.Frame()
		}
		name := "agents-light.png"
		if dark {
			name = "agents-dark.png"
		}
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, tt.Image())
		f.Close()
	}
}
