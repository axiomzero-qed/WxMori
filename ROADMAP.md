# Roadmap / 路线图

## Available in the current preview release / 当前预览版已实现

- Local master-key input, per-page authentication, verified chat SQLite snapshots. / 本地主密钥输入、逐页校验、聊天库完整性检查与快照。
- Seven stdio MCP tools, including experimental Windows key capture, CLI, monthly TXT/JSONL exports, synthetic demo. / 七个 stdio MCP 工具（含实验性 Windows 取密钥）、CLI、月度 TXT/JSONL 和合成演示。
- Flat schemas, bounded previews, stable errors and candidate selection for weaker models. / 扁平参数、有界预览、稳定错误与候选选择，便于较弱模型调用。

## Next / 下一阶段

1. Preview release `v0.2.0-alpha.1` includes independent experimental Windows 64-bit login-time capture. Default mode uses temporary debugger hardware breakpoints and briefly pauses threads while restoring debug registers on cleanup; no code patch or DLL injection. `read_only` selects polling. One Weixin 4.1.15.56 single-account trial captured and HMAC-verified 12/12 core database keys, then completed reads and exports with retained encrypted WALs. That sample had zero current-generation committed WAL frames, so committed-frame replay has synthetic coverage only. No WeFlow DLL or code is used. / 预览版 `v0.2.0-alpha.1` 包含独立的 Windows 64 位实验性登录时捕获。默认方式使用临时调试器硬件断点，短暂暂停线程并在清理时恢复调试寄存器；不修改代码或注入 DLL。`read_only` 选择轮询。一次 Weixin 4.1.15.56 单账号试验捕获并通过 HMAC 验证了 12/12 个核心数据库密钥，随后在保留加密 WAL 时完成读取和导出。该样本当前代次的 WAL 提交帧数为 0，因此提交帧回放目前只有合成测试覆盖。不使用 WeFlow DLL 或代码。
2. Broaden committed-frame and source-race coverage, and test additional WeChat versions, models, and hosts. Current three-system code CI passed; capture compatibility beyond the single sample remains unverified. / 扩展提交帧与源变化测试，并测试更多微信版本、模型和宿主。当前三系统代码 CI 已通过；该单一样本之外的捕获兼容性仍未验证。
3. More WeChat versions, models and actual host integrations; publish the tested matrix. / 增加微信版本、模型与实际宿主测试，公布实测矩阵。
4. Improve unresolved-sender diagnostics and incremental indexing before live monitoring. / 改善发送者未解析诊断与增量索引，再实现实时监听。

## Later / 后续

- Explicit multi-account merge policies; attachment export; additional platform-specific acquisition methods. / 明确多账号合并规则、附件导出及其他平台的取密钥方法。

**中文：** 以上尚未实现事项不作发布日期或兼容性承诺。

**English:** Unimplemented items have no promised release date or compatibility guarantee.
