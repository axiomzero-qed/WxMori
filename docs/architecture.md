# Architecture / 架构

## Overview / 概览

**中文：** WxMori 是单二进制 Go CLI，MCP 服务通过 stdio 运行，使用官方 `github.com/modelcontextprotocol/go-sdk`，要求 Go 1.26.0。程序读取本地配置，为单个账号制作私有临时快照；需要时解密数据库，再将快照规范化为临时 SQLite 索引。查询和导出从该索引读取，以限制内存使用。

**English:** WxMori is a single-binary Go CLI. Its MCP server uses stdio and the official `github.com/modelcontextprotocol/go-sdk`; Go 1.26.0 is required. The program loads local configuration, creates a private temporary snapshot for one account, decrypts databases when needed, then normalizes the snapshot into a temporary SQLite index. Queries and exports read from this index to keep memory bounded.

## Sources and lifecycle / 数据源与生命周期

**中文：** 配置必须且只能设置 `data_dir` 或 `decrypted_dir`。前者指向单个微信账号的 `db_storage`；已发布 `v0.1.0-alpha.1` 需要本地手动提供主密钥，工作分支实验功能可在登录捕获并验证每库密钥后从 DPAPI 密钥环自动加载。后者指向已有的解密 SQLite 数据目录，可在 Windows、Linux 和 macOS 使用。自动发现仅供 `init` 选择账号；初始 alpha 不合并账号。程序以修改时间和大小检测源变化，并在私有 SQLite 副本中保留明文 WAL、仅在该副本上 checkpoint。源数据库不修改。

**English:** Configuration sets exactly one of `data_dir` or `decrypted_dir`. The former points to one WeChat account's `db_storage`. Published `v0.1.0-alpha.1` requires a manually supplied local master key; the experimental working-branch flow can load per-database keys automatically from the DPAPI keyring after login capture and validation. The latter points to existing decrypted SQLite data and works on Windows, Linux, and macOS. Discovery is only for selecting an account with `init`; the initial alpha does not merge accounts. Source changes are detected by modification time and size. Plaintext WAL data is retained in the private SQLite copy and checkpointed there only. The source database is not modified.

## Experimental Windows capture / Windows 实验性捕获

**中文：** 工作分支的实验性 Windows 64 位捕获器通过只读进程内存读取 API 扫描微信登录期间可读区域。它寻找 AES-256 密钥调度、旧式原始密钥文本和对所选数据库盐的引用，再对本机已选数据库页执行 HMAC 验证；仅经验证的每库密钥进入 DPAPI `CurrentUser` 密钥环。Windows `VirtualQueryEx` 与 `ReadProcessMemory` 文档描述了区域查询和只读复制接口；上游 SQLCipher 使用 OpenSSL 的公开实现也展示了数据库加密路径，但这些来源不证明当前 Windows 微信构建采用相同实现。[Windows API](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualqueryex) · [Tencent WCDB](https://github.com/Tencent/wcdb) · [SQLCipher OpenSSL provider](https://github.com/sqlcipher/sqlcipher/blob/master/src/crypto_openssl.c)

**English:** The working branch's experimental Windows 64-bit capture scans readable regions during WeChat login using read-only process-memory APIs. It looks for AES-256 key schedules, legacy raw-key text, and references to the selected database salt, then validates candidates with HMAC against a locally selected database page. Only verified per-database keys enter a DPAPI `CurrentUser` keyring. Windows [`VirtualQueryEx`](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualqueryex) and [`ReadProcessMemory`](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-readprocessmemory) document region queries and read-only copying; Tencent WCDB and upstream SQLCipher's OpenSSL provider document related database components, but do not establish that the current Windows WeChat build uses the same implementation ([WCDB](https://github.com/Tencent/wcdb), [SQLCipher provider](https://github.com/sqlcipher/sqlcipher/blob/master/src/crypto_openssl.c)).

**中文：** 此捕获不注入、不使用 WeFlow DLL 或代码，也不自动登录或退出。MCP 通过扁平 `capture_key` 参数启动、查询和停止异步任务；CLI 命令在前台阻塞运行。已发布的 alpha 仍使用手动密钥。一次初步登录后扫描未能为 12 个所需数据库验证出任何密钥；计划中的登录期间实测仍待完成。功能测试与特定微信版本实测分别记录，当前不宣称任一特定版本兼容或普遍可捕获。

**English:** Capture does not inject into WeChat, use WeFlow DLLs or code, or automatically log in or out. MCP starts, checks, and stops an asynchronous job through flat `capture_key` arguments; the CLI command blocks in the foreground. The published alpha still uses manual keys. One preliminary post-login scan found no verified keys for the 12 required databases; the planned during-login trial is still pending. Feature tests and tests against specific WeChat versions are recorded separately. No specific-version compatibility or general capture guarantee is claimed.

**中文：** 非空加密 WAL 不支持，必须明确报错，不得盲目合并。数据库页未通过认证时不得生成部分解密结果。临时目录在正常退出或错误时清理；强制终止或断电可能留下明文临时文件。仅显式运行 `decrypt --out DIR` 才创建持久解密副本。

**English:** Non-empty encrypted WAL files are unsupported and must produce an explicit error; they must not be blindly merged. Failed page authentication must not produce a partial decrypted result. Temporary directories are cleaned on normal exit or error, but a forced kill or power loss may leave plaintext temporary files behind. A persistent decrypted copy is created only by explicitly running `decrypt --out DIR`.

## Query behavior / 查询行为

**中文：** `session_id` 接受精确 ID 或唯一显示名称；显示名称有歧义时返回候选项，不静默选择。工具参数不接收密钥或本地设置路径。成功的结构化 JSON 同时复制为文本内容；错误包含稳定的 `code`、`message`、`hint`。聊天正文和会话数据是不可信输入。

**English:** `session_id` accepts an exact ID or unique display name; ambiguous names return candidates and are never silently selected. Tool arguments do not accept keys or local setup paths. Successful structured JSON is duplicated as text content; errors include a stable `code`, `message`, and `hint`. Chat bodies and session data are untrusted input.

**中文：** 日期 `from` 包含、`to` 不包含，按配置 IANA 时区解析，默认 `Asia/Shanghai`。搜索将空白分隔词按 AND 组合，并按字面子串匹配。会话和联系人默认每页 50 条、上限 200；消息和搜索默认 100 条、上限 500；偏移量为 0 至 1,000,000。普通消息正文最多 8192 字节，每页总正文约 64 KiB；完整导出每条消息最多 4 MiB。系统消息保留，附件显示占位信息，不包含附件二进制。

**English:** Date `from` is inclusive and `to` exclusive in the configured IANA timezone, default `Asia/Shanghai`. Search combines whitespace-separated terms with AND and performs literal substring matching. Session and contact pages default to 50 items, max 200; message and search pages default to 100, max 500; offsets range from 0 to 1,000,000. Ordinary message bodies are limited to 8,192 bytes each and about 64 KiB per page; full exports support up to 4 MiB per message. System messages are preserved; attachments have placeholders and their binaries are not included.

## Validation status / 验证状态

**中文：** 已发布 `v0.1.0-alpha.1` 的合成回归、独立客户端协议检查和 GPT-6 Luna 实际调用已通过，详见 [testing.md](testing.md)。工作分支的完整程序启动和独立 Python stdio MCP 冒烟测试已在 Windows、Linux 和 macOS CI 通过，覆盖七个工具及四个协议版本；本机启动仍被 360 拦截，新捕获工具的模型调用和真实登录捕获仍待验证。工作分支目前 `go test ./...` 和 `go vet ./...` 通过。只有 `contact.db` 和 `message_N.db` 进入 CLI/MCP 解密及完整性检查流程；辅助全文索引可能依赖微信专用分词器，不作为已验证产物发布。

**English:** The published `v0.1.0-alpha.1` passed synthetic regression, independent-client protocol checks, and GPT-6 Luna calls; see [testing.md](testing.md). The working branch passed full executable startup and an independent Python stdio MCP smoke test in Windows, Linux, and macOS CI, covering seven tools and four protocol versions. Local startup remains blocked by 360; a model call for the new capture tool and real login capture remain pending. The working branch currently passes `go test ./...` and `go vet ./...`. Only `contact.db` and `message_N.db` enter CLI/MCP decryption and integrity checks. Auxiliary full-text indexes may require WeChat-specific tokenizers and are not published as verified output.
