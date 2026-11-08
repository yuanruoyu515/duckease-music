# Privacy notes / 隐私说明

**Code scope / 适用版本：首次公开快照，2026-11-08。** This describes the code in this repository, not third-party repackaging or the platform's own practices.

## 中文

### 请求会去哪里

应用直接请求网易云音乐接口（包括 `music.163.com`、`interfacepc.music.163.com`）及平台返回的媒体、图片地址。搜索词、播放请求和登录后的账号操作会发送给相关服务。它们会获得连接所需的网络信息，登录请求还可能带客户端与设备标识。维护者不控制这些服务的数据处理、保存期限或地区。

代码中没有维护者运营的登录服务器、音乐代理、遥测或崩溃上传服务。原作者的自动更新功能已移除。下载 Go 依赖、访问 GitHub、点击外部链接也会产生对相应服务的请求。README 的徽章由 Shields.io 提供，浏览仓库时由 GitHub 和相关图片服务处理。

### 设备上保存什么

| 数据 | 存储与用途 |
|---|---|
| 登录态 | 打包版通过系统凭据存储（macOS 为钥匙串）保存；开发版在应用数据目录保存 `session` 文件。开发版文件不能视为加密保险箱 |
| 设置、设备标识、搜索历史 | 保存在本机应用数据目录，用于恢复偏好和客户端状态 |
| 播放队列与进度 | `session.json` 保存上次播放状态；它与登录态 `session` 是不同文件 |
| 歌曲与图片缓存 | 保存在本机缓存目录，用于减少重复加载；可以在设置中清除歌曲缓存 |
| 诊断日志 | macOS 为 `~/Library/Logs/DuckEase Music/app.log`，可能包含歌曲 ID、音质、请求错误和其他诊断细节 |

macOS 的常用目录为 `~/Library/Application Support/DuckEase Music/` 和 `~/Library/Caches/DuckEase Music/`；开发版名称通常带 `Dev`。首次迁移可能移动旧应用目录，详见 README。其他平台的路径由 MyGo 决定，本分支尚未验证。

### 退出、清理和反馈

在应用内退出登录会执行清除登录态和平台登出操作，网络操作可能失败；需要撤销其他会话时，请在网易云官方账号安全功能中处理。删除应用本身不会自动删除应用数据、缓存或钥匙串项。清理本机目录前先退出应用并备份需要的数据；不要误删其他应用的同名凭据。

提交问题时，先删除 Cookie、令牌、session 内容、扫码登录地址、账号资料、私人歌单及不必要的本地路径。维护者只会看到你主动提交到 GitHub 等渠道的信息，这些渠道的数据处理遵循各自规则。

## English

The app contacts NetEase API hosts, including `music.163.com` and `interfacepc.music.163.com`, and media or image locations returned by the platform. Queries, playback requests, authenticated actions, connection metadata, and applicable client/device identifiers reach those services. The maintainer does not control their retention or processing.

This code has no maintainer-operated login server, music proxy, telemetry, or crash-upload service. The original updater has been removed. Dependency downloads, GitHub visits, and external links contact their respective services. README badges are served through Shields.io and GitHub's image handling.

Packaged builds use the system credential store for login data; development builds save a local `session` file, which should not be treated as an encrypted vault. Settings, device state, search history, and playback state remain in the application data directory. `session.json` contains playback state and is distinct from the login session file. Audio and artwork are cached locally. Diagnostic logs may contain track IDs, quality choices, request errors, and other troubleshooting details.

On macOS, typical locations are `~/Library/Application Support/DuckEase Music/`, `~/Library/Caches/DuckEase Music/`, and `~/Library/Logs/DuckEase Music/app.log`. Development app names generally include `Dev`. Older folders may be moved on first launch. Other platforms use MyGo's path rules and have not been verified for this fork.

Signing out clears local login state and attempts platform logout; network requests can fail. Use the platform's official account-security tools to revoke other sessions. Removing the app does not automatically remove its data, caches, or credential-store entries. Quit the app and back up needed data before cleaning local files.

Before submitting a report, redact credentials, cookies, session contents, QR login URLs, private account or playlist information, and unnecessary local paths. The maintainer receives information you choose to submit through GitHub or other channels; those services have their own privacy rules.
