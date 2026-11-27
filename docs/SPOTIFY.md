# Spotify 歌单

连上 Spotify 以后，侧边栏多一组「Spotify 歌单」，最上面是「Spotify 喜欢的歌曲」。点开一个歌单，里面的歌会在网易云上找到对应的那首，直接在 DuckEase Music 里播放。Spotify 歌单可以新建、改名、改简介、删除，也可以加歌删歌。还能把 Spotify 歌单复制到网易云，或者关联一个已有的网易云歌单；网易云歌单也能复制到 Spotify。关联过的两个歌单之后可以再同步。

Spotify 的音频不会在这里播放：Web API 不提供音频，能放的只有网易云（和打开了的备用音源）里有的歌。

## 连接

Spotify 只给个人开发者 app 开放接口，所以要自己建一个，几分钟的事：

设置里有一步一步的指引，照着点就行：

1. 用你的 Spotify 账号登录 [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard)（设置里有「打开」按钮），点 Create app。
2. 名字和描述随便填。Redirect URI 填 `http://127.0.0.1:43821/callback`，一个字都不能差（设置里有「复制地址」）。APIs used 勾 Web API。
3. 建好后打开 app 的 Settings，复制 Client ID。
4. 在 DuckEase Music 的「设置 → Spotify」里粘贴 Client ID，点「连接」，在浏览器里点同意。位数不对时设置里会提示。

0.1.25-duckease.8 之前连接过的，要在设置里点一次「重新连接」：读写“喜欢的歌曲”需要多两项授权（`user-library-read`、`user-library-modify`），Client ID 不用重填。

不需要 Client Secret。登录走 PKCE，令牌存在钥匙串里，和网易云的登录态放在一起。

Spotify 从 2026 年 3 月起的限制：

- 开发者模式的 app，**所有者要有 Spotify Premium**，否则接口全部拒绝。
- 一个 app 最多 5 个用户。只给自己用就不用管；给别人用，要在 Dashboard 的 User Management 里加上他的 Spotify 邮箱。
- Web API 只开放**自己创建或参与协作的歌单**里的歌。收藏的别人的歌单和 Spotify 官方歌单，改从它的公开播放页（`open.spotify.com/embed/playlist/…`，网站嵌入用的那种）读，只能读到前 100 首，没有专辑信息；私密歌单读不了。这个页面不是正式接口，Spotify 改版就可能失效。想要完整的，在 Spotify 里把它复制成自己的歌单。
- 搜索每次最多返回 10 条，调用额度按开发者账号合计。

## 匹配

一首 Spotify 歌在网易云里怎么找：用「歌名 + 第一个歌手」搜网易云，取前 8 条结果，按歌名（去掉括号里的 feat.、Live、Remastered 这类后缀）、歌手和时长打分，分数够高才算找到。找不到的歌列在歌单页最底下。

网易云那边会把歌手的别名也算上（比如「周杰伦」的 Jay Chou）。歌名相同但歌手对不上的，只有时长也一样才算同一首，免得把翻唱或同名的另一首歌当成原曲。

找过的结果记在应用数据目录的 `crossplay.json` 里。没找到的歌 7 天后才会再搜一次。

打开歌单页不会自动去网易云搜。第一次打开时读一遍 Spotify，用以前找到过的结果直接显示；还没找过的歌，页面上写着「还有 N 首没有匹配」。点「更新」才会重新读 Spotify，并在网易云搜没找过的歌：每首搜一次，间隔 0.8 秒，标题下面显示进度，歌单还一首都没有时页面上有鸭子游过去。离开页面也会在后台接着找，侧边栏的歌单名旁边显示百分比。网易云嫌搜得太快（405）时会停一会儿，自动接着来；遇到 `-460` 则等冷却结束。

每个歌单页读到的内容存在应用数据目录的 `spotify-pages/<歌单 id>.json`，下次启动打开直接显示，不联网。Spotify 那边歌单有变化时，标题下面会提示点「更新」。

匹配错了，可以让 agent 用 MCP 的 `fix_match` 改成正确的那首，以后的导入和同步都按改过的来。

反方向（网易云歌找 Spotify）用 Spotify 搜索的 `track:` 和 `artist:` 条件，找不到再用普通关键词搜一次。

## 复制和同步

- Spotify 歌单页的「复制到网易云」：先问网易云歌单叫什么，默认用 Spotify 上的名字。网易云不接受某些名字（部分符号、表情、敏感词），被拒时对话框会重新弹出，改个名字就行。然后把找到的歌按原来的顺序加进去，并关联两个歌单。
- 同一个对话框里选「关联已有歌单」，可以直接映射到你已有的网易云歌单（包括「我喜欢的音乐」），马上同步一次。自己的 Spotify 歌单还可以勾「双向同步」。已关联的，「同步」菜单里可以改关联到别的歌单。
- 自己的网易云歌单，「⋯」菜单里的「复制到 Spotify」：新建一个私密的 Spotify 歌单，反过来做一遍。
- 关联以后，Spotify 歌单页的「同步」可以选「同步到网易云」或「双向同步」，网易云歌单的菜单里是「同步到 Spotify」。

同步**只加不删**：一边删掉的歌，另一边不会跟着删，这样匹配错了也不会误删东西。另外，每次同步会记下两边当时有哪些歌。上次同步之后你从某一边删掉的歌，下次同步不会再从另一边加回来。

同步会先把所有的歌都找完，再一次性按顺序加进去。中途停下（离开、风控、MCP 单次调用的上限）就一首都不加，下次接着找。双向同步时，同一首歌两边版本不同（比如 Spotify 上是重制版），不会来回各加一遍。

## 编辑

- 侧边栏 Spotify 那一组右边的「⋯」：新建 Spotify 歌单、选择显示哪些歌单、刷新。单个歌单右键也可以「在侧边栏隐藏」。隐藏的歌单 id 存在 `settings.json` 的 `spotifyHidden`。
- 歌单封面右下角的「⋯」：编辑名称和简介、复制到网易云、关联已有网易云歌单、复制 Spotify 链接、删除歌单。Spotify 没有真正删除歌单的接口，“删除”是把它从你的资料库移除，和 Spotify 客户端里删除自己的歌单一样；别人的歌单则是取消收藏。
- 任何一首网易云歌曲的右键菜单都有「添加到 Spotify 歌单」，会先在 Spotify 上找到对应的那首再加进去。Spotify 歌单页里的歌，右键可以「从这个 Spotify 歌单移除」；最底下网易云没找到的歌也可以右键移除，或者拿去网易云搜索。
- 「Spotify 喜欢的歌曲」：加歌就是点赞，移除就是取消点赞。它不能改名或删除。

## 给 agent 用

MCP 服务器里也有对应的工具：`spotify_playlists`、`spotify_playlist`、`spotify_search`、`spotify_add_songs`、`spotify_remove_songs`、`spotify_import`、`spotify_export`、`sync_links`、`sync_playlist`、`fix_match`，见 [MCP.md](MCP.md)。歌单 id 写 `liked` 就是“喜欢的歌曲”。连接仍然要在 app 的设置里做。每次调用找歌约 45 秒就停（AI 客户端一般一分钟左右就会放弃等待），没找完会告诉 agent 再调一次 `sync_playlist`。

app 和 MCP 是两个进程，共用 `crossplay.json` 和钥匙串里的 Spotify 登录：写文件时先读最新内容再合并，不会互相覆盖；Spotify 令牌两边同时刷新时，后刷新的一方会改用另一方存下的新令牌，不会把两边都登出。

## 代码

- `internal/spotify`：登录（PKCE，回调监听 `127.0.0.1:43821`）和 Web API。2026 年 2 月起歌单的歌叫 `items`（以前是 `tracks`），接口路径是 `/playlists/{id}/items`、`/me/playlists`，解析时两种字段名都认。喜欢的歌曲用 `GET /me/tracks` 读，点赞、取消点赞、移除歌单都走 `PUT`/`DELETE /me/library?uris=…`（每次最多 40 个），对还在旧接口上的 app 会退回 `/me/tracks`、`/playlists/{id}/followers`。`LikedID`（`"liked"`）在各函数里代表喜欢的歌曲。读不到的歌单走 `embed.go` 解析公开播放页的 `__NEXT_DATA__`。
- `internal/app/spotify_ui.go`：设置里的连接指引、编辑和导入对话框、歌单页的磁盘缓存。
- `internal/match`：判断两首歌是不是同一首。
- `internal/crossplay`：查找、缓存匹配结果、关联和同步歌单。
- `internal/app/spotify.go`：设置、侧边栏和歌单页。
