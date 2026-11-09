# Reproducing the README images / 复现 README 图片

The screenshots are rendered by the application's actual native Go views through MyGo's `ui.NewTester`. They are not mockups of an unimplemented interface. The render uses no real account or live music requests; it does not verify playback access.

截图来自应用真实界面的离屏渲染。曲目、歌单、歌词都是演示数据，封面由本地代码生成，不使用私人账号、平台封面或真实歌曲录音。

```bash
NM_PRESS_SHOTS="$PWD/assets/screenshots" CGO_ENABLED=0   go test ./internal/app -run '^TestPressScreenshots$' -count=1
```

`internal/app/press_test.go` renders home, Now Playing, the lyric stage, and the queue in light and dark appearances at 1200 × 800. Home greetings and date labels use the tester's current clock; screenshot pixels are deliberately not asserted.

The original demo lyrics are written for this repository and are not transcribed from a recording. Abstract artwork is drawn by `pressCover`. No music is played or downloaded. Inspect new images before committing them.

## Banner and icons

```bash
swift scripts/make-banner.swift
```

The generator writes English and Simplified Chinese banners, each in light and dark, on a transparent canvas. It draws with native AppKit and uses Century Gothic for Latin text, registered from the local Microsoft Excel font folder when available, and PingFang SC for Chinese. Fonts are used for rasterization and are not bundled. The now-playing card shows the same invented track and original demo lyric as the screenshots.

`assets/brand/app-icon.png` is the square app icon exported at 1024 px from a local build, which renders `icon/DuckEase.icon`. The banner and the README logo both use it. Re-export it after the icon changes:

```bash
swift scripts/export-app-icon.swift "$PWD/build/darwin-universal/DuckEase Music.app" assets/brand/app-icon.png
```

Badges come from shields.io in the same colors as the banner. The section icons are small SVGs drawn for this fork.

## Local memory snapshot

`assets/screenshots/memory-macos.png` is the maintainer's original Mac screenshot, supplied on 2026-11-08. It shows DuckEase Music at 191 MB. It is separate from the generated UI renders and is not a controlled benchmark or a fixed memory guarantee. No CPU measurement, workload, or comparison baseline is inferred from it.
