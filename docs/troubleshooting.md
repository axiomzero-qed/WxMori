# Troubleshooting / 故障排查

> **中文：** 下列 `accounts`、`--force`、身份检查和批量导出步骤适用于 `v0.2.0-alpha.2`；alpha1 下载包不包含这些接口。
>
> **English:** The `accounts`, `--force`, identity-inspection, and batch-export steps below apply to `v0.2.0-alpha.2`; they are not included in the alpha1 download.

## Choosing the account / 选择账号

**中文：** 运行 `wxmori accounts`，核对目录 ID、候选内部 ID、数据库数量/大小、最近文件变化时间及密钥环可用性；目录名可能不等于自定义微信号，文件变化也不证明当前登录或历史已同步。用 `init --data-dir` 明确选择目录。`whoami` 或 `status --inspect` 可建立已验证快照；仅当 contact 中的 username 与目录推导 ID 完全相等时身份才标记为已验证，否则显示未知，不按昵称猜测。

**English:** Run `wxmori accounts` and review the directory ID, candidate internal ID, database counts/sizes, last file-change time, and keyring availability. A directory label may differ from the custom WeChat ID, and file changes do not prove current login or history sync. Select the directory explicitly with `init --data-dir`. `whoami` or `status --inspect` builds a verified snapshot; identity is verified only when the contact username exactly matches the directory-derived ID. Otherwise it is unknown; no nickname guess is made.

## Capture stays at zero verified keys / 捕获一直没有已验证密钥

**中文：** 确认所选目录正确，并在 `ready_for_login: true` 后亲自在本机退出并重新登录。`key_setup_events` 仅表示观察到密钥设置相关事件，不证明发生登录；`candidate_count` 是验证尝试数。未验证 AES 候选或 0 个已验证密钥本身都不能证明账号配错。只有工具报告 `KEY_ACCOUNT_MISMATCH` 时，才表示已用另一个本地账号的 contact/message 首页、两种不同盐的 HMAC 证据确认错配；工具会停止并分离，不保存该账号密钥、不自动切换。未出现此证据时按未验证捕获问题处理，避免反复登录。

**English:** Confirm the selected directory, then log out and back in locally only after `ready_for_login: true`. `key_setup_events` means a key-setup-related event was observed, not that a login occurred; `candidate_count` counts verification attempts. Unverified AES candidates or zero verified keys do not by themselves prove the account is wrong. `KEY_ACCOUNT_MISMATCH` is emitted only when HMAC evidence from another local account's contact/message first pages validates under two distinct salts; the tool then stops and detaches without saving that account's keys or switching automatically. Without this evidence, treat the result as an unverified capture issue and avoid repeated logins.

## `init` and existing config / `init` 与已存在配置

**中文：** `init` 只读取目录元数据并写配置，不打开 SQLite，因此不需要关闭正在使用数据库的微信。若目标配置已经存在，程序返回 `CONFIG_EXISTS`；检查目标文件后，只有确实要替换时才显式加 `--force`，该操作会原子替换配置。若 MCP `serve` 正在运行，替换后需重启服务以重新载入配置；不会自动切换账号。不要把通用旧版 `OPERATION_FAILED` 直接解释为 SQLite 锁；确认具体错误码和原因。测试报告中的锁冲突只是猜测，合成复现发现的原因是重复使用配置目标。

**English:** `init` reads directory metadata and writes the config; it does not open SQLite, so WeChat need not be closed to avoid a database lock. If the config target already exists, the command returns `CONFIG_EXISTS`. Inspect the file and add `--force` only when replacement is intended; replacement is atomic. If MCP `serve` is running, restart it after replacement to reload the config; accounts are never switched automatically. Do not interpret a generic `OPERATION_FAILED` from an older build as proof of a SQLite lock. Check the actual code and cause. The reported lock explanation was a hypothesis; a synthetic reproduction instead found a reused config destination.

## Little local history / 本地历史记录较少

**中文：** 当全局会话数 ≤3 且消息数 ≤10 时，`whoami`/inspect 会返回 `SPARSE_LOCAL_HISTORY`。消息库小于 1 MiB 只是独立的文件大小提示，既不证明消息缺失，也不证明历史未同步。工具只说明本机当前数据量有限；请先在微信客户端检查本地记录和同步状态，再决定是否导出。

**English:** `whoami`/inspect reports `SPARSE_LOCAL_HISTORY` when global counts are at most 3 sessions and at most 10 messages. A message-store file under 1 MiB is a separate size hint; it proves neither missing messages nor unsynced history. The tool only reports limited data currently present on this device. Check records and sync status in the WeChat client before deciding what to export.

## Progress and batch export / 进度与批量导出

**中文：** CLI 捕获默认 300 秒、上限 600 秒。`--progress human` 在 stderr 更新单行，最终 JSON 写到 stdout；`--progress json` 在状态变化或每 5 秒输出，最后 30 秒提醒超时。批量导出可用 `export --all-sessions` 或 `--all`；文件写入 `chat-<12位ID哈希>-<清理后的名称>` 子目录，每批以原子方式发布，并生成含路径、条数和 SHA-256 的清单。MCP 的 `export_all_chats` 预览最多 50 个文件。

**English:** CLI capture defaults to 300 seconds and is capped at 600. `--progress human` updates one stderr line and prints final JSON to stdout; `--progress json` prints on state changes or every five seconds and warns during the final 30 seconds. For batch export, use `export --all-sessions` or `--all`; outputs go under `chat-<12-hex-ID-hash>-<sanitized-name>` directories, publish each batch atomically, and include a manifest of paths, counts, and SHA-256 hashes. MCP `export_all_chats` previews at most 50 files.
