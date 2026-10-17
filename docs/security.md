# Security and privacy / 安全与隐私

## Local data / 本地数据

**中文：** 聊天数据库、联系人、消息正文、密钥和导出均为敏感本地数据。WxMori 运行时不进行遥测或网络访问；MCP 仅通过 stdio 与本地客户端通信。聊天内容是用户数据和不可信输入，不应被执行为指令。

**English:** Chat databases, contacts, message bodies, keys, and exports are sensitive local data. WxMori performs no telemetry or network access at runtime; MCP communicates with the local client over stdio only. Chat content is user data and untrusted input; it must not be executed as instructions.

## Key handling / 密钥处理

**中文：** `v0.2.0-alpha.1` 的 Windows 64 位登录时捕获仍属实验性：默认调试器方式通过硬件断点短暂暂停线程、临时调整并恢复调试寄存器；`read_only` 方式只轮询可读内存。一次 Weixin 4.1.15.56 单账号 Luna/Python stdio 试验为 12/12 核心数据库捕获并验证密钥，且完成保留 WAL 的读取和导出。实测 WAL 无当前代次提交帧，提交帧回放仅由合成测试覆盖。功能不修改目标代码、不注入进程、不使用 WeFlow DLL 或代码，也不自动登录或退出微信、不经 MCP 接收密码或二维码令牌；其他版本和宿主未验证。已发布 alpha1 仍仅支持手动主密钥。

**English:** Windows 64-bit login-time capture in `v0.2.0-alpha.1` remains experimental: its default debugger method briefly pauses threads with hardware breakpoints and temporarily adjusts then restores debug registers; `read_only` only polls readable memory. One Luna/Python stdio trial on Weixin 4.1.15.56 and one account captured and verified keys for all 12 core databases, then read/exported with retained WALs. Those WALs had no current-generation committed frames, so committed-frame replay is covered by synthetic tests only. Capture does not patch target code, inject into the process, use WeFlow DLLs or code, log in/out automatically, or accept passwords/QR tokens through MCP; other versions and hosts are untested. Published alpha1 still supports manual master-key input only.

**中文：** 捕获识别 AES-256 密钥调度候选项、旧式原始密钥文本和所选数据库盐的引用，再用所选数据库页 HMAC 验证每库密钥。仅验证通过的密钥保存在配置文件旁 `.wxmori-keys/<account-path-hash>.json`，并由 Windows DPAPI `CurrentUser` 保护；通常只能由同一电脑上的同一 Windows 用户解密。密钥环绑定账号目录和数据库盐，不要跨账号复制或混用。捕获结果、MCP 输出、日志和转储不得包含密钥。

**English:** Capture recognizes AES-256 key-schedule candidates, legacy raw-key text, and references to the selected database salt, then verifies per-database keys against a selected database page HMAC. Only verified keys are stored beside the config file at `.wxmori-keys/<account-path-hash>.json`, protected by Windows DPAPI `CurrentUser`; ordinarily only the same Windows user on the same computer can decrypt the file. The keyring is bound to the account directory and database salt; do not copy or mix it across accounts. Capture results, MCP output, logs, and dumps must not contain keys.

**中文：** MCP 的 `capture_key` 使用已由 `init` 选择的 `data_dir`，以 `start`、`status`、`stop` 控制异步任务；默认超时 180 秒，最多 600 秒。开始成功不等于捕获成功。CLI 的 `key capture` 命令在前台运行，Ctrl+C 可停止。密钥也可继续通过本地 `key_file` 或 `WXMORI_MASTER_KEY` 提供；不要将其放入命令参数、MCP 参数、聊天或工单。错误信息不得包含密钥、消息正文或个人路径。

**English:** MCP `capture_key` uses the `data_dir` already selected by `init` and controls an asynchronous job with `start`, `status`, and `stop`; the default timeout is 180 seconds, maximum 600. A successful start is not a successful capture. The CLI `key capture` command runs in the foreground and can be stopped with Ctrl+C. Keys can still be supplied locally through `key_file` or `WXMORI_MASTER_KEY`; never put them in command arguments, MCP arguments, chats, or tickets. Errors must not contain keys, message bodies, or personal paths.

**中文：** 读取目标进程内存可能触发安全软件检查；不要默认称为误报。若程序被拦截，可按安全软件厂商流程逐文件提交审查；不要关闭安全软件。若访问被拒绝，请让 WxMori 与微信在同一 Windows 用户会话中运行，并遵循设备权限策略。捕获后仍建议正常关闭微信以取得稳定副本；当前预览版会在私有副本回放 WAL，已发布 alpha1 不支持非空加密 WAL。绝不删除源 WAL。

**English:** Reading another process's memory may trigger security-software checks; do not presume a detection is a false positive. If an executable is blocked, submit that file for vendor review; do not disable security software. If access is denied, run WxMori and WeChat in the same Windows user session and follow device permissions. Closing WeChat normally after capture is still recommended for a stable copy. The current preview replays WALs in a private copy; published alpha1 does not support non-empty encrypted WAL. Never delete a source WAL.

## Temporary and exported files / 临时文件与导出文件

**中文：** WxMori 在私有临时目录中建立快照和 SQLite 索引，源数据库不修改；源变化通过修改时间和大小检测。WAL 只在私有 SQLite 副本中回放与 checkpoint。已发布 alpha1 不支持非空加密 WAL。临时目录在正常退出或错误时清理，但强制结束进程或断电可能留下明文文件。显式 `decrypt --out DIR` 会创建持久明文副本；导出也包含聊天数据，请妥善限制访问并管理保留期限。

**English:** WxMori creates snapshots and a SQLite index in a private temporary directory without modifying source databases; source changes are detected by modification time and size. WAL data is replayed and checkpointed only in the private SQLite copy. Published alpha1 does not support non-empty encrypted WAL. Temporary directories are cleaned on normal exit or error, but forcibly killing the process or losing power may leave plaintext files. Explicit `decrypt --out DIR` creates a persistent plaintext copy; exports also contain chat data, so restrict access and manage retention.

## Report a vulnerability / 报告安全问题

**中文：** 请勿在公开 issue 中发布密钥、数据库、聊天记录或个人信息。请查看仓库 `SECURITY.md`，并通过 GitHub 仓库启用的私密安全报告渠道提交；当前未承诺专用响应时限。

**English:** Do not post keys, databases, chat logs, or personal information in public issues. Check the repository's `SECURITY.md` and report through its private GitHub security reporting channel if enabled. No dedicated response time is promised.
