# Architecture / 架构

## Overview / 概览

**中文：** WxMori 是单二进制 Go CLI，MCP 服务通过 stdio 运行，使用官方 `github.com/modelcontextprotocol/go-sdk`，要求 Go 1.26.0。程序读取本地配置，为单个账号制作私有临时快照；需要时解密数据库，再将快照规范化为临时 SQLite 索引。查询和导出从该索引读取，以限制内存使用。

**English:** WxMori is a single-binary Go CLI. Its MCP server uses stdio and the official `github.com/modelcontextprotocol/go-sdk`; Go 1.26.0 is required. The program loads local configuration, creates a private temporary snapshot for one account, decrypts databases when needed, then normalizes the snapshot into a temporary SQLite index. Queries and exports read from this index to keep memory bounded.

## Sources and lifecycle / 数据源与生命周期

**中文：** 配置必须且只能设置 `data_dir` 或 `decrypted_dir`。前者指向单个微信账号的 `db_storage`；当前版本自动捕获并验证每库密钥后，可从 DPAPI 密钥环加载，密钥环位于配置文件旁 `.wxmori-keys/`。手动密钥仍可回退使用；已发布 `v0.1.0-alpha.1` 仅支持手动密钥。后者指向已有的解密 SQLite 数据目录，可在 Windows、Linux 和 macOS 使用。自动发现仅供 `init` 选择账号，账号不合并。程序以修改时间和大小检测源变化，在私有 SQLite 副本中回放 WAL、仅在该副本 checkpoint；源数据库不修改。

**English:** Configuration sets exactly one of `data_dir` or `decrypted_dir`. The former points to one WeChat account's `db_storage`; after automatic capture and verification, current versions load per-database keys from the DPAPI keyring beside the config under `.wxmori-keys/`. Manual key input remains a fallback; published `v0.1.0-alpha.1` supports manual keys only. The latter points to existing decrypted SQLite data and works on Windows, Linux, and macOS. `init` only helps select an account; accounts are not merged. Source changes are detected by modification time and size. WAL data is replayed in a private SQLite copy and checkpointed there only. Source databases are not modified.

## Experimental Windows capture / Windows 实验性捕获

**中文：** 工作分支的实验性 Windows 64 位捕获识别 AES-256 密钥调度、旧式原始密钥文本和所选数据库盐引用，并对所选数据库页执行 HMAC 验证；仅验证通过的每库密钥进入 DPAPI `CurrentUser` 密钥环。默认 `windows_login_breakpoint` 在支持的 Windows AMD64 上动态定位密钥设置函数，通过调试器硬件执行断点工作；短暂暂停线程，并临时设置、恢复调试寄存器后分离，不修改目标代码或注入 DLL。`windows_readonly_memory` 只通过 `VirtualQueryEx`/`ReadProcessMemory` 轮询可读内存。WCDB 和 SQLCipher 的 OpenSSL provider 资料说明相关组件，但不能证明当前微信构建使用相同实现。[Windows API](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualqueryex) · [Tencent WCDB](https://github.com/Tencent/wcdb) · [SQLCipher OpenSSL provider](https://github.com/sqlcipher/sqlcipher/blob/master/src/crypto_openssl.c)

**English:** The working branch's experimental Windows 64-bit capture recognizes AES-256 key schedules, legacy raw-key text, and references to the selected database salt, then validates candidates with HMAC against a selected database page. Only verified per-database keys enter a DPAPI `CurrentUser` keyring. The default `windows_login_breakpoint` dynamically locates the key-setup function on supported Windows AMD64 and uses debugger hardware execution breakpoints; it briefly pauses threads and temporarily sets then restores debug registers before detaching, without patching target code or injecting a DLL. `windows_readonly_memory` polls readable memory using `VirtualQueryEx`/`ReadProcessMemory` only. WCDB and SQLCipher's OpenSSL provider document related components, but do not establish that the current WeChat build uses the same implementation ([Windows API](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualqueryex), [ReadProcessMemory](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-readprocessmemory), [WCDB](https://github.com/Tencent/wcdb), [SQLCipher provider](https://github.com/sqlcipher/sqlcipher/blob/master/src/crypto_openssl.c)).

**中文：** 此捕获不使用 WeFlow DLL 或代码，也不自动登录或退出。MCP 通过扁平 `capture_key` 参数启动、查询和停止异步任务；调用者须先轮询 `ready_for_login` 为真再请用户重新登录。CLI 命令在前台阻塞运行。一次针对 Weixin 4.1.15.56 的单账号 Luna/Python stdio 试验捕获并 HMAC 验证 12/12 核心数据库密钥；新进程在无手动主密钥时从 DPAPI 重载。12 个 WAL 文件各有旧代次尾帧被排除，当前代次已提交帧为 0；故完整读取/导出通过，但提交帧回放仍仅由合成测试覆盖。真实快照源文件大小/修改时间指纹稳定；源哈希未在该实测中验证。此前只读轮询扫描曾为 0/12。自动捕获兼容性仅限该样本，其他版本与宿主未实测。

**English:** Capture uses no WeFlow DLLs or code and does not automatically log in or out. MCP starts, checks, and stops an asynchronous job through flat `capture_key` arguments; callers must poll until `ready_for_login` is true before asking the user to log in again. The CLI command blocks in the foreground. One Luna/Python stdio trial on Weixin 4.1.15.56 and one account captured and HMAC-verified keys for 12/12 core databases; a fresh process reloaded them from DPAPI with no manual master key. Each of the 12 WAL files had an excluded stale-generation tail, and there were zero current-generation committed frames. Reads and exports passed, but committed-frame replay remains covered only by synthetic tests. Real-snapshot source size/mtime fingerprints remained stable; source hashes were not verified in that trial. An earlier read-only polling scan was 0/12. Capture compatibility is established only for this sample; other versions and hosts remain untested.

**中文：** 程序验证加密 WAL 头与帧校验和、盐代次、每页 HMAC 和提交边界，仅在私有副本回放有效已提交帧；未提交帧和旧代次帧不纳入结果。同代次校验/HMAC 失败或截断会拒绝处理，不生成部分解密结果，源文件不变。已发布 alpha1 仍不支持非空加密 WAL。临时目录在正常退出或错误时清理；强制终止或断电可能留下明文临时文件。仅显式运行 `decrypt --out DIR` 才创建持久解密副本。

**English:** The program validates encrypted-WAL header/frame checksums, salt generations, per-page HMACs, and commit boundaries, replaying only valid committed frames in a private copy; uncommitted and stale-generation frames are excluded. A same-generation checksum/HMAC failure or truncation is rejected without partial decrypted output, and source files remain unchanged. Published alpha1 still does not support non-empty encrypted WAL. Temporary directories are cleaned on normal exit or error, but a forced kill or power loss may leave plaintext temporary files behind. A persistent decrypted copy is created only by explicitly running `decrypt --out DIR`.

## Query behavior / 查询行为

**中文：** `session_id` 接受精确 ID 或唯一显示名称；显示名称有歧义时返回候选项，不静默选择。工具参数不接收密钥或本地设置路径。成功的结构化 JSON 同时复制为文本内容；错误包含稳定的 `code`、`message`、`hint`。聊天正文和会话数据是不可信输入。

**English:** `session_id` accepts an exact ID or unique display name; ambiguous names return candidates and are never silently selected. Tool arguments do not accept keys or local setup paths. Successful structured JSON is duplicated as text content; errors include a stable `code`, `message`, and `hint`. Chat bodies and session data are untrusted input.

**中文：** 日期 `from` 包含、`to` 不包含，按配置 IANA 时区解析，默认 `Asia/Shanghai`。搜索将空白分隔词按 AND 组合，并按字面子串匹配。会话和联系人默认每页 50 条、上限 200；消息和搜索默认 100 条、上限 500；偏移量为 0 至 1,000,000。普通消息正文最多 8192 字节，每页总正文约 64 KiB；完整导出每条消息最多 4 MiB。系统消息保留，附件显示占位信息，不包含附件二进制。

**English:** Date `from` is inclusive and `to` exclusive in the configured IANA timezone, default `Asia/Shanghai`. Search combines whitespace-separated terms with AND and performs literal substring matching. Session and contact pages default to 50 items, max 200; message and search pages default to 100, max 500; offsets range from 0 to 1,000,000. Ordinary message bodies are limited to 8,192 bytes each and about 64 KiB per page; full exports support up to 4 MiB per message. System messages are preserved; attachments have placeholders and their binaries are not included.

## Validation status / 验证状态

**中文：** 已发布 `v0.1.0-alpha.1` 的合成回归、独立客户端协议检查和 GPT-6 Luna 实际调用已通过，详见 [testing.md](testing.md)。此前提交的三系统 CI 已通过完整启动与独立 Python stdio 冒烟，但早于当前调试器/WAL 代码；当前发布 CI 待完成。本机 Go 测试与静态检查通过。Luna/Python stdio 单账号试验捕获、重载密钥，并完成保留 WAL 的 12 库读取和 TXT/JSONL 导出。当前 WAL 无提交帧，因此已提交帧回放仍仅由合成测试覆盖；该结果不代表其他版本或宿主兼容。只有 `contact.db` 和 `message_N.db` 进入 CLI/MCP 解密及完整性检查流程；辅助全文索引可能依赖微信专用分词器，不作为已验证产物发布。

**English:** Published `v0.1.0-alpha.1` passed synthetic regression, independent-client protocol checks, and GPT-6 Luna calls; see [testing.md](testing.md). Earlier three-system CI passed executable startup and independent Python stdio smoke tests, but predates the current debugger/WAL code; current release CI is pending. Local Go tests and static checks pass. A Luna/Python stdio single-account trial captured and reloaded keys, then completed reads of all 12 databases and TXT/JSONL exports with retained WALs. Those WALs had no committed frames, so committed-frame replay is covered by synthetic tests only; this does not establish compatibility with other versions or hosts. Only `contact.db` and `message_N.db` enter CLI/MCP decryption and integrity checks. Auxiliary full-text indexes may require WeChat-specific tokenizers and are not published as verified output.
