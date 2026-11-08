# Disclaimer / 免责声明

**Updated / 更新：2026-11-08**

中英文两版内容不一致时，以中文版为准。The Chinese version prevails if the two versions differ.

## 中文

### 项目身份与维护范围

DuckEase Music 是 Ruoyu / SysNomad 在 MyGo Music 下线后，基于公开保留源码维护的独立衍生项目。原应用由 egoist 及原项目贡献者开发。这里的「个人接续维护」不代表正式移交，也不代表原作者同意本次发布。

本项目不是网易或网易云音乐的官方产品，没有获得其认可、赞助或合作背书，也不代表原作者、MyGo 框架作者或相关贡献者发言。项目名称、旧 bundle identifier 和兼容代码均不能作为上述关系的证明。应用沿用原 bundle identifier `dev.egoist.cloudmusic`，只是为了让旧版用户的本地数据能够迁移，不表示与原作者有关。

### 原项目下线情况与停止分发的可能

据原仓库公开说明，原应用在收到网易的函件后下线（详见 [NOTICE.md](NOTICE.md)）。本项目不对该争议作法律定性，但继续维护不代表相关风险已经消失。本项目可能因平台或权利人的要求随时停止分发，已下载的版本也可能随时无法使用。是否使用，请自行评估。

### 音乐、账号与服务权限

软件开源许可不包含音乐、录音、歌词、封面、艺人资料、平台数据或第三方商标的授权。相关权利归各权利人所有。界面中出现内容，不表示维护者拥有、提供或授权该内容。

使用时请遵守适用法律、平台协议、账号规则和已有订阅权限。会员、地区、设备或版权限制仍可能影响播放；本项目不保证可取得任何特定歌曲、音质或服务。本项目按平台返回的账号权限判断能否播放，不包含破解 DRM、解锁会员或付费内容、绕过地区限制或批量下载导出的功能。请勿用它实施侵权传播、盗用账号、批量抓取、绕过付费权限或其他未经授权的行为。

上面这些「请勿」是在提醒现行法律和平台规则下本来就有的义务，不是在 AGPL 之外附加的许可条件。

### 本地缓存

为减少重复加载，应用会把播放过的歌曲和图片缓存在本机。这些缓存只供本应用在本机播放，不是下载功能，也不代表你取得了相应内容的任何权利。请勿从缓存目录导出、复制、转换或传播其中的音频和图片。缓存可以在设置中清除，超过上限时会自动删除较早的内容。

本公开快照移除了旧官网的歌曲演示。README 使用虚构曲目、本项目原创演示歌词和本地生成的封面；`internal/player/testdata/` 里的两段 5 秒音频是为解码测试合成的测试音，不取自任何音乐作品，也不是向用户提供的曲库。

### 非官方接口与账号风险

本应用使用非官方的服务交互方式，向平台发送的请求会标识为桌面客户端，以便兼容平台的登录和播放接口。平台可以改变接口、拒绝请求、要求验证、限制访问或停止服务。登录态有效也不保证能播放；`-460` 等风险控制可能持续一段时间，应用遇到时会停下等待，而不是加大重试。

维护者无法保证账号不会被限制、登出或触发验证，也不能代平台恢复账号、会员或数据。请评估使用第三方客户端的风险，不要把账号密码、Cookie、session 或完整扫码链接交给维护者、其他用户或公开 issue。

### 隐私、本地数据与构建

数据存储和请求范围见 [PRIVACY.md](PRIVACY.md)。首次迁移、清理缓存、切换构建或系统凭据授权可能影响本机数据，操作前应保留需要的备份。开发版登录态文件尤其需要保护。

自行构建的 macOS 应用默认采用 ad hoc 签名，不经过 Apple 公证，不使用原作者的 Developer ID。维护者不为来源不明的二次打包、镜像下载或修改版背书。构建成功、测试通过或附有截图，不等于完成安全审计、平台授权或法律审查。

### 维护与责任

这是个人业余维护，没有可用性、更新频率、兼容性或支持时限承诺，可能暂停维护或停止分发。请勿把它当作必须持续可用的服务。

在适用法律允许的范围内，软件按「现状」提供，不附带明示或默示保证；保证与责任限制以 [AGPL-3.0 第 15–17 节](LICENSE)及适用法律为准。本声明不排除依法不得排除的责任，不修改 AGPL 授予的权利，也不把维护者自身依法应承担的义务转嫁给用户。

### 权利反馈

若你认为仓库中的特定内容涉及权利问题，可通过[权利反馈表单](https://github.com/yuanruoyu515/duckease-music/issues/new?template=rights.yml)提供具体 URL、内容位置、权利依据及希望采取的措施。公开 issue 不适合上传个人身份证明、账号凭据或私人函件；需正式处理时可使用 GitHub 的[版权投诉流程](https://docs.github.com/en/site-policy/content-removal-policies/guide-to-submitting-a-dmca-takedown-notice)。

不想公开具体内容的，可以在表单各必填栏只写「请求私下联系」和你的身份类别（如权利人、代理人），不必写细节，维护者会回复非公开的联系方式。

维护者一般会在收到后 7 天内回复。权利依据看起来成立时，会先下架或替换涉及的内容，再进一步核实。如果你认为某项内容被误下架，可以用同一个表单说明理由，维护者会重新评估。

**免责声明不是授权书、法律意见或免责护身符。「个人维护」「免费开源」「学习研究」等描述本身不会消除侵权、合同或其他法律责任，也不要求用户放弃法定权利。**

### 声明修订

本声明可能随项目变化修订，以仓库中最新版本和顶部日期为准，修订记录见本文件的 Git 历史。

## English

### Identity and scope

DuckEase Music is an independent derivative maintained by Ruoyu / SysNomad after MyGo Music went offline. The original app was developed by egoist and its contributors. Personal continuation does not mean an official handover or permission from the original author for this publication.

This is not an official NetEase or NetEase Cloud Music product. There is no claimed affiliation, approval, sponsorship, or endorsement from the platform, the original author, the MyGo framework author, or their contributors. Retained identifiers and compatibility code do not establish such a relationship. The app keeps the original bundle identifier `dev.egoist.cloudmusic` only so that existing users' local data can migrate; it does not indicate a connection with the original author.

### Takedown history and possible end of distribution

According to the original repository's public README, the original app went offline after a letter from NetEase (see [NOTICE.md](NOTICE.md)). This project makes no legal finding about that dispute, and continued maintenance does not mean the underlying risk has gone away. Distribution may stop at any time at the request of the platform or a rights holder, and downloaded versions may stop working. Decide for yourself whether to use it.

### Content and service rights

The software license does not grant rights to music, recordings, lyrics, artwork, artist information, platform data, or trademarks. Their respective rights holders retain those rights. Displayed content is not a representation that the maintainer owns, supplies, or licenses it.

Use must comply with applicable law, platform terms, account rules, and existing subscription rights. Membership, region, device, and licensing restrictions can still affect playback. No particular track, audio quality, or service is guaranteed. Playback follows the account privileges the platform returns. The project has no features for breaking DRM, unlocking membership or paid content, bypassing region locks, or bulk downloading and export. Do not use the project for unauthorized distribution, account misuse, bulk scraping, payment bypasses, or other unauthorized access.

These "do not" statements are reminders of obligations that already exist under law and platform rules. They are not license conditions added to the AGPL.

### Local cache

To avoid reloading, the app caches played songs and artwork on your computer. The cache exists only for playback by this app on this machine. It is not a download feature and gives you no rights to the content. Do not export, copy, convert, or distribute audio or images from the cache folder. You can clear the song cache in Settings, and older items are removed automatically once the cache exceeds its limit.

The old website's music demo is removed from this publication. README renders use invented tracks, original demo lyrics, and locally generated sleeves. The two 5-second clips in `internal/player/testdata/` are synthesized test tones for decoder tests. They are not taken from any musical work and are not a music catalog offered to users.

### Unofficial integration and accounts

The app interacts with services through unofficial interfaces, and its requests identify themselves as the desktop client for compatibility with the platform's login and playback endpoints. The platform may change them, reject requests, require verification, restrict access, or stop service. A valid login does not guarantee playback. Risk controls such as `-460` can persist; when the app receives one, it pauses instead of retrying harder.

The maintainer cannot guarantee that an account will remain unrestricted or signed in, and cannot restore platform accounts, subscriptions, or data on the platform's behalf. Assess the risks of an unofficial client. Do not share passwords, cookies, sessions, or complete QR login URLs in public reports or with the maintainer.

### Privacy, builds, and local data

See [PRIVACY.md](PRIVACY.md) for storage and network behavior. Migration, cache removal, changing builds, and credential-store permissions can affect local data. Back up data you need, and protect development session files.

Local macOS builds default to ad hoc signing without Apple notarization or the original author's Developer ID. Unofficial repackaging, mirrors, and modified binaries are not endorsed. A passing build, test run, or screenshot is not a security audit, platform authorization, or legal review.

### Maintenance and liability

This is spare-time personal maintenance, with no availability, update, compatibility, or support-time commitment. Maintenance or distribution may stop.

To the extent permitted by applicable law, the software is provided as is, without express or implied warranties. Warranty and liability terms are governed by [AGPL-3.0 sections 15–17](LICENSE) and applicable law. This notice does not exclude liability that cannot lawfully be excluded, alter AGPL rights, or transfer the maintainer's own legal obligations to users.

### Rights reports

Use the [rights report form](https://github.com/yuanruoyu515/duckease-music/issues/new?template=rights.yml) to identify specific material, its URL, the basis of your claim, and the requested action. Do not upload private identity documents, credentials, or correspondence publicly. GitHub also offers a formal [copyright process](https://docs.github.com/en/site-policy/content-removal-policies/guide-to-submitting-a-dmca-takedown-notice).

If you prefer not to describe the matter publicly, fill the required fields with only "requesting private contact" and your role (for example, rights holder or agent). The maintainer will reply with a non-public contact method.

The maintainer aims to respond within 7 days. Where a claim appears well founded, the material will be removed or replaced first and reviewed further afterwards. If you believe material was removed by mistake, explain why through the same form and it will be reassessed.

**This notice is not a license grant, legal advice, or immunity from liability. Personal use, free distribution, open source, and research labels do not by themselves remove copyright, contractual, or other legal obligations. No waiver of statutory rights is requested.**

### Revisions

This notice may change as the project changes. The latest version in the repository and the date at the top apply; earlier versions are in this file's Git history.
