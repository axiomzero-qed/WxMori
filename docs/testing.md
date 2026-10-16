# Validation record / 验证记录

**中文：** 日期：2026-10-17；版本：`v0.1.0-alpha.1`。以下是实测结果，不代表所有微信版本、模型或宿主均兼容。

**English:** Date: 2026-10-17; version: `v0.1.0-alpha.1`. These are observed results, not proof of compatibility with every WeChat version, model, or host.

| Check / 检查 | Evidence / 证据 |
| --- | --- |
| Go regression / Go 回归 | `go test ./...` and `go vet ./...` passed locally on Windows / Windows 本机通过 |
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

## Remaining coverage / 待覆盖范围

**中文：** 商业宿主（如 Claude Desktop、Cursor）和其他模型尚未实测。跨系统 CI 与发布构建状态以 GitHub Actions 为准，本机环境为 Windows。自动取密钥、加密 WAL、实时监听、附件原文件及多账号合并均未实现。

**English:** Commercial hosts such as Claude Desktop and Cursor, and other models, have not been tested. GitHub Actions records cross-platform CI and release builds; local execution used Windows. Automatic key acquisition, encrypted WAL, live monitoring, attachment binaries, and multi-account merging are not implemented.

**中文：** 协议参考 [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) 与 [stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)，日志语义参考 [SQLite WAL](https://www.sqlite.org/wal.html)。

**English:** Protocol references: [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) and [stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). Journal semantics: [SQLite WAL](https://www.sqlite.org/wal.html).
