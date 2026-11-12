# 维护要点

Ruoyu / SysNomad 的个人维护分支，公开仓库为 https://github.com/yuanruoyu515/duckease-music 。首次公开为清理后的源码快照，不上传较早的本地开发历史。许可证、来源与使用边界见 NOTICE.md、LICENSE 和 DISCLAIMER.md。

## 来源

- 原作者 egoist 的仓库 `egoist/mygo-cloudmusic` 已归档。最后一个提交 `1e15b2b 下线` 删掉了全部代码，Release 和下载页也删了。
- 本仓库来自 `LeslieYon/mygo-cloudmusic`，它停在下线前一个提交 `3c0eb71`，是本分支采用的完整源码。远端名为 `upstream`，只作参考，不向它推送。
- 原始应用基于 0.1.25；个人维护版从 `0.1.25-duckease.1` 起单独记录版本。`3c0eb71` 相比原版 0.1.25 只修改了旧网站。

## 构建

```bash
brew install go                # 需要 go.mod 里写的 Go 1.27.3
./scripts/build-local.sh       # 先跑测试，再构建
```

产物在 `build/darwin-arm64/`：`DuckEase Music.app` 和 `duckease-music-<版本>-arm64.dmg`。要同时支持 Intel，加 `-platform darwin/universal`。

- 签名用的是 ad hoc（`-sign -`），不做公证。本机构建不代表通过 Apple 的分发认证。下载的构建可能需要在「隐私与安全性」中批准打开，不要关闭系统整体安全保护。
- 图标是 Icon Composer 文档 `icon/DuckEase.icon`：红色铺满的圆角方形（`icon.json` 的 `fill`），上面是白环和鸭子（`Assets/duck-ring.png`，由圆形图 `icon/duck/duck-round.svg` 抠掉红色圆盘得到），缩放比例在 `icon.json` 的 `scale` 里调。`resources/icon.png` 是同一张圆形图，给旧版 macOS 的 `.icns` 用。系统从 macOS 26 开始只让 App 图标用圆角方形，其他形状会被套进灰框，所以不要改回纯圆形。`icon/AppIcon.icon` 和 `icon/main.go` 是原版红色唱片图标的源文件，留着备查。
- 显示名改成了 DuckEase Music（`mygo.json` 的 `name`），identifier 仍是 `dev.egoist.cloudmusic`。登录信息按 identifier 存在钥匙串里，不受改名影响；原版由作者的 Developer ID 签名，换成自编版后第一次读取登录信息，钥匙串会弹窗问一次，点「始终允许」，不行就重新扫码。
- 设置和缓存的文件夹跟着显示名走。第一次启动 DuckEase Music 时，`moveOldData` 按 `MyDuck Music`、`MyGo 云音乐`、`网易云音乐` 的顺序，把找到的第一个旧文件夹搬过来。
- 新名字的 app 不会覆盖旧名字的。几个版本的 identifier 相同，只能同时开一个：旧的开着时，再开新的只会把旧的窗口调出来。装好新版后，退出并删掉旧版。

## 和原版的差别

原作者下线后，有两样功能依赖的下载地址没了，这个分支直接删掉了它们：

- **自动更新**：原来查的是 egoist 的 Release，MyGo 的 updater 插件和 `mygo.json` 的 `updates` 都已去掉。现在 `internal/app/update.go` 只查 `yuanruoyu515/duckease-music` 的 latest release，比较版本号（`newerVersion`，先比 `0.1.25`，再比 `duckease.N`），有新版就弹窗提示下载 DMG、稍后提醒或忽略此版本，不自动安装。发版时标签必须是 `v<版本>`，并附上 `.dmg`，否则提示会打开 Release 页面。
- **播客字幕**（本机语音识别）：sherpa-onnx 动态库原本从 egoist 的 Release 下载，现在是 404。`internal/speech`、`scripts/sherpa`、正在播放的「字幕」标签和设置里的「播客」一组都已去掉。播客本身照常能听。

## 公开版发布范围

- 原作者的签名、Apple 公证配置与自动 Release 工作流已移除，`mygo.json` 默认采用 ad hoc 签名。
- 旧官网、网站歌曲演示和对应的 Bun 配置已移除；不沿用原作者的官网或下载域名。
- `.github/workflows/test.yml` 只跑源码测试，不生成或上传安装包。
- 公开仓库从一次清理后的源码快照开始，本机 `codex/pre-publication-history` 保留之前的开发历史，不设远端跟踪。本机 `main` 跟踪 `origin/main`；普通 `git pull` / `git push` 对应自己的公开仓库。不要推送历史备份分支或镜像所有本地引用，以免重新带入已移除的历史媒体。
- 分发二进制前，重新核对许可证与对应源码交付方式，并明确签名、公证及平台验证状态。`scripts/build-local.sh` 仅构建，不上传。

## 日常维护

1. `git status`，确认工作区干净。改动大就开分支。
2. 改代码，按 `AGENTS.md` 的规范补测试。
3. `./scripts/build-local.sh`，退出旧 app，打开 `build/darwin-arm64/DuckEase Music.app` 试用。
4. 没问题就提交，再把 `.app` 拖进「应用程序」替换旧的。
5. 想留版本记录，改 `mygo.json` 的 `version`，在 `CHANGELOG.md` 顶部写一节。`mygo build` 要求当前版本在 changelog 里有对应一节。

UI 框架的规则（Context、Element 生命周期、线程）见 `.agents/skills/mygo-maintenance/SKILL.md`，改界面前先读。

## 本机数据与备份

- 侧边栏的顺序和隐藏项在 `settings.json` 的 `sidebar`，正在播放卡片在 `hideSideNowPlaying`。新加的侧边栏区块记得加进 `internal/app/sidebar.go` 的 `sideSections`，旧设置里没有的项会自动排到最后并显示。
- 自定义歌单封面存在用户数据目录的 `covers/<歌单 ID>.jpg`，裁成正方形，边长最多 1000 px。删歌单时一并删掉。
- iCloud 备份写到 `~/Library/Mobile Documents/com~apple~CloudDocs/DuckEase Music/DuckEase Music 备份.zip`，内含 `manifest.json`、`settings.json` 和 `covers/`。改备份结构时把 `backupFormat` 加一，旧版遇到新格式会拒绝恢复。不要把登录态、`session.json` 或缓存放进去。
- 批量加歌、删歌单、复制歌单都在 `internal/app/bulk.go`：请求串行，间隔 `bulkPause`（400 ms），每次最多 `bulkChunk`（200）首。别改成并发，原因见上面的 `-460` 一节。

## 依赖

- 最大的外部依赖是 UI 框架 [egoist/mygo](https://github.com/egoist/mygo)。它没有下线，仍在更新。`go.mod` 固定在一个伪版本上，不升级就不会变。
- 升级：`go get github.com/egoist/mygo@latest`，`go tool mygo migrate-ui .` 预览，`-write` 应用，再跑全部测试。0.x 阶段每次小版本号变化都可能有破坏性改动。
- 防止依赖消失：Go 模块代理会缓存已发布的版本，一般拉得到。想更保险，在单独分支执行 `go mod vendor` 并提交。

## 姊妹项目 MyGo Music

[egoist/mygo-music](https://github.com/egoist/mygo-music)（AGPL-3.0）是原作者 2026-11-04 下线云音乐的同一天开的新项目：把云音乐的代码拿掉网易云部分，改成靠插件接音源的通用播放器。它不是云音乐的前身，是同一套代码的后续。播放器（`internal/player`）、歌词（`internal/lyrics`）和界面都是同源的（刘海播放器本分支已删），它每修一个 bug，我们这边多半也有。

- 两边没有共同的 git 历史（它的第一个提交是一次性导入），不能直接 merge 或 cherry-pick，只能对照着手工移植。
- 已加成远端 `mygo-music`，只拉不推：`git fetch mygo-music`，然后 `git log mygo-music/main -- internal/player internal/lyrics` 看它最近改了什么，`git diff HEAD mygo-music/main -- internal/player/<文件>` 对照具体差异。
- 值得移植的：播放、解码、拖动进度、歌词和界面上的 bug 修复，以及 MyGo 框架的升级（它升级时改了哪些地方，正好是我们升级的参照）。
- 不要搬的：插件系统、多音源、本地音乐文件夹、播客字幕（我们已删）、自动更新（我们已删）、刘海播放器（我们已删）、它的图标和名字。
- 它更新很快（头 4 天 115 个提交）。建议每次准备发版前看一眼，或者遇到播放类 bug 时先去它那边找找有没有修过。

## 网易云换接口时怎么修

所有网络请求都在 `internal/netease/`，界面代码不直接碰网易云。走两条通道：

- 普通接口：`netease.go` 里的 `get`/`post`，请求 `https://music.163.com/api/...`，覆盖歌单、榜单、每日推荐、歌手、普通歌词、搜索等。
- eapi：`eapi.go` 里的 `eapiPost`，冒充 PC 客户端发加密请求到 `https://interfacepc.music.163.com/eapi/...`。用在扫码登录、播放地址（`/api/song/enhance/player/url/v1`）、歌曲详情和逐字歌词上。最容易被风控的就是这条。

### 先定位哪里坏了

先看日志：`~/Library/Logs/DuckEase Music/app.log`（超过 1 MB 时，下次启动会挪到 `app.log.1`）。歌曲打不开时，每一次尝试都会记一行，带歌曲 ID、音质和报错，比如 `song url: code -460`（接口拒绝）、`403 Forbidden`（CDN）、`this song isn't available to stream`（接口说不能播）。播放器遇到失败会重试：一般错误再试两次，间隔 0.5 秒和 1.5 秒；「不能播」只再试一次；都失败才跳下一首。重试次数和间隔在 `internal/player/player.go` 的 `retryDelays`。

`code -460` 是网易云的风控，会在几十秒里拒绝所有歌。遇到它时播放器不重试、不跳歌，停在这首并提示「网易云暂时拒绝了请求，稍后再点播放」，之后 30 秒内（`blockCooldown`）不再请求播放地址，缓存里的歌照常能播。同一首歌正在加载时再点不会重复请求。2026-11-08 在美国直连 IP 下实测出现过 `-460`，持续 23 秒以上。

```bash
LIVE=1 CGO_ENABLED=0 go test ./internal/netease/ -run Live -count=1 -v
```

这组测试会真的请求网易云，哪个失败就是哪个接口出了问题。

2026-11-08 的基线：除 `TestPrivilegesLive` 外全部通过。它失败是因为测试里的歌曲 `2652820720` 现在不登录就播不了，属于测试数据过期，接口没坏（`TestSongURLEapiLive` 能拿到播放地址）。`TestRecentPlaysLive`、`TestVIPSongPlayableLive` 需要设置 `NM_SESSION` 指向一份登录态，否则跳过。

### 备用音源（GD音乐台）

设置里的「备用音源」默认关闭。打开后，网易云取不到地址（含 `-460` 冷却期间）时向 `https://music-api.gdstudio.xyz/api.php?types=url&source=netease&id=<ID>&br=<码率>` 再要一次，只试一次；灰色歌曲（`Track.Granted()` 为假）不再问网易云，直接问它。代码在 `internal/netease/fallback.go` 和 `internal/player/player.go` 的 `openAny`。

- 站方要求注明出处“GD音乐台(music.gdstudio.xyz)”，设置说明、README、NOTICE.md 里都写了，别删。
- 站方限 5 分钟 50 次，`fallbackLimit` 设为 40，超出就不请求、直接报无法播放。
- 返回的 `size` 实测是字节，文档写的 KB 不对。地址带时间戳会过期，只现取，不存。
- 坏了先跑 `LIVE=1 CGO_ENABLED=0 go test ./internal/netease/ -run FallbackURLLive -count=1 -v`，日志里搜 `fallback source`。

- 播放页的「备用音源」标签看 `netease.Stream.Fallback`，它随歌曲一起写进缓存的 `.json`；设置里的限额条读 `netease.FallbackUsage`。

### 按症状对照

| 症状 | 多半是 | 改哪里 |
|---|---|---|
| 某个页面空了或报 JSON 解析错误 | 返回字段改名或换了结构 | 对应文件里的结构体 json tag。把新的真实响应存成测试样本，补一个解析测试 |
| 接口 404 或返回 code 非 200 | 路径升级了版本（如 `v6` → `v7`）或改了参数 | 对应函数里的路径和参数 |
| 扫码登录失败、播放地址为空、提示版本过低 | eapi 的客户端身份被拒 | `eapi.go` 的 `eapiClient`（`appver`、`osver`）、`eapiUA`、header 里的 `versioncode` |
| 所有 eapi 请求都解不开 | 加密密钥 `eapiKey` 换了 | 从新版客户端或社区项目找新密钥 |
| 登录后很快掉线 | Cookie 名或刷新机制变了 | `auth.go` |
| 能播但没有无损或 Hi-Res | 音质参数 `level` 的取值变了 | `SongURL` 和 `Lossless` |

### 去哪找新接口

1. 抓官方的请求：网页版 `music.163.com` 用浏览器开发者工具看 Network；PC 或 Mac 客户端用 Proxyman 或 Charles 抓包。对照路径、参数和返回字段。
2. 参考还在维护的开源项目，见下一节。
3. 修完先跑 LIVE 测试，再构建，用自己的账号实际播几首歌（含 VIP 歌、无损），确认登录、播放、歌词都正常。

### 还活着的参考项目

接口失效、被风控、加密方式变了，先去这些项目看最近的提交和 issue。别人多半已经踩过坑，也常有人先发现网易云的改动。下表是 2026-11-08 查的状态，用之前先确认它还在更新。

| 项目 | 状态（2026-11-08） | 主要参考什么 |
|---|---|---|
| [NeteaseCloudMusicApiEnhanced/api-enhanced](https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced) | 活跃，11-06 还有提交 | **首选**。原 NeteaseCloudMusicApi 的接续版。`util/request.js` 是请求头、Cookie、加密方式（eapi、xeapi、neapi）的权威参考，`module/` 下每个接口一个文件，`util/config.json` 有各个域名。issue 区最早出现接口变化和风控的讨论 |
| [go-musicfox/go-musicfox](https://github.com/go-musicfox/go-musicfox) | 活跃（10-21） | 同样是 Go 写的终端客户端，接口层在 [go-musicfox/netease-music](https://github.com/go-musicfox/netease-music)（04-26 最后提交）。Go 实现可以直接对照着抄 |
| [algerkong/AlgerMusicPlayer](https://github.com/algerkong/AlgerMusicPlayer) | 活跃（10-28），1.7 万星 | 桌面客户端，播放地址出问题时看它的修复，比如 11-08 的 PR #785 修了取歌曲地址 |
| [stark81/VutronMusic](https://github.com/stark81/VutronMusic) | 活跃（11-02） | 桌面客户端，YesPlayMusic 一路的后继 |
| [qier222/YesPlayMusic](https://github.com/qier222/YesPlayMusic) | 有提交（10-10） | 老牌客户端，issue 多，历史问题好搜 |
| [chthollyphile/folia-major](https://github.com/chthollyphile/folia-major) | 活跃（11-08） | 本项目歌词动画参考过它 |
| [feeluown/FeelUOwn](https://github.com/feeluown/FeelUOwn) | 活跃（10-23） | Python 客户端，网易云插件是 feeluown/feeluown-netease |
| [jinghuashang/Hydrogen-Music](https://github.com/jinghuashang/Hydrogen-Music) | 活跃（11-08） | 碰到 `-460` 时可以看它的节流和熔断做法 |

已经不能用的：Binaryify/NeteaseCloudMusicApi 于 2024 年归档；SPlayer 已归档（2026-06）；egoist/mygo-cloudmusic 是本项目原仓库，已下线。

### 2026-11 已知线索：`-460` 和取歌曲地址

- api-enhanced 的 [issue #300](https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced/issues/300)（2026-11-06）和我们遇到的情况一样：登录有效，搜索、歌词都正常，只有取歌曲地址返回 `-460`，提示「检测到您的网络环境存在风险，请稍后再试」。换 IP、重新登录、降音质都没用。提问的人推测，是短时间里大量并发请求（1.3 秒 30 多次歌词请求）触发的账号级标记，会持续很久。目前还没有官方回复。
- api-enhanced 现在取歌曲地址（`module/song_url_v1.js`）已经不走我们用的 PC eapi，改走 xeapi（伪装成 Android 客户端，域名 `interface3.music.163.com`，带公钥和会话密钥）。xeapi 支持是 2026-05-29 加进去的（提交 `d0935f6`）。如果 eapi 取地址以后越来越常被拒，就移植 xeapi；不过 issue #300 用的就是 api-enhanced，照样被拒，所以 xeapi 也不保证管用。
- 由此得出的做法，本项目已经照着做了：被拒就冷却，不连续重试，同一首歌不重复请求，eapi 的 User-Agent 和请求头保持同一个客户端身份。以后加预加载、批量拉歌词这类功能时，要控制并发，别在一两秒里发出几十个请求。

## 要盯的外部变化（2026-11-10 整理）

### 可能让 App 直接坏掉的

按出事概率从高到低：

1. **网易云接口。** eapi 的客户端身份（`eapi.go` 里的 `appver` 3.1.17.204416、`versioncode` 140、`eapiKey`）被淘汰时，登录、取歌曲地址、逐字歌词会一起坏。取地址接口 `/api/song/enhance/player/url/v1` 的路径或字段（`level`、`freeTrialInfo`）改了，就会能登录但放不了歌。`privilege` 的 `st`、`pl` 改名，灰色歌的判断会全错。CDN 开始校验 Referer 或 User-Agent 时，地址拿得到但下载 403。`-460` 风控收紧时，30 秒冷却（`blockCooldown`）可能不够。最早的信号在 api-enhanced 的 issue 区和 `util/request.js`。
2. **GD音乐台。** 停服、换域名、改返回字段（`url`、`br`、`size`）或收紧频率限制（现在是 5 分钟 50 次）。功能默认关闭，出事也只是降级成无法播放。
3. **GitHub Release。** 更新检查读 `releases/latest`。发版时标签必须是 `v<版本>`，附 `.dmg`，不能勾预发布：预发布不会被 latest 返回，老版本就收不到提示。
4. **macOS。** 对 ad hoc 签名、未公证 App 的拦截越来越严，这是分发上最大的风险。钥匙串政策变化可能读不出登录态。Icon Composer 格式变了要重新导出图标。

### 值得跟上的

1. **MyGo Music 的播放器修复。** 最值得移植的是 `ce1b2ea`（提前下载下一首，可以在没下完的歌里拖动进度）。其次是 `0927157`（频谱只留最后几秒）、`c360a84`（封面按显示尺寸下载、大图限额解码）、`b021067`（改用 MyGo 自带的 FLAC 解码，可能去掉 `mewkiz/flac`）、`aabab00`（随机播放从随机一首开始）。它已经在 MyGo 0.8 上，有些改动要先升 MyGo 才能搬。
2. **MyGo 0.7 → 0.8。** 本项目固定在 `v0.7.1-0.20261027023711-da3657e7f86a`，上游已发 `v0.8.3`，MyGo Music 用 `v0.8.4` 预发布版。开分支升级，按「依赖」一节跑迁移和全部测试，参照 MyGo Music 升级时改过的地方。
3. **Go。** 1.27.x 的小版本基本每月一次，多是 `net/http`、`crypto/tls` 的安全修复，跟上即可：改 `go.mod` 的 `go` 行，`brew upgrade go`，跑测试。Go 1.28 预计 2027 年 2 月，等 MyGo 支持了再升。`golang.org/x/text`、`x/image` 等依赖用 `govulncheck ./...` 定期扫。

### 节奏

- 每次发版前：`LIVE=1` 接口测试（含 `FallbackURLLive`）；看 MyGo Music 最近改了 `internal/player`、`internal/lyrics` 的什么；看 api-enhanced 最新 issue。
- 每月：Go 小版本、`govulncheck`。
- 出播放问题：先看日志，再去 MyGo Music 找修复，再看 api-enhanced、AlgerMusicPlayer。
- 找时机做一次大的：升 MyGo 0.8，顺带移植提前下载下一首。

## 风险提醒

- 原作者公开说明收到网易函件后下线。本分支使用独立名称，不暗示原作者正式交接、平台授权或背书。发布时保留署名、来源和免责声明。
- 源码延续 AGPL-3.0。修改、分发或以修改版提供网络服务时，遵守具体情形适用的源码、署名及许可义务；不额外限制许可证授予的权利。
- eapi 冒充官方客户端，账号有被风控的可能，别高频批量请求。
