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

The banner generator uses native AppKit drawing and the fork's existing duck artwork. It registers Century Gothic from the local Microsoft Excel font folder when available; otherwise it falls back to the system font. Chinese text uses PingFang SC. Fonts are used for rasterization and are not bundled in this repository.

The section icons are small SVGs drawn for this fork. Documentation graphics follow the app's existing red and duck identity. The light and dark banners share geometry and use separate background and text colors.
