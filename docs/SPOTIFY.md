# Spotify 歌单

连上 Spotify 以后，侧边栏多一组「Spotify 歌单」。点开一个歌单，里面的歌会在网易云上逐首找到对应的那首，直接在 DuckEase Music 里播放。也可以把 Spotify 歌单复制到网易云、把网易云歌单复制到 Spotify，复制过的两个歌单会关联起来，之后可以再同步。

Spotify 的音频不会在这里播放：Web API 不提供音频，能放的只有网易云（和打开了的备用音源）里有的歌。

## 连接

Spotify 只给个人开发者 app 开放接口，所以要自己建一个，几分钟的事：

1. 用你的 Spotify 账号登录 [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard)，点 Create app。
2. 名字和描述随便填。Redirect URI 填 `http://127.0.0.1:43821/callback`，一个字都不能差。APIs used 勾 Web API。
3. 建好后打开 app 的 Settings，复制 Client ID。
4. 在 DuckEase Music 的「设置 → Spotify」里粘贴 Client ID，点「连接」，在浏览器里点同意。

不需要 Client Secret。登录走 PKCE，令牌存在钥匙串里，和网易云的登录态放在一起。

Spotify 从 2026 年 3 月起的限制：

- 开发者模式的 app，**所有者要有 Spotify Premium**，否则接口全部拒绝。
- 一个 app 最多 5 个用户。只给自己用就不用管；给别人用，要在 Dashboard 的 User Management 里加上他的 Spotify 邮箱。
- 只能读到**自己创建或参与协作的歌单**里的歌。关注的别人的歌单、Spotify 官方的编辑歌单，侧边栏里会显示成灰色，点开只会提示读不了。想听的话，先在 Spotify 里把它复制成自己的歌单。
- 搜索每次最多返回 10 条，调用额度按开发者账号合计。

## 匹配

一首 Spotify 歌在网易云里怎么找：用「歌名 + 第一个歌手」搜网易云，取前 8 条结果，按歌名（去掉括号里的 feat.、Live、Remastered 这类后缀）、歌手和时长打分，分数够高才算找到。找不到的歌列在歌单页最底下。

网易云那边会把歌手的别名也算上（比如「周杰伦」的 Jay Chou）。歌名相同但歌手对不上的，只有时长也一样才算同一首，免得把翻唱或同名的另一首歌当成原曲。

找过的结果记在应用数据目录的 `crossplay.json` 里，下次打开不会重新搜。没找到的歌 7 天后才会再搜一次。第一次打开一个长歌单会慢一些：每首歌搜一次，间隔 0.8 秒，页面上有一只鸭子游过去表示进度。离开页面也会在后台接着匹配，侧边栏的歌单名旁边显示百分比。以后再打开，或者歌单加了新歌，只搜没找过的那些。网易云嫌搜得太快（405）时会停一会儿，自动接着来；遇到 `-460` 则等冷却结束。

匹配错了，可以让 agent 用 MCP 的 `fix_match` 改成正确的那首，以后的导入和同步都按改过的来。

反方向（网易云歌找 Spotify）用 Spotify 搜索的 `track:` 和 `artist:` 条件，找不到再用普通关键词搜一次。

## 复制和同步

- Spotify 歌单页的「复制到网易云」：新建一个同名网易云歌单，把找到的歌按原来的顺序加进去，并关联两个歌单。
- 自己的网易云歌单，「⋯」菜单里的「复制到 Spotify」：新建一个私密的 Spotify 歌单，反过来做一遍。
- 关联以后，Spotify 歌单页的「同步」可以选「同步到网易云」或「双向同步」，网易云歌单的菜单里是「同步到 Spotify」。

同步**只加不删**：一边删掉的歌，另一边不会跟着删，这样匹配错了也不会误删东西。另外，每次同步会记下两边当时有哪些歌。上次同步之后你从某一边删掉的歌，下次同步不会再从另一边加回来。

同步会先把所有的歌都找完，再一次性按顺序加进去。中途停下（离开、风控、MCP 单次调用的上限）就一首都不加，下次接着找。双向同步时，同一首歌两边版本不同（比如 Spotify 上是重制版），不会来回各加一遍。

## 给 agent 用

MCP 服务器里也有对应的工具：`spotify_playlists`、`spotify_playlist`、`spotify_import`、`spotify_export`、`sync_links`、`sync_playlist`、`fix_match`，见 [MCP.md](MCP.md)。连接仍然要在 app 的设置里做。每次调用最多找 60 首新歌（AI 客户端一般一分钟左右就会放弃等待），没找完会告诉 agent 再调一次 `sync_playlist`。

app 和 MCP 是两个进程，共用 `crossplay.json` 和钥匙串里的 Spotify 登录：写文件时先读最新内容再合并，不会互相覆盖；Spotify 令牌两边同时刷新时，后刷新的一方会改用另一方存下的新令牌，不会把两边都登出。

## 代码

- `internal/spotify`：登录（PKCE，回调监听 `127.0.0.1:43821`）和 Web API。2026 年 2 月起歌单的歌叫 `items`（以前是 `tracks`），接口路径是 `/playlists/{id}/items`、`/me/playlists`，解析时两种字段名都认。
- `internal/match`：判断两首歌是不是同一首。
- `internal/crossplay`：查找、缓存匹配结果、关联和同步歌单。
- `internal/app/spotify.go`：设置、侧边栏和歌单页。
