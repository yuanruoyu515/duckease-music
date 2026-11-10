# Publishing DuckEase Music / 发布说明

Version `0.1.25-duckease.3` provides source and a macOS universal drag-install DMG. The repository has a test workflow, no automatic release uploader, and no signed update feed. The app finds new versions through the GitHub API's latest release, so tag each release `v<version>` and attach the `.dmg`. It does not use the original author's signing credentials, releases, or website.

公开版提供 macOS 通用 DMG 和对应标签源码。`scripts/build-local.sh` 负责测试并生成本地 ad hoc 签名的 macOS 应用和 DMG，不上传产物，也不做 Apple 公证。

## Source updates

1. Work on a branch based on the public repository's `main`. Local `main` tracks `origin/main`. The local-only `codex/pre-publication-history` branch contains removed website media and is retained only as development history. Do not push that backup or mirror all refs.
2. Make a focused change, update `CHANGELOG.md` for user-visible behavior, and keep `mygo.json`'s version distinct from the original app.
3. Follow `AGENTS.md`. Run `CGO_ENABLED=0 go test ./internal/...` and `go tool mygo vet .`; render and inspect UI changes. Reproduce README screenshots with `docs/SCREENSHOTS.md` when necessary.
4. Commit the checked change. Push only with the repository owner's authorization. In this checkout, ordinary `git push` targets `origin/main`.
5. Verify the remote commit and the GitHub Actions result. A source push does not publish an installation package.

## Binary releases

Document which platforms were actually tested, the signing and notarization state, changes from the previous version, and how recipients can obtain the corresponding source under AGPL-3.0. Keep `NOTICE.md`, `LICENSE`, `DISCLAIMER.md`, and `PRIVACY.md` available with the release information.

Do not reuse the original author's Developer ID, update feed, download URLs, or website identity. Do not upload account state, caches, logs, private credentials, or third-party music as release assets. A disclaimer does not supply missing content or platform permissions.

For the macOS release, run `./scripts/build-local.sh -platform darwin/universal`. Mount the resulting DMG, check that Applications links to `/Applications`, verify both executable architectures and the app signature, then eject it. The app includes `LICENSE`, `NOTICE.md`, `DISCLAIMER.md`, `PRIVACY.md`, and `SOURCE.md` through `mygo.json` resources.

Commit the checked source before the final build. Tag that exact commit as `v<version>`, push the authorized source and tag, and create a GitHub Release with the DMG and a `SHA256SUMS.txt` file. Record the tested machine and signing state in the release notes. Confirm that the remote tag resolves to the build commit and the uploaded artifact's checksum matches the local file. Never move an already published tag.
