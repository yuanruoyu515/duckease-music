# 让 Codex、Claude Code 管理歌单（MCP）

DuckEase Music 自带一个 MCP 服务器。Codex、Claude Code 这类 agent 接上以后，可以读你的网易云歌单、搜歌、批量匹配歌名、建歌单、加歌删歌、找重复。

它就是 app 本身：`DuckEase Music mcp` 不开窗口，只在标准输入输出上说 MCP。登录用的是 app 存在钥匙串里的那份，所以要先在 app 里扫码登录。

## 接上

Codex，写进 `~/.codex/config.toml`：

```toml
[mcp_servers.duckease]
command = "/Applications/DuckEase Music.app/Contents/MacOS/DuckEase Music"
args = ["mcp"]
```

Claude Code：

```bash
claude mcp add duckease -- "/Applications/DuckEase Music.app/Contents/MacOS/DuckEase Music" mcp
```

第一次调用时，钥匙串可能会问能不能让 DuckEase Music 读取登录信息，点「始终允许」。每次换新构建都可能再问一次，因为 ad hoc 签名会变。

## 工具

| 工具 | 做什么 |
|---|---|
| `playlists` | 列出你的歌单：id、名字、歌曲数、类型（liked / mine / saved） |
| `playlist` | 读一个歌单的歌，可以用 `query` 只要标题、歌手、专辑里带某个词的 |
| `search` | 搜歌 |
| `match` | 一次给最多 200 行「歌名 - 歌手」，每行返回最像的网易云歌曲 id 和相似度 |
| `duplicates` | 找歌单里重复的歌（同一个 id，或者歌名和歌手一样） |
| `create_playlist` | 建歌单，可以顺便把歌加进去，顺序照给的来 |
| `add_songs` / `remove_songs` | 往自己的歌单里加歌、删歌，已经有的歌自动跳过 |
| `rename_playlist` / `delete_playlist` | 改名、删除自己的歌单。「我喜欢的音乐」不能改也不能删 |
| `like_songs` | 喜欢或取消喜欢 |

## 为什么省 token

- 回复是带表头的制表符分隔文本，一首歌一行：`id  歌名  歌手  专辑  时长`，不用 JSON，字段名不重复出现，比同样内容的 JSON 短得多。
- 歌单长的时候，用 `playlist` 的 `query`、`offset`、`limit` 只拿需要的部分。
- 导入一份歌单，`match` 一次调用就够，用不着每首歌搜一次。
- 读过的歌单和歌单列表缓存一分钟，同一轮对话里反复看不会重复请求网易云。改过的歌单会马上作废缓存。

## 风控

写操作走的是 app 批量编辑那套节奏：每次最多 200 首，两次请求之间隔 400 毫秒；`match` 每搜一首隔 200 毫秒，遇到 `-460` 立刻停下，告诉 agent 剩下的稍后再做。请求一次只处理一个，不会并发。别让 agent 同时开好几个写操作。

只有你自己建的歌单能改。收藏的别人的歌单只能读，改它会直接报错，不会发请求。

## 代码

- `main.go`：参数是 `mcp` 时走 `internal/mcp.Serve`，不启动界面。
- `internal/mcp/server.go`：JSON-RPC 和 MCP 握手，支持协议 2024-11-05、2025-03-26、2025-06-18。
- `internal/mcp/tools.go`：各个工具。
- `internal/match`：按歌名、歌手、时长判断两首歌是不是同一首。Spotify 导入也用它。
