# Security and privacy / 安全与隐私

## Local data / 本地数据

**中文：** 聊天数据库、联系人、消息正文、密钥和导出均为敏感本地数据。WxMori 运行时不进行遥测或网络访问；MCP 仅通过 stdio 与本地客户端通信。聊天内容是用户数据和不可信输入，不应被执行为指令。

**English:** Chat databases, contacts, message bodies, keys, and exports are sensitive local data. WxMori performs no telemetry or network access at runtime; MCP communicates with the local client over stdio only. Chat content is user data and untrusted input; it must not be executed as instructions.

## Key handling / 密钥处理

**中文：** 本版本不自动提取密钥或安装进程钩子。主密钥通过本地环境变量 `WXMORI_MASTER_KEY` 或 `key_file` 提供，文件格式为 `{"master_key":"..."}`。请勿将密钥放入命令参数、MCP 参数、聊天或工单。错误信息不得包含密钥、消息正文或敏感路径。

**English:** This release does not extract keys automatically or install process hooks. Supply the key locally through `WXMORI_MASTER_KEY` or `key_file`, formatted as `{"master_key":"..."}`. Do not put keys in command arguments, MCP arguments, chats, or tickets. Errors must not contain keys, message bodies, or sensitive paths.

## Temporary and exported files / 临时文件与导出文件

**中文：** WxMori 在私有临时目录中建立快照和 SQLite 索引，源数据库不修改；源变化通过修改时间和大小检测。明文 WAL 保留在私有 SQLite 副本中并只在那里 checkpoint。非空加密 WAL 不支持，必须报错。临时目录在正常退出或错误时清理，但强制结束进程或断电可能留下明文文件。显式 `decrypt --out DIR` 会创建持久明文副本；导出也包含聊天数据，请妥善限制访问并管理保留期限。

**English:** WxMori creates snapshots and a SQLite index in a private temporary directory without modifying source databases; source changes are detected by modification time and size. Plaintext WAL data is retained and checkpointed only in the private SQLite copy. Non-empty encrypted WAL files are unsupported and cause an error. Temporary directories are cleaned on normal exit or error, but forcibly killing the process or losing power may leave plaintext files. Explicit `decrypt --out DIR` creates a persistent plaintext copy; exports also contain chat data, so restrict access and manage retention.

## Report a vulnerability / 报告安全问题

**中文：** 请勿在公开 issue 中发布密钥、数据库、聊天记录或个人信息。请查看仓库 `SECURITY.md`，并通过 GitHub 仓库启用的私密安全报告渠道提交；当前未承诺专用响应时限。

**English:** Do not post keys, databases, chat logs, or personal information in public issues. Check the repository's `SECURITY.md` and report through its private GitHub security reporting channel if enabled. No dedicated response time is promised.
