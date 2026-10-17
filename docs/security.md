# Security and privacy / 安全与隐私

## Local data / 本地数据

**中文：** 聊天数据库、联系人、消息正文、密钥和导出均为敏感本地数据。WxMori 运行时不进行遥测或网络访问；MCP 仅通过 stdio 与本地客户端通信。聊天内容是用户数据和不可信输入，不应被执行为指令。

**English:** Chat databases, contacts, message bodies, keys, and exports are sensitive local data. WxMori performs no telemetry or network access at runtime; MCP communicates with the local client over stdio only. Chat content is user data and untrusted input; it must not be executed as instructions.

## Key handling / 密钥处理

**中文：** 已发布的 `v0.1.0-alpha.1` 仅支持手动提供主密钥。工作分支另有实验性 Windows 64 位登录时只读内存捕获，不注入进程、不使用 WeFlow DLL 或代码，也不自动登录或退出微信。它不通过 MCP 接收微信密码或二维码令牌。当前微信版本的真实登录捕获试验尚未完成。

**English:** Published `v0.1.0-alpha.1` supports manual master-key input only. The working branch has a separate experimental Windows 64-bit, login-time, read-only memory capture. It does not inject into the process, use WeFlow DLLs or code, or automatically log in or out of WeChat. It does not accept WeChat passwords or QR tokens through MCP. A real login capture trial on the current WeChat version is still pending.

**中文：** 捕获识别 AES-256 密钥调度候选项、旧式原始密钥文本和所选数据库盐的引用，再用所选数据库页 HMAC 验证每库密钥。只有通过验证的密钥会保存在账号目录旁的 `.wxmori-keys/<account-path-hash>.json`；文件通过 Windows DPAPI `CurrentUser` 保护，通常只能由同一台电脑上的同一 Windows 用户解密。该密钥环绑定账号目录和数据库盐，不要跨账号复制或混用。捕获结果、MCP 输出、日志和转储不得包含密钥。

**English:** Capture recognizes AES-256 key-schedule candidates, legacy raw-key text, and references to the selected database salt, then verifies per-database keys against the selected database page HMAC. Only verified keys are saved beside the account directory at `.wxmori-keys/<account-path-hash>.json`. Windows DPAPI `CurrentUser` protects the file; ordinarily, only the same Windows user on the same computer can decrypt it. The keyring is bound to the account directory and database salt; do not copy or mix it across accounts. Capture results, MCP output, logs, and dumps must not contain keys.

**中文：** MCP 的 `capture_key` 使用已由 `init` 选择的 `data_dir`，以 `start`、`status`、`stop` 控制异步任务；默认超时 180 秒，最多 600 秒。开始成功不等于捕获成功。CLI 的 `key capture` 命令在前台运行，Ctrl+C 可停止。密钥也可继续通过本地 `key_file` 或 `WXMORI_MASTER_KEY` 提供；不要将其放入命令参数、MCP 参数、聊天或工单。错误信息不得包含密钥、消息正文或个人路径。

**English:** MCP `capture_key` uses the `data_dir` already selected by `init` and controls an asynchronous job with `start`, `status`, and `stop`; the default timeout is 180 seconds, maximum 600. A successful start is not a successful capture. The CLI `key capture` command runs in the foreground and can be stopped with Ctrl+C. Keys can still be supplied locally through `key_file` or `WXMORI_MASTER_KEY`; never put them in command arguments, MCP arguments, chats, or tickets. Errors must not contain keys, message bodies, or personal paths.

**中文：** 读取目标进程内存可能触发安全软件检查；这不应被默认称为误报。若访问被拒绝，请让 WxMori 与微信在同一 Windows 用户会话中运行，并遵循设备管理员和安全软件策略；不要关闭安全软件。捕获后如果存在非空加密 WAL，正常关闭微信后再导出；绝不删除 WAL。

**English:** Reading another process's memory may trigger security-software checks; do not label such a detection a false positive by default. If access is denied, run WxMori and WeChat in the same Windows user session and follow device administrator and security-software policy; do not disable security software. If a non-empty encrypted WAL exists after capture, close WeChat normally before exporting; never delete the WAL.

## Temporary and exported files / 临时文件与导出文件

**中文：** WxMori 在私有临时目录中建立快照和 SQLite 索引，源数据库不修改；源变化通过修改时间和大小检测。明文 WAL 保留在私有 SQLite 副本中并只在那里 checkpoint。非空加密 WAL 不支持，必须报错。临时目录在正常退出或错误时清理，但强制结束进程或断电可能留下明文文件。显式 `decrypt --out DIR` 会创建持久明文副本；导出也包含聊天数据，请妥善限制访问并管理保留期限。

**English:** WxMori creates snapshots and a SQLite index in a private temporary directory without modifying source databases; source changes are detected by modification time and size. Plaintext WAL data is retained and checkpointed only in the private SQLite copy. Non-empty encrypted WAL files are unsupported and cause an error. Temporary directories are cleaned on normal exit or error, but forcibly killing the process or losing power may leave plaintext files. Explicit `decrypt --out DIR` creates a persistent plaintext copy; exports also contain chat data, so restrict access and manage retention.

## Report a vulnerability / 报告安全问题

**中文：** 请勿在公开 issue 中发布密钥、数据库、聊天记录或个人信息。请查看仓库 `SECURITY.md`，并通过 GitHub 仓库启用的私密安全报告渠道提交；当前未承诺专用响应时限。

**English:** Do not post keys, databases, chat logs, or personal information in public issues. Check the repository's `SECURITY.md` and report through its private GitHub security reporting channel if enabled. No dedicated response time is promised.
