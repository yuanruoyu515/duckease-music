# Disclaimer / 免责声明

**Updated / 更新：2026-11-08**

## 中文

### 项目身份与维护范围

DuckEase Music 是 Ruoyu / SysNomad 在 MyGo Music 下线后，基于公开保留源码维护的独立衍生项目。原应用由 egoist 及原项目贡献者开发。这里的「个人接续维护」不代表正式移交，也不代表原作者同意本次发布。

本项目不是网易或网易云音乐的官方产品，没有获得其认可、赞助或合作背书，也不代表原作者、MyGo 框架作者或相关贡献者发言。项目名称、旧 bundle identifier 和兼容代码均不能作为上述关系的证明。

### 音乐、账号与服务权限

软件开源许可不包含音乐、录音、歌词、封面、艺人资料、平台数据或第三方商标的授权。相关权利归各权利人所有。界面中出现内容，不表示维护者拥有、提供或授权该内容。

使用时请遵守适用法律、平台协议、账号规则和已有订阅权限。会员、地区、设备或版权限制仍可能影响播放；本项目不保证可取得任何特定歌曲、音质或服务。请勿用它实施侵权传播、盗用账号、批量抓取、绕过付费权限或其他未经授权的行为。

本公开快照移除了旧官网的歌曲演示。README 使用虚构曲目、本项目原创演示歌词和本地生成的封面；音频解码测试仍保留必要的小型测试样本，它们不是向用户提供的音乐曲库。

### 非官方接口与账号风险

本应用使用非官方的服务交互方式，平台可以改变接口、拒绝请求、要求验证、限制访问或停止服务。登录态有效也不保证能播放；`-460` 等风险控制可能持续一段时间。

维护者无法保证账号不会被限制、登出或触发验证，也不能代平台恢复账号、会员或数据。请评估使用第三方客户端的风险，不要把账号密码、Cookie、session 或完整扫码链接交给维护者、其他用户或公开 issue。

### 隐私、本地数据与构建

数据存储和请求范围见 [PRIVACY.md](PRIVACY.md)。首次迁移、清理缓存、切换构建或系统凭据授权可能影响本机数据，操作前应保留需要的备份。开发版登录态文件尤其需要保护。

自行构建的 macOS 应用默认采用 ad hoc 签名，不经过 Apple 公证，不使用原作者的 Developer ID。维护者不为来源不明的二次打包、镜像下载或修改版背书。构建成功、测试通过或附有截图，不等于完成安全审计、平台授权或法律审查。

### 维护与责任

这是个人业余维护，没有可用性、更新频率、兼容性或支持时限承诺，可能暂停维护或停止分发。请勿把它当作必须持续可用的服务。

在适用法律允许的范围内，软件按「现状」提供，不附带明示或默示保证；保证与责任限制以 [AGPL-3.0 第 15–17 节](LICENSE)及适用法律为准。本声明不排除依法不得排除的责任，不修改 AGPL 授予的权利，也不把维护者自身依法应承担的义务转嫁给用户。

### 权利反馈

若你认为仓库中的特定内容涉及权利问题，可通过[权利反馈表单](https://github.com/yuanruoyu515/duckease-music/issues/new?template=rights.yml)提供具体 URL、内容位置、权利依据及希望采取的措施。公开 issue 不适合上传个人身份证明、账号凭据或私人函件；需正式处理时可使用 GitHub 的[版权投诉流程](https://docs.github.com/en/site-policy/content-removal-policies/guide-to-submitting-a-dmca-takedown-notice)。维护者会根据收到的信息评估修正、替换、移除或暂停分发。

**免责声明不是授权书、法律意见或免责护身符。「个人维护」「免费开源」「学习研究」等描述本身不会消除侵权、合同或其他法律责任，也不要求用户放弃法定权利。**

## English

### Identity and scope

DuckEase Music is an independent derivative maintained by Ruoyu / SysNomad after MyGo Music went offline. The original app was developed by egoist and its contributors. Personal continuation does not mean an official handover or permission from the original author for this publication.

This is not an official NetEase or NetEase Cloud Music product. There is no claimed affiliation, approval, sponsorship, or endorsement from the platform, the original author, the MyGo framework author, or their contributors. Retained identifiers and compatibility code do not establish such a relationship.

### Content and service rights

The software license does not grant rights to music, recordings, lyrics, artwork, artist information, platform data, or trademarks. Their respective rights holders retain those rights. Displayed content is not a representation that the maintainer owns, supplies, or licenses it.

Use must comply with applicable law, platform terms, account rules, and existing subscription rights. Membership, region, device, and licensing restrictions can still affect playback. No particular track, audio quality, or service is guaranteed. Do not use the project for unauthorized distribution, account misuse, bulk scraping, payment bypasses, or other unauthorized access.

The old website's music demo is removed from this publication. README renders use invented tracks, original demo lyrics, and locally generated sleeves. Small audio fixtures remain for decoder tests; they are not a music catalog offered to users.

### Unofficial integration and accounts

The app interacts with services through unofficial interfaces. The platform may change them, reject requests, require verification, restrict access, or stop service. A valid login does not guarantee playback. Risk controls such as `-460` can persist.

The maintainer cannot guarantee that an account will remain unrestricted or signed in, and cannot restore platform accounts, subscriptions, or data on the platform's behalf. Assess the risks of an unofficial client. Do not share passwords, cookies, sessions, or complete QR login URLs in public reports or with the maintainer.

### Privacy, builds, and local data

See [PRIVACY.md](PRIVACY.md) for storage and network behavior. Migration, cache removal, changing builds, and credential-store permissions can affect local data. Back up data you need, and protect development session files.

Local macOS builds default to ad hoc signing without Apple notarization or the original author's Developer ID. Unofficial repackaging, mirrors, and modified binaries are not endorsed. A passing build, test run, or screenshot is not a security audit, platform authorization, or legal review.

### Maintenance and liability

This is spare-time personal maintenance, with no availability, update, compatibility, or support-time commitment. Maintenance or distribution may stop.

To the extent permitted by applicable law, the software is provided as is, without express or implied warranties. Warranty and liability terms are governed by [AGPL-3.0 sections 15–17](LICENSE) and applicable law. This notice does not exclude liability that cannot lawfully be excluded, alter AGPL rights, or transfer the maintainer's own legal obligations to users.

### Rights reports

Use the [rights report form](https://github.com/yuanruoyu515/duckease-music/issues/new?template=rights.yml) to identify specific material, its URL, the basis of your claim, and the requested action. Do not upload private identity documents, credentials, or correspondence publicly. GitHub also offers a formal [copyright process](https://docs.github.com/en/site-policy/content-removal-policies/guide-to-submitting-a-dmca-takedown-notice). Reports may lead to correction, replacement, removal, or suspension of distribution after evaluation.

**This notice is not a license grant, legal advice, or immunity from liability. Personal use, free distribution, open source, and research labels do not by themselves remove copyright, contractual, or other legal obligations. No waiver of statutory rights is requested.**
