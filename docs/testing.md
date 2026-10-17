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

**中文：** 测试副本明确不含源加密 WAL，属于静态主库样本，不是微信最新记录。实际输入遇到非空加密 WAL 仍会中止，程序不会为获得通过结果而静默丢弃 WAL。

**English:** These fixtures explicitly exclude source encrypted WAL and represent static main-database samples, not the latest WeChat history. Real inputs with non-empty encrypted WAL still stop; the program does not silently discard it to obtain a passing result.

**中文：** GPT-6 Luna 直接使用交接材料明确指定的会话 ID，分别导出 TXT 和 JSONL，各 381 条；文件存在性、条数、UTF-8 与 SHA-256 均通过，235 条含换行的正文在 TXT 中完整保留。真实身份、路径和正文未进入公开报告。

**English:** GPT-6 Luna used the exact session ID specified in the supplied handoff and exported TXT and JSONL with 381 records each. File existence, counts, UTF-8 and SHA-256 checks passed; all 235 returned multiline bodies were preserved in TXT. Real identities, paths and bodies are absent from public reports.

## Working-branch development checks / 工作分支开发检查

**中文：** 日期：2026-10-17；版本：`v0.2.0-dev`。以下是开发分支的自动化或合成数据检查，与上方已发布 `v0.1.0-alpha.1` 的记录分开；它们不代表完整可执行程序启动、独立 stdio 客户端、模型调用或真实微信兼容性。

**English:** Date: 2026-10-17; version: `v0.2.0-dev`. These are automated or synthetic-data checks for the development branch, separate from the published `v0.1.0-alpha.1` record above. They do not establish full executable startup, an independent stdio client, model calls, or compatibility with real WeChat.

| Check / 检查 | Evidence / 证据 |
| --- | --- |
| Go regression and static checks / Go 回归与静态检查 | Working-branch `go test ./...` and `go vet ./...` passed locally on Windows. / 工作分支在 Windows 本机通过 `go test ./...` 和 `go vet ./...`。 |
| Capture MCP protocol / 捕获 MCP 协议 | `TestCaptureToolProtocolWithSyntheticSource` passed using an in-memory official SDK transport; this is not an independent stdio test or model trial. / 使用内存中的官方 SDK 传输通过；这不是独立 stdio 测试或模型试用。 |
| Read-only capture / 只读捕获 | Windows synthetic child-process memory scan and HMAC validation passed. / Windows 合成子进程内存扫描和 HMAC 校验通过。 |
| Protected keyring / 受保护密钥环 | DPAPI `CurrentUser` tests passed. / DPAPI `CurrentUser` 测试通过。 |
| Full executable startup / 完整程序启动 | Failed with WinError 5. The cause is unconfirmed; the user's 360 Antivirus protection history is still awaited. This is not confirmed as an antivirus block or false positive. / 启动失败并返回 WinError 5。原因尚未确认，仍待用户查看 360 杀毒的拦截记录；不能据此认定是安全软件拦截或误报。 |
| Real-account capture / 真实账号捕获 | The user selected their usual current account, but the real login trial is pending. A preliminary post-login scan yielded zero verified keys for 12 required databases. / 用户已选择平时使用的当前账号，但真实登录试验尚未进行。一次初步登录后扫描对 12 个所需数据库均未得到已验证密钥。 |

**中文：** 当前七个 MCP 工具中的 `capture_key` 仍为实验性功能。独立完整程序 stdio 测试、新捕获工具的模型实际调用和登录期间真实捕获均待验证；合成协议测试不得表述为上述任一项已通过。

**English:** `capture_key`, one of the current seven MCP tools, remains experimental. An independent full-program stdio test, an actual model call for the new capture tool, and real capture during login remain unverified; the synthetic protocol test must not be reported as passing any of these.

## Remaining coverage / 待覆盖范围

**中文：** 商业宿主（如 Claude Desktop、Cursor）和其他模型尚未实测。跨系统 CI 与发布构建状态以 GitHub Actions 为准，本机环境为 Windows。已发布 alpha1 未实现自动取密钥；开发分支实验采集尚未通过真实登录验证。加密 WAL、实时监听、附件原文件及多账号合并均未实现。

**English:** Commercial hosts such as Claude Desktop and Cursor, and other models, have not been tested. GitHub Actions records cross-platform CI and release builds; local execution used Windows. Published alpha1 has no automatic key capture; experimental development capture has not yet been validated with a real login. Encrypted WAL, live monitoring, attachment binaries, and multi-account merging are not implemented.

**中文：** 协议参考 [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) 与 [stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)，日志语义参考 [SQLite WAL](https://www.sqlite.org/wal.html)。

**English:** Protocol references: [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) and [stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). Journal semantics: [SQLite WAL](https://www.sqlite.org/wal.html).
