# Contributing / 贡献说明

Small fixes with clear reproduction steps are welcome. This is a personal maintenance project; a report or pull request does not imply a response deadline or a promise to merge.

欢迎能复现、范围清楚的小修复。维护者按自己的时间处理，没有固定回复或合并时限。

## Before changing code

Read [NOTICE.md](NOTICE.md), [DISCLAIMER.md](DISCLAIMER.md), [AGENTS.md](AGENTS.md), and [MAINTENANCE.md](MAINTENANCE.md). Preserve upstream attribution and the AGPL license. Contributions are provided under the project's existing license; do not include material you lack the right to contribute.

Use a focused branch. Describe the problem, resulting behavior, and checks actually run. UI fixes should include a screenshot with demo data. Do not include credentials, real account sessions, private playlists, copyrighted media added for illustration, or generated build output.

## Checks

```bash
CGO_ENABLED=0 go test ./internal/...
go tool mygo vet .
```

Follow the behavior-focused test guidance in `AGENTS.md`. Run a new UI test with `-count=5`. Render visual changes and inspect them; do not test wording or label order as behavior.

Interface changes should follow the pinned MyGo version. Consult `.agents/skills/mygo-maintenance/SKILL.md`; do not silently upgrade dependencies while fixing an unrelated bug.

## Reporting issues

Include your OS and architecture, commit or build version, reproduction steps, expected behavior, and redacted diagnostics. A screenshot of a private account must be anonymized before upload.

Please do not request access-control or payment bypasses, stolen sessions, aggressive scraping, or removal of upstream attribution. A platform risk-control response should be handled with fewer requests and a cooldown.

权利问题请使用专门的权利反馈表单；个人文件、凭据及私人函件不要贴进公开 issue。
