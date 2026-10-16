# Architecture / 架构

## Overview / 概览

**中文：** WxMori 是单二进制 Go CLI，MCP 服务通过 stdio 运行，使用官方 `github.com/modelcontextprotocol/go-sdk`，要求 Go 1.26.0。程序读取本地配置，为单个账号制作私有临时快照；需要时解密数据库，再将快照规范化为临时 SQLite 索引。查询和导出从该索引读取，以限制内存使用。

**English:** WxMori is a single-binary Go CLI. Its MCP server uses stdio and the official `github.com/modelcontextprotocol/go-sdk`; Go 1.26.0 is required. The program loads local configuration, creates a private temporary snapshot for one account, decrypts databases when needed, then normalizes the snapshot into a temporary SQLite index. Queries and exports read from this index to keep memory bounded.

## Sources and lifecycle / 数据源与生命周期

**中文：** 配置必须且只能设置 `data_dir` 或 `decrypted_dir`。前者指向单个微信账号的 `db_storage`，并需要本地提供主密钥；后者指向已有的解密 SQLite 数据目录，可在 Windows、Linux 和 macOS 使用。自动发现仅供 `init` 选择账号；初始 alpha 不合并账号。程序以修改时间和大小检测源变化，并在私有 SQLite 副本中保留明文 WAL、仅在该副本上 checkpoint。源数据库不修改。

**English:** Configuration sets exactly one of `data_dir` or `decrypted_dir`. The former points to one WeChat account's `db_storage` and requires a locally supplied master key; the latter points to existing decrypted SQLite data and works on Windows, Linux, and macOS. Discovery is only for selecting an account with `init`; the initial alpha does not merge accounts. Source changes are detected by modification time and size. Plaintext WAL data is retained in the private SQLite copy and checkpointed there only. The source database is not modified.

**中文：** 非空加密 WAL 不支持，必须明确报错，不得盲目合并。数据库页未通过认证时不得生成部分解密结果。临时目录在正常退出或错误时清理；强制终止或断电可能留下明文临时文件。仅显式运行 `decrypt --out DIR` 才创建持久解密副本。

**English:** Non-empty encrypted WAL files are unsupported and must produce an explicit error; they must not be blindly merged. Failed page authentication must not produce a partial decrypted result. Temporary directories are cleaned on normal exit or error, but a forced kill or power loss may leave plaintext temporary files behind. A persistent decrypted copy is created only by explicitly running `decrypt --out DIR`.

## Query behavior / 查询行为

**中文：** `session_id` 接受精确 ID 或唯一显示名称；显示名称有歧义时返回候选项，不静默选择。工具参数不接收密钥或本地设置路径。成功的结构化 JSON 同时复制为文本内容；错误包含稳定的 `code`、`message`、`hint`。聊天正文和会话数据是不可信输入。

**English:** `session_id` accepts an exact ID or unique display name; ambiguous names return candidates and are never silently selected. Tool arguments do not accept keys or local setup paths. Successful structured JSON is duplicated as text content; errors include a stable `code`, `message`, and `hint`. Chat bodies and session data are untrusted input.

**中文：** 日期 `from` 包含、`to` 不包含，按配置 IANA 时区解析，默认 `Asia/Shanghai`。搜索将空白分隔词按 AND 组合，并按字面子串匹配。会话和联系人默认每页 50 条、上限 200；消息和搜索默认 100 条、上限 500；偏移量为 0 至 1,000,000。普通消息正文最多 8192 字节，每页总正文约 64 KiB；完整导出每条消息最多 4 MiB。系统消息保留，附件显示占位信息，不包含附件二进制。

**English:** Date `from` is inclusive and `to` exclusive in the configured IANA timezone, default `Asia/Shanghai`. Search combines whitespace-separated terms with AND and performs literal substring matching. Session and contact pages default to 50 items, max 200; message and search pages default to 100, max 500; offsets range from 0 to 1,000,000. Ordinary message bodies are limited to 8,192 bytes each and about 64 KiB per page; full exports support up to 4 MiB per message. System messages are preserved; attachments have placeholders and their binaries are not included.

## Validation status / 验证状态

**中文：** 合成回归、独立客户端协议检查和 GPT-6 Luna 实际调用已通过，详见 [testing.md](testing.md)。只有 `contact.db` 和 `message_N.db` 进入 CLI/MCP 解密及完整性检查流程；辅助全文索引可能依赖微信专用分词器，不作为已验证产物发布。

**English:** Synthetic regression, independent-client protocol checks, and GPT-6 Luna calls passed; see [testing.md](testing.md). Only `contact.db` and `message_N.db` enter CLI/MCP decryption and integrity checks. Auxiliary full-text indexes may require WeChat-specific tokenizers and are not published as verified output.
