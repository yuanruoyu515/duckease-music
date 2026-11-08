# Publishing DuckEase Music / 发布说明

The initial public version is a source publication, `0.1.25-duckease.1`. The repository has a test workflow, no automatic release uploader, and no automatic update feed. It does not use the original author's signing credentials, releases, or website.

首次公开只提供源码。`scripts/build-local.sh` 负责测试并生成本地 ad hoc 签名的 macOS 应用和 DMG，不上传产物，也不做 Apple 公证。

## Source updates

1. Work on a branch based on the public repository's `main`. In the original local checkout, use `codex/public-release`; the older local `main` contains removed website media and is retained only as development history. Do not push that older branch or mirror all refs.
2. Make a focused change, update `CHANGELOG.md` for user-visible behavior, and keep `mygo.json`'s version distinct from the original app.
3. Follow `AGENTS.md`. Run `CGO_ENABLED=0 go test ./internal/...` and `go tool mygo vet .`; render and inspect UI changes. Reproduce README screenshots with `docs/SCREENSHOTS.md` when necessary.
4. Commit the checked change. Push only with the repository owner's authorization. In the original checkout, the explicit branch mapping is `git push origin codex/public-release:main`.
5. Verify the remote commit and the GitHub Actions result. A source push does not publish an installation package.

## Before any future binary release

Document which platforms were actually tested, the signing and notarization state, changes from the previous version, and how recipients can obtain the corresponding source under AGPL-3.0. Keep `NOTICE.md`, `LICENSE`, `DISCLAIMER.md`, and `PRIVACY.md` available with the release information.

Do not reuse the original author's Developer ID, update feed, download URLs, or website identity. Do not upload account state, caches, logs, private credentials, or third-party music as release assets. A disclaimer does not supply missing content or platform permissions.

No release tag or binary is created by these instructions. A future release requires its own verified artifacts and publication decision.
