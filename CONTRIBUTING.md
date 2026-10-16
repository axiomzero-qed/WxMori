# Contributing / 参与贡献

## Scope / 范围

**中文：** 欢迎提交问题报告和改进建议。WxMori 是面向本地数据的独立 Go CLI/MCP 服务；请准确描述支持范围，不要声称兼容未验证的微信版本、数据库变体或 MCP 宿主。

**English:** Bug reports and improvement proposals are welcome. WxMori is an independent Go CLI and MCP server for local data. Describe support accurately; do not claim compatibility with unverified WeChat versions, database variants, or MCP hosts.

## Changes and reports / 修改与报告

**中文：** 提交前请用 `gofmt` 格式化 Go 文件，并运行项目已有检查。报告问题时提供操作系统、WxMori 版本、脱敏复现步骤和错误代码。请勿提交密钥、数据库、聊天正文、联系人信息或真实账号目录。更改 MCP 参数或结果时同步更新文档。

**English:** Format Go files with `gofmt` and run checks already provided by the project before submitting. Bug reports should include the OS, WxMori version, redacted reproduction steps, and error code. Never submit keys, databases, message bodies, contact details, or real account directories. Update the docs when MCP arguments or results change.

## Validation / 验证

**中文：** 提交前运行 `go test ./...`、`go vet ./...` 和 `python scripts/smoke_test.py --command bin/wxmori.exe`。请区分合成数据、静态真实样本、模型调用与特定宿主测试，不将一种结果推广到未验证场景。现有记录见 `docs/testing.md`。

**English:** Run `go test ./...`, `go vet ./...`, and `python scripts/smoke_test.py --command bin/wxmori.exe` before submitting. Distinguish synthetic data, static real samples, model calls, and host-specific testing. Do not generalize beyond observed results. See `docs/testing.md`.
