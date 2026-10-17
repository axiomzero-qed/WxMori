# Validation record / 验证记录

**中文：** 日期：2026-10-17；版本：`v0.1.0-alpha.1`。以下是实测结果，不代表所有微信版本、模型或宿主均兼容。

**English:** Date: 2026-10-17; version: `v0.1.0-alpha.1`. These are observed results, not proof of compatibility with every WeChat version, model, or host.

| Check / 检查 | Evidence / 证据 |
| --- | --- |
| Go regression / Go 回归 | `go test ./...` and `go vet ./...` passed locally on Windows / Windows 本机通过 |
| Cloud CI / 云端检查 | Windows, Linux and macOS passed; Linux `go test -race ./...` passed / 三系统通过，Linux 并发检查通过；[run](https://github.com/axiomzero-qed/WxMori/actions/runs/38944361112) |
| Crypto / 加密 | Independent PBKDF2 vector; multi-block CBC; page-number/ciphertext/IV/MAC tampering; later-page failure cleanup / 独立派生向量、多块 CBC、页号及密文/IV/MAC 篡改、失败清理 |
| Parsing / 解析 | Per-shard senders; zstd; compound types; nested XML; multiline text; repeated zero-ID messages; dedup / 每库发送者、压缩、复合类型、嵌套 XML、多行、零 ID 重复、去重 |
| Snapshot / 快照 | Plaintext WAL committed data retained, originals unchanged, error cleanup / 明文 WAL 已提交数据保留、源文件不变、错误清理 |
| Independent MCP client / 独立 MCP 客户端 | All six tools on `2024-11-05`, `2025-03-26`, `2025-06-18`, `2025-11-25`; text equals structured JSON / 六工具、四版本、文本与结构化结果一致 |
| Recovery / 恢复 | Ambiguity, missing fields, wrong types, invalid dates/months/limits, rejected path arguments / 同名、缺字段、错误类型、非法日期月份上限、拒绝路径参数 |
| GPT-6 Luna / 模型调用 | Actual discovery, Chinese/English export, search, pagination, recovery / 实际发现、中英文导出、搜索、分页、恢复；见 [trial](luna-call-trial.md) |

## Local real-data sample / 本机真实数据样本

**中文：** 仅在本机使用真实数据的静态主库副本，未提交身份、密钥或正文：30 个数据库（1,112,178,688 字节）的每页 HMAC 全通过；其中 CLI/MCP 所需的 12 个聊天与联系人库全通过 `PRAGMA integrity_check`，解析出 685 个会话。辅助全文索引不在首版输出范围，其专用分词器可能无法被通用 SQLite 校验。无法解析的发送者会明确警告，不能保证每位发送者均能映射为昵称。

**English:** Local validation used static copies of real main databases; no identities, keys, or bodies were committed. Every page in 30 databases (1,112,178,688 bytes) passed HMAC authentication. All 12 chat/contact databases used by CLI/MCP passed `PRAGMA integrity_check`, and 685 sessions were parsed. Auxiliary full-text indexes are outside this release; custom tokenizers may prevent generic SQLite validation. Unresolved senders produce warnings; nickname resolution is not guaranteed for every sender.

**中文：** 上述已发布版本测试副本明确不含源加密 WAL，属于静态主库样本，不是微信最新记录。后续开发版本增加加密 WAL 校验和私有副本回放；相关自动化检查及当前单账号真实数据的读取与导出验收均已通过。该真实样本的 12 个 WAL 文件各有一个旧代次尾部被排除，当前代次提交帧为 0；提交帧回放仍只有合成测试覆盖。程序不得静默丢弃 WAL 帧。

**English:** The published-version fixtures explicitly exclude source encrypted WAL and represent static main-database samples, not the latest WeChat history. Later development added encrypted-WAL validation and private-copy replay; its automated checks and read/export acceptance on the current single-account sample have passed. Each of that sample's 12 WAL files had one excluded stale-generation tail, and there were zero current-generation committed frames; committed-frame replay therefore has synthetic coverage only. The program must not silently discard WAL frames.

**中文：** GPT-6 Luna 直接使用交接材料明确指定的会话 ID，分别导出 TXT 和 JSONL，各 381 条；文件存在性、条数、UTF-8 与 SHA-256 均通过，235 条含换行的正文在 TXT 中完整保留。真实身份、路径和正文未进入公开报告。

**English:** GPT-6 Luna used the exact session ID specified in the supplied handoff and exported TXT and JSONL with 381 records each. File existence, counts, UTF-8 and SHA-256 checks passed; all 235 returned multiline bodies were preserved in TXT. Real identities, paths and bodies are absent from public reports.

## Working-branch development checks / 工作分支开发检查

**中文：** 日期：2026-10-17；版本：`v0.2.0-alpha.1`。以下开发版自动化、CI 和真实样本记录与上方 alpha1 历史分开。此前提交的三系统 CI 验证了完整程序启动和独立 stdio MCP 冒烟，但早于当前调试器捕获及 WAL 回放后端。当前源码本机测试与静态检查通过，当前发布 CI 仍待完成。Luna/Python stdio 单账号试验已完成捕获、DPAPI 重载、12 库读取及 TXT/JSONL 导出。

**English:** Date: 2026-10-17; version: `v0.2.0-alpha.1`. These development automation, CI, and real-sample results are separate from the alpha1 history above. Earlier three-system CI verified full executable startup and an independent stdio MCP smoke test, but predates the current debugger capture and WAL-replay backends. Current-source local tests and static checks pass; release CI is still pending. A single-account Luna/Python stdio trial completed capture, DPAPI reload, reads of 12 databases, and TXT/JSONL exports.

| Check / 检查 | Evidence / 证据 |
| --- | --- |
| Go regression and static checks / Go 回归与静态检查 | `go test ./...` and `go vet ./...` pass locally for `v0.2.0-alpha.1`. / `v0.2.0-alpha.1` 本机通过 `go test ./...` 和 `go vet ./...`。 |
| Cross-platform CI and stdio / 跨平台 CI 与 stdio | Commit `1bb45bd87cff10b0b8604a6a04a2d225e9bb9043` passed on Windows, Linux, and macOS, including full executable startup and an independent Python stdio MCP smoke test with seven tools across four protocol versions. [Run](https://github.com/axiomzero-qed/WxMori/actions/runs/39025380150) / 提交 `1bb45bd87cff10b0b8604a6a04a2d225e9bb9043` 通过 Windows、Linux 和 macOS 检查，包括完整程序启动及独立 Python stdio MCP 冒烟测试（七个工具、四个协议版本）。[运行记录](https://github.com/axiomzero-qed/WxMori/actions/runs/39025380150) |
| Capture MCP protocol / 捕获 MCP 协议 | `TestCaptureToolProtocolWithSyntheticSource` passed using an in-memory official SDK transport; this is not an independent stdio test or model trial. / 使用内存中的官方 SDK 传输通过；这不是独立 stdio 测试或模型试用。 |
| Capture backend / 捕获后端 | Windows synthetic child-process test passed for debugger arguments, child continuation/re-execution after detach, and repeated cycles without handle leaks. Separate read-only polling scan/HMAC validation also passed synthetically. / Windows 合成子进程测试验证调试器参数、分离后子进程继续运行并可再次执行，以及重复周期无句柄泄漏；只读轮询扫描与 HMAC 校验也通过合成测试。 |
| Encrypted WAL / 加密 WAL | Pure-Go tests passed for both checksum byte orders, commit boundaries, uncommitted tails, stale generations, and corruption. Synthetic snapshot integration retained source hashes and validated the replay boundary. In actual capture, 12 WALs had stale-generation tails and zero current-generation committed frames, so committed-frame replay remains synthetic-only. / 纯 Go 测试覆盖两种校验和字节序、提交边界、未提交尾帧、旧代次和损坏并通过。合成快照集成测试验证了回放边界且源哈希不变。真实捕获中的 12 个 WAL 有旧代次尾帧、当前代次提交帧为 0，因此提交帧回放仍仅由合成测试覆盖。 |
| Protected keyring / 受保护密钥环 | DPAPI `CurrentUser` tests passed. / DPAPI `CurrentUser` 测试通过。 |
| Security-software handling / 安全软件处理 | 360 history confirmed deletion of an earlier executable build. A later same-name rebuild started after 360 was exited; neither fact establishes the earlier detection was false. For blocks, submit the individual file to the vendor for review; do not recommend disabling protection. / 360 记录确认曾删除早期程序构建；退出 360 后重建同名程序可启动，但两者均不能证明此前检测为误报。若被拦截，建议按厂商流程逐文件提交审查，不建议关闭防护。 |
| Read-only login trial / 只读登录试验 | After re-login, polling found 0 verified keys for 12 required databases (5 processes, 0 access-denied errors, 45 passes, about 16.57 GB read, 86,791 candidates). It stopped cleanly; no export was run. This was the read-only method, not the new debugger backend. / 重新登录后轮询对 12 个所需数据库找到 0 个已验证密钥（5 个进程、0 次拒绝访问、45 轮、读取约 16.57 GB、86,791 个候选）。任务正常停止，未执行导出。此结果来自只读方式，不是新的调试器后端。 |
| Luna capture and export / Luna 捕获与导出 | On one Weixin 4.1.15.56 account via Luna's Python stdio host: capture verified 12/12 keys; a fresh MCP process loaded them from DPAPI with `master_key_available=false`; status was ready; exact user-specified session lookup returned one result. TXT and JSONL exports each had 381 records and matching hashes; UTF-8 passed, TXT had 381 headers and preserved all 235 multiline bodies, and JSONL had 381 valid rows. No identities, paths, or bodies are reported here. / 通过 Luna Python stdio 宿主对一个 Weixin 4.1.15.56 账号实测：捕获验证 12/12 密钥；新 MCP 进程在 `master_key_available=false` 时从 DPAPI 加载密钥，状态 ready，按用户指定会话精确查询返回一个结果。TXT 与 JSONL 各 381 条且摘要匹配；UTF-8 通过，TXT 有 381 个标题并保留全部 235 条多行正文，JSONL 有 381 行有效记录。此处不公开身份、路径或正文。 |

**中文：** `capture_key` 仍为实验性功能。当前真实样本的 12 库读取和导出已通过；其 WAL 当前代次提交帧为 0，已提交帧回放仍只由合成测试覆盖。当前发布 CI 待完成。此前只读轮询试验曾为 0/12。

**English:** `capture_key` remains experimental. Reads and exports passed on the current real sample; its WALs had zero current-generation committed frames, so committed-frame replay is covered by synthetic tests only. Current release CI is pending. The earlier read-only polling trial was 0/12.

## Remaining coverage / 待覆盖范围

**中文：** 自动捕获实测仅覆盖一个 Windows AMD64/Weixin 4.1.15.56 单账号及 Luna Python stdio 宿主；其他版本、模型和商业宿主未实测，当前发布 CI 待完成。已发布 alpha1 未实现自动取密钥且不支持非空加密 WAL。实时监听、附件原文件及多账号合并均未实现。

**English:** Automatic capture was tested only on one Windows AMD64/Weixin 4.1.15.56 single-account sample through Luna's Python stdio host; other versions, models, and commercial hosts remain untested, and release CI is pending. Published alpha1 has no automatic key capture and does not support non-empty encrypted WAL. Live monitoring, attachment binaries, and multi-account merging are not implemented.

**中文：** 协议参考 [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) 与 [stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)，日志语义参考 [SQLite WAL](https://www.sqlite.org/wal.html)。

**English:** Protocol references: [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) and [stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). Journal semantics: [SQLite WAL](https://www.sqlite.org/wal.html).
