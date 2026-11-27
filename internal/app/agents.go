package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Connecting the listener's AI agents to the app's MCP server (`DuckEase
// Music mcp`, internal/mcp): each agent keeps its MCP servers in a file of
// its own, which the app adds a server named duckease to, keeping all
// else in it, after keeping a copy of it beside.

// mcpName is the server's name in the agents' settings.
const mcpName = "duckease"

// homeDir and findClaude are where the agents' files are, and the Claude
// Code command; tests replace them.
var (
	homeDir    = os.UserHomeDir
	findClaude = func() string {
		home, _ := homeDir()
		for _, p := range []string{
			filepath.Join(home, ".local/bin/claude"),
			filepath.Join(home, ".claude/local/claude"),
			"/opt/homebrew/bin/claude",
			"/usr/local/bin/claude",
		} {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
		if p, err := exec.LookPath("claude"); err == nil {
			return p
		}
		return ""
	}
	runClaude = func(bin string, args ...string) error {
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
)

// mcpCommand is the app's executable, which serves MCP with the argument
// mcp: where it is now, links followed.
func mcpCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return "/Applications/DuckEase Music.app/Contents/MacOS/DuckEase Music"
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe
}

// agentKind is an agent the app can connect.
type agentKind struct {
	id, name, note string
	// file is the agent's settings file, under the home folder; dir, the
	// folder whose being there says the agent is installed.
	file, dir string
	// state reads whether the agent is installed and has the server.
	state func(k agentKind) (installed, connected bool)
	// connect adds the server, run with the app's executable; remove
	// takes it out.
	connect func(k agentKind, exe string) error
	remove  func(k agentKind) error
	// snippet is what to add by hand.
	snippet func(exe string) string
}

var agentKinds = []agentKind{
	{
		id: "codex", name: "Codex", note: "写入 ~/.codex/config.toml；重开 Codex 生效",
		file: ".codex/config.toml", dir: ".codex",
		state: func(k agentKind) (bool, bool) {
			b, _ := os.ReadFile(k.path())
			return k.installed(), tomlHasServer(string(b))
		},
		connect: func(k agentKind, exe string) error {
			return editFile(k.path(), func(old string) (string, error) {
				return tomlWithServer(old, exe), nil
			})
		},
		remove: func(k agentKind) error {
			return editFile(k.path(), func(old string) (string, error) { return tomlWithoutServer(old), nil })
		},
		snippet: tomlBlock,
	},
	{
		id: "claude-code", name: "Claude Code", note: "用 claude mcp add 加到用户范围，所有项目都能用；新开的会话生效",
		file: ".claude.json", dir: ".claude",
		state: func(k agentKind) (bool, bool) {
			b, _ := os.ReadFile(k.path())
			return k.installed() || findClaude() != "", jsonHasServer(b)
		},
		connect: func(k agentKind, exe string) error {
			bin := findClaude()
			if bin == "" {
				return errors.New("找不到 claude 命令：复制配置，在终端里运行")
			}
			runClaude(bin, "mcp", "remove", "-s", "user", mcpName) // an older one, if any
			return runClaude(bin, "mcp", "add", "-s", "user", mcpName, "--", exe, "mcp")
		},
		remove: func(k agentKind) error {
			bin := findClaude()
			if bin == "" {
				return errors.New("找不到 claude 命令")
			}
			return runClaude(bin, "mcp", "remove", "-s", "user", mcpName)
		},
		snippet: func(exe string) string {
			return fmt.Sprintf("claude mcp add -s user %s -- %q mcp", mcpName, exe)
		},
	},
	jsonAgent("claude-desktop", "Claude Desktop", "写入 claude_desktop_config.json；退出并重开 Claude Desktop 生效",
		"Library/Application Support/Claude/claude_desktop_config.json", "Library/Application Support/Claude"),
	jsonAgent("cursor", "Cursor", "写入 ~/.cursor/mcp.json；在 Cursor 设置的 MCP 里打开它",
		".cursor/mcp.json", ".cursor"),
}

// jsonAgent is an agent keeping its servers under mcpServers of a JSON
// file, as Claude Desktop and Cursor do.
func jsonAgent(id, name, note, file, dir string) agentKind {
	return agentKind{
		id: id, name: name, note: note, file: file, dir: dir,
		state: func(k agentKind) (bool, bool) {
			b, _ := os.ReadFile(k.path())
			return k.installed(), jsonHasServer(b)
		},
		connect: func(k agentKind, exe string) error {
			return editFile(k.path(), func(old string) (string, error) { return jsonWithServer(old, exe) })
		},
		remove: func(k agentKind) error {
			return editFile(k.path(), func(old string) (string, error) { return jsonWithoutServer(old) })
		},
		snippet: jsonBlock,
	}
}

func (k agentKind) path() string {
	home, _ := homeDir()
	return filepath.Join(home, k.file)
}

func (k agentKind) installed() bool {
	home, _ := homeDir()
	fi, err := os.Stat(filepath.Join(home, k.dir))
	return err == nil && fi.IsDir()
}

// editFile changes the file at path with change, keeping what it had in
// path.duckease-backup first, and writing through a temporary file, so
// that the agent never reads half of it.
func editFile(path string, change func(old string) (string, error)) error {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next, err := change(string(old))
	if err != nil {
		return err
	}
	if next == string(old) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
		if err := os.WriteFile(path+".duckease-backup", old, mode); err != nil {
			return err
		}
	}
	tmp := path + ".duckease-tmp"
	if err := os.WriteFile(tmp, []byte(next), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// TOML, as Codex keeps its settings.

const tomlHeader = "[mcp_servers." + mcpName + "]"

func tomlQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func tomlBlock(exe string) string {
	return tomlHeader + "\ncommand = " + tomlQuote(exe) + "\nargs = [\"mcp\"]\n"
}

func tomlHasServer(s string) bool {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) == tomlHeader {
			return true
		}
	}
	return false
}

// tomlWithoutServer takes the server's table out of s: its header, and
// the lines up to the next table's.
func tomlWithoutServer(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	in := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case t == tomlHeader || strings.HasPrefix(t, "[mcp_servers."+mcpName+"."):
			in = true
			continue
		case strings.HasPrefix(t, "["):
			in = false
		}
		if !in {
			out = append(out, l)
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func tomlWithServer(s, exe string) string {
	rest := strings.TrimRight(tomlWithoutServer(s), "\n")
	if rest == "" {
		return tomlBlock(exe)
	}
	return rest + "\n\n" + tomlBlock(exe)
}

// JSON, as Claude Desktop, Cursor and Claude Code keep theirs, under
// mcpServers.

func jsonBlock(exe string) string {
	b, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{
		mcpName: map[string]any{"command": exe, "args": []string{"mcp"}},
	}}, "", "  ")
	return string(b)
}

func jsonHasServer(b []byte) bool {
	var v struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	json.Unmarshal(b, &v)
	_, ok := v.MCPServers[mcpName]
	return ok
}

// jsonEdit decodes s, an object or nothing, lets fn change it, and
// encodes it again; what it doesn't know of stays as it was.
func jsonEdit(s string, fn func(servers map[string]json.RawMessage)) (string, error) {
	top := map[string]json.RawMessage{}
	if strings.TrimSpace(s) != "" {
		if err := json.Unmarshal([]byte(s), &top); err != nil {
			return "", fmt.Errorf("设置文件不是有效的 JSON，没有改动它：%w", err)
		}
	}
	servers := map[string]json.RawMessage{}
	if raw, ok := top["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return "", fmt.Errorf("mcpServers 的格式不认识，没有改动它：%w", err)
		}
	}
	fn(servers)
	b, _ := json.Marshal(servers)
	top["mcpServers"] = b
	out, err := json.MarshalIndent(top, "", "  ")
	return string(out) + "\n", err
}

func jsonWithServer(s, exe string) (string, error) {
	return jsonEdit(s, func(servers map[string]json.RawMessage) {
		servers[mcpName], _ = json.Marshal(map[string]any{"command": exe, "args": []string{"mcp"}})
	})
}

func jsonWithoutServer(s string) (string, error) {
	if !jsonHasServer([]byte(s)) {
		return s, nil
	}
	return jsonEdit(s, func(servers map[string]json.RawMessage) { delete(servers, mcpName) })
}

// agentPrompt is what the listener pastes to any agent, which then
// connects itself.
func agentPrompt(exe string) string {
	return "请帮我把 DuckEase Music 的 MCP 服务器接到你（现在这个 AI 工具）里，用来管理我的网易云音乐和 Spotify 歌单。\n\n" +
		"- 类型：stdio\n" +
		"- 服务器名：" + mcpName + "\n" +
		"- 命令：" + exe + "\n" +
		"- 参数：mcp\n" +
		"- 不需要 API key 和环境变量，登录状态由 DuckEase Music app 提供\n\n" +
		"按你自己的 MCP 配置方式加上它，不要改动其他已有的 MCP 服务器。参考写法：\n\n" +
		"TOML（如 Codex 的 ~/.codex/config.toml）：\n" + tomlBlock(exe) + "\n" +
		"JSON（如 Claude Desktop、Cursor）：\n" + jsonBlock(exe) + "\n\n" +
		"接好后调用它的 playlists 工具试一下，能列出我的网易云歌单就说明成功了。"
}

// agentsState is the dialog that connects agents, as it shows.
type agentsState struct {
	open bool
	// What each agent's settings say, by id, read off the main thread.
	installed, connected map[string]bool
	busy                 string // the agent being connected or removed
	exe                  string
}

// openAgents opens the dialog, reading what each agent has.
func (a *app) openAgents() {
	a.agents.open = true
	a.agents.exe = mcpCommand()
	a.readAgents()
}

func (a *app) readAgents() {
	go func() {
		inst, conn := map[string]bool{}, map[string]bool{}
		for _, k := range agentKinds {
			inst[k.id], conn[k.id] = k.state(k)
		}
		a.post(func() { a.agents.installed, a.agents.connected = inst, conn })
	}()
}

// agentDo connects agent k, or takes the server out of it, off the main
// thread: it writes files and may run a command.
func (a *app) agentDo(k agentKind, connect bool) {
	if a.agents.busy != "" {
		return
	}
	a.agents.busy = k.id
	exe := a.agents.exe
	go func() {
		var err error
		if connect {
			err = k.connect(k, exe)
		} else {
			err = k.remove(k)
		}
		a.post(func() {
			a.agents.busy = ""
			switch {
			case err != nil:
				a.toast = k.name + "：" + err.Error()
			case connect:
				a.toast = "已接入 " + k.name + "，重开它就能用了"
			default:
				a.toast = "已从 " + k.name + " 移除"
			}
			a.readAgents()
		})
	}()
}

// agentsView is the dialog: each agent, whether it is connected, and a
// button to connect it or copy what to add by hand; and what to paste to
// any other agent.
func (a *app) agentsView(c *ui.Context) {
	p := a.pal
	d := &a.agents
	ui.DialogBase(c.Key("agents"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(480).Padding(22, 20, 20, 20).Radius(18).Gap(12).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Column(c).Gap(4).Padding(0, 4).Children(func() {
			ui.Text(c, "连接 AI Agent").FontSize(17).FontWeight(700)
			ui.Text(c, "接入后，可以直接让 AI 帮你整理歌单：查重、批量加歌、按名单建歌单、同步 Spotify。用的是这台电脑上 DuckEase Music 的登录状态，不经过任何服务器。").
				FontSize(12.5).TextColor(p.muted).LineHeight(1.45)
		})
		if !strings.HasPrefix(d.exe, "/Applications/") {
			ui.Text(c, "提示：DuckEase Music 现在不在「应用程序」文件夹里。挪动 app 后要重新接入。").
				FontSize(12).TextColor(ui.Hex("#d97706")).Padding(0, 4).LineHeight(1.4)
		}
		ui.Column(c).Radius(12).Background(p.hover.Over(p.bg)).Dividers(1, p.hairline).Children(func() {
			for _, k := range agentKinds {
				a.agentRow(c, k)
			}
		})
		ui.Row(c).Padding(4, 4, 0, 4).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "其他 Agent").FontSize(13.5).FontWeight(500)
				ui.Text(c, "复制一段话，粘贴给任何支持 MCP 的 AI，让它自己接上").FontSize(12).TextColor(p.muted)
			})
			if pillButton(c.Key("prompt"), iconCopy, "复制给 Agent 的话", p.accent, p.onAccent).Clicked() {
				copyText(agentPrompt(d.exe))
				a.toast = "已复制，粘贴给你的 AI 就行"
			}
		})
		ui.Row(c).Justify(ui.End).Margin(4, 0, 0, 0).Children(func() {
			if pillButton(c.Key("done"), nil, "完成", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
		})
	})
}

func (a *app) agentRow(c *ui.Context, k agentKind) {
	p := a.pal
	d := &a.agents
	installed, connected := d.installed[k.id], d.connected[k.id]
	ui.Row(c.Key(k.id)).Padding(12, 14).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, k.name).FontSize(13.5).FontWeight(600)
				switch {
				case d.installed == nil:
				case connected:
					ui.Text(c, "已接入").FontSize(11).FontWeight(600).TextColor(ui.Hex("#16a34a")).
						Padding(1, 6).Radius(5).Background(ui.Hex("#16a34a").Alpha(0.12))
				case !installed:
					ui.Text(c, "未检测到").FontSize(11).TextColor(p.faint)
				}
			})
			ui.Text(c, k.note).FontSize(11.5).TextColor(p.muted).LineHeight(1.4)
		})
		if iconButton(c.Key("copy"), iconCopy, 14, p.muted, "复制 "+k.name+" 的配置").Size(30, 30).Radius(8).Clicked() {
			copyText(k.snippet(d.exe))
			a.toast = "已复制 " + k.name + " 的配置"
		}
		label, connect := "一键接入", true
		switch {
		case d.busy == k.id:
			label = "请稍候…"
		case connected:
			label, connect = "移除", false
		}
		if pillButton(c.Key("do"), nil, label, p.bg, p.text).Label(label + " " + k.name).Disabled(d.busy != "" || d.installed == nil).Clicked() {
			a.agentDo(k, connect)
		}
	})
}
