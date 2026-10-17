# Roadmap / 路线图

## Available in alpha / Alpha 已实现

- Local master-key input, per-page authentication, verified chat SQLite snapshots. / 本地主密钥输入、逐页校验、聊天库完整性检查与快照。
- Six stdio MCP tools, CLI, monthly TXT/JSONL exports, synthetic demo. / 六个 stdio MCP 工具、CLI、月度 TXT/JSONL 和合成演示。
- Flat schemas, bounded previews, stable errors and candidate selection for weaker models. / 扁平参数、有界预览、稳定错误与候选选择，便于较弱模型调用。

## Next / 下一阶段

1. Working branch: experimental independent Windows 64-bit login-time, read-only key capture; real-WeChat login trial and versioned fixtures are pending. No WeFlow DLL or code. This is not part of published `v0.1.0-alpha.1`. / 工作分支：实验性独立 Windows 64 位登录时只读密钥捕获；真实微信登录试验和版本样本仍待完成。不使用 WeFlow DLL 或代码，且不属于已发布的 `v0.1.0-alpha.1`。
2. Authenticated encrypted WAL snapshots with committed-frame and source-race tests. / 可校验的加密 WAL 快照及提交帧、并发源变化测试。
3. More WeChat versions, models and actual host integrations; publish the tested matrix. / 增加微信版本、模型与实际宿主测试，公布实测矩阵。
4. Improve unresolved-sender diagnostics and incremental indexing before live monitoring. / 改善发送者未解析诊断与增量索引，再实现实时监听。

## Later / 后续

- Explicit multi-account merge policies; attachment export; additional platform-specific acquisition methods. / 明确多账号合并规则、附件导出及其他平台的取密钥方法。

**中文：** 以上尚未实现事项不作发布日期或兼容性承诺。

**English:** Unimplemented items have no promised release date or compatibility guarantee.
