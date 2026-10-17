# Changelog / 更新记录

## v0.2.0-alpha.1 — release candidate / 候选版

**中文：** 新增 Windows AMD64 登录时实验性密钥捕获（调试器 `auto` 与轮询 `read_only`）、DPAPI `CurrentUser` 每库密钥环，以及私有副本中的加密 WAL 校验和回放。MCP `capture_key` 和 CLI 现在都支持选择捕获模式。非空加密 WAL 仍不受已发布 `v0.1.0-alpha.1` 支持。当前发布 CI 尚待完成；自动捕获仅在一个 Weixin 4.1.15.56 单账号样本及 Luna/Python stdio 宿主实测。

**English:** Adds experimental Windows AMD64 login-time key capture (debugger `auto` or polling `read_only`), a DPAPI `CurrentUser` per-database keyring, and encrypted-WAL validation/replay in private copies. Both MCP `capture_key` and the CLI support capture-mode selection. Published `v0.1.0-alpha.1` still does not support non-empty encrypted WAL. Release CI is pending; automatic capture was tested only on one Weixin 4.1.15.56 single-account sample through Luna/Python stdio.

**中文：** Luna 实测捕获并验证 12/12 核心数据库密钥，新 MCP 进程在无手动主密钥时从 DPAPI 重载；保留 WAL 的读取及 381 条 TXT/JSONL 导出通过。该 WAL 当前代次提交帧数为 0，因此真实提交帧回放仍未验收，只有合成覆盖。详细测试范围见 `docs/testing.md` 和 `docs/releases/v0.2.0-alpha.1.md`。

**English:** Luna's trial captured and verified keys for all 12 core databases; a fresh MCP process reloaded them from DPAPI without a manual master key. Reads and 381-record TXT/JSONL exports passed with retained WALs. Those WALs had zero current-generation committed frames, so real committed-frame replay is not yet accepted and has synthetic coverage only. See `docs/testing.md` and `docs/releases/v0.2.0-alpha.1.md` for scope.

## v0.1.0-alpha.1 — 2026-10-17

**中文：** 首个开源预览版：六个 stdio MCP 工具，按月 TXT/JSONL 导出，本地主密钥解密和已有 SQLite 数据读取；提供合成演示、中英对照文档、错误恢复及依赖许可证。

**English:** First open-source preview: six stdio MCP tools, monthly TXT/JSONL exports, local master-key decryption and existing SQLite input, with synthetic demos, paired Chinese/English docs, recoverable errors and dependency licenses.

**中文：** Windows、Linux、macOS 云端检查与四种 MCP 协议版本测试通过。GPT-6 Luna 已完成合成调用和真实静态样本导出，后者每种格式 381 条；测试范围见 `docs/testing.md`。

**English:** Cloud checks passed on Windows, Linux and macOS, along with four MCP protocol versions. GPT-6 Luna completed synthetic calls and exports from one static real-data sample with 381 records per format. See `docs/testing.md` for scope.

**中文：** 本版需要已有主密钥或解密数据库；非空加密 WAL 会中止操作。自动取密钥、实时监听和附件原文件未实现。

**English:** This release requires an existing master key or decrypted databases. Non-empty encrypted WAL stops the operation. Automatic key acquisition, live monitoring and attachment binaries are not implemented.
