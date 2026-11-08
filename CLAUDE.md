# CLAUDE.md

Ruoyu 的个人维护分支（fork 自 LeslieYon/mygo-cloudmusic，它是 egoist/mygo-cloudmusic 下线前最后一个完整提交 3c0eb71 的镜像；原仓库已归档，最后一个提交删掉了全部代码）。

先读 AGENTS.md（测试规范）、MAINTENANCE.md（维护要点）和 `.agents/skills/mygo-maintenance/SKILL.md`（MyGo 框架规则）。

## Version control
- This folder is a git repo. Run `git status` before starting; if there are uncommitted changes, tell me first.
- Commit after each verifiable stage with a message saying what changed and why. Report the commit hash.
- Do not push, force-push, rewrite history, or move tags unless I ask.
- Use a new branch for large or experimental changes.
- Never commit secrets or files ignored by .gitignore.
- Remote `upstream` is LeslieYon/mygo-cloudmusic (read-only reference). Remote `origin` is yuanruoyu515/duckease-music. Publish the cleaned `codex/public-release` branch to `origin/main`; do not push the older local `main` history, which contains removed website media.

## Build and test (verified 2026-11-08, Go 1.27.3 from Homebrew)
- `CGO_ENABLED=0 go test ./internal/...`
- `scripts/build-local.sh` — local macOS build, ad-hoc signed, no notarization, no upload.
