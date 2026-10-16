# Changelog / 更新记录

## v0.1.0-alpha.1 — 2026-10-17

**中文：** 首个开源预览版：六个 stdio MCP 工具，按月 TXT/JSONL 导出，本地主密钥解密和已有 SQLite 数据读取；提供合成演示、中英对照文档、错误恢复及依赖许可证。

**English:** First open-source preview: six stdio MCP tools, monthly TXT/JSONL exports, local master-key decryption and existing SQLite input, with synthetic demos, paired Chinese/English docs, recoverable errors and dependency licenses.

**中文：** Windows、Linux、macOS 云端检查与四种 MCP 协议版本测试通过。GPT-6 Luna 已完成合成调用和真实静态样本导出，后者每种格式 381 条；测试范围见 `docs/testing.md`。

**English:** Cloud checks passed on Windows, Linux and macOS, along with four MCP protocol versions. GPT-6 Luna completed synthetic calls and exports from one static real-data sample with 381 records per format. See `docs/testing.md` for scope.

**中文：** 本版需要已有主密钥或解密数据库；非空加密 WAL 会中止操作。自动取密钥、实时监听和附件原文件未实现。

**English:** This release requires an existing master key or decrypted databases. Non-empty encrypted WAL stops the operation. Automatic key acquisition, live monitoring and attachment binaries are not implemented.
