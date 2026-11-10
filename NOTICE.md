# Provenance and modification notice / 来源与修改声明

## Original work

DuckEase Music is a modified continuation of **MyGo 云音乐** (MyGo Cloud Music), originally developed by **egoist and the upstream contributors**. Their authorship and applicable copyright notices remain with them. Renaming this fork does not transfer ownership of their work.

- Original repository: https://github.com/egoist/mygo-cloudmusic
- Source mirror used: https://github.com/LeslieYon/mygo-cloudmusic
- Preserved source commit: `3c0eb7122527d5906a858624b14f75e2318498ed`.
- Original application version: `0.1.25`.
- The original repository's public README, checked on **2026-11-08**, says it went offline after a letter from NetEase. This fork does not make a finding about that dispute or imply the author's approval of this continuation.

## Modifications

**Ruoyu / SysNomad**, **2026-11-08**: renamed the app to DuckEase Music and replaced its icon; retained migration of old local data; removed the original updater and podcast speech transcription; added bounded stream retries, a pause when the platform reports risk control, and compatibility changes to client identification; prepared bilingual documentation and demo screenshots; removed the old website and inherited signing/release automation; changed the build configuration to ad hoc signing without notarization.

The published repository begins with a new source snapshot so that the inherited website's audio and obsolete distribution material are not uploaded through earlier Git history. The original development history is preserved locally. This snapshot does not claim the upstream code as new work by the maintainer.

## License and third-party material

The application source continues under **GNU Affero General Public License, version 3**, as supplied in [LICENSE](LICENSE). Dependencies retain their own licenses. Preserve applicable notices and comply with the license when modifying, distributing, or providing a modified version over a network.

The AGPL covers the licensed software; it does not license music, lyrics, album art, platform data, service access, or third-party marks. NetEase and NetEase Cloud Music names identify the interoperated service. MyGo identifies the original app or framework as the context requires. No endorsement is implied.

The optional fallback source uses the public API of **GD音乐台(music.gdstudio.xyz)** by GD Studio, credited here and in the app's settings as the site asks. It is off by default. The site states its API is CC BY-NC 4.0, for study only and not for commercial use; this fork does not operate, proxy, or vouch for it.

The README interface renders use invented track metadata, lyrics written for this project, and abstract covers generated locally. The separate 191 MB memory screenshot was supplied by the maintainer from their own Mac; it is a single local observation. Banner graphics and section icons were drawn for this fork; the duck mark follows this fork's existing app icon. These project-created documentation assets are offered under the same AGPL-3.0 license. This statement does not relicense any third-party content.

## 中文说明

原始应用及大部分代码由 **egoist 与原项目贡献者**完成，本项目通过 **LeslieYon 的镜像**接续维护。Ruoyu / SysNomad 的修改日期为 **2026-11-08**，范围见上文。公开版从清理后的源码快照开始，本地仍保留原开发历史，不冒称原作，也不暗示正式交接。

可选的备用音源使用 GD Studio 的 **GD音乐台(music.gdstudio.xyz)** 公开接口，按站方要求在此处和应用设置中注明出处，默认关闭。站方声明接口采用 CC BY-NC 4.0，仅供学习、禁止商用；本分支不运营、不转发、也不担保该服务。

源码延续 AGPL-3.0；第三方依赖保留各自许可。音乐、歌词、封面、平台数据和商标的权利另行归属。原作者下线的事实以其公开仓库说明为来源，本分支不对相关争议作法律定性。
