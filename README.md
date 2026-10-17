# WxMori / 微忆

## About / 项目简介

**中文：** WxMori（微忆）是独立的 Go 命令行工具和本地 MCP 服务，用于读取微信 4.x 聊天数据库。当前预览版为 `v0.2.0-alpha.2`，通过官方 `github.com/modelcontextprotocol/go-sdk` 使用 stdio 通信，采用 MIT 许可证。仅从源码构建需要 Go 1.26 或更新版本。

**English:** WxMori is an independent Go CLI and local MCP server for reading WeChat 4.x chat databases. The current preview release is `v0.2.0-alpha.2`; it uses the official `github.com/modelcontextprotocol/go-sdk` over stdio, under the MIT license. Go 1.26 or later is needed only to build from source.

**中文：** `v0.2.0-alpha.2` 提供九个 MCP 工具、Windows AMD64 登录时实验性自动取密钥、DPAPI 密钥环、加密 WAL 私有副本回放和按会话批量导出。真实捕获仅在 alpha1 的 Weixin 4.1.15.56 单账号样本中验证 12/12 个核心数据库；alpha2 真实试用只检查状态、身份和已有密钥，未捕获或重新登录。该样本当前代次 WAL 提交帧为 0，真实提交帧回放尚无实测覆盖。代码 CI 见[测试记录](docs/testing.md)，alpha1 历史见[发布历史](CHANGELOG.md)。

**English:** `v0.2.0-alpha.2` provides nine MCP tools, experimental Windows AMD64 login-time key capture, a DPAPI keyring, encrypted-WAL replay in private copies, and per-session batch export. Real capture was verified only in the alpha1 trial on one Weixin 4.1.15.56 account, with keys validated for 12/12 core databases. The alpha2 live trial checked status, identity, and existing keys only; it did not capture or re-login. The sample had zero current-generation WAL commits, so real committed-frame replay remains untested. See [testing notes](docs/testing.md) and the alpha1 [release history](CHANGELOG.md).

**中文：** 登录时自动取密钥仅支持 Windows AMD64，仍属实验功能；不使用 WeFlow DLL 或代码。默认 `auto` 使用临时硬件执行断点，`read_only` 只轮询可读内存；不注入 DLL 或修改目标代码。捕获仅在 alpha1 单账号样本实测，详见 [测试记录](docs/testing.md) 与 [alpha2 发布说明](docs/releases/v0.2.0-alpha.2.md)。

**English:** Login-time key capture is experimental and supported only on Windows AMD64; no WeFlow DLL or code is used. Default `auto` uses temporary hardware execution breakpoints; `read_only` polls readable memory only. The tool does not inject a DLL or patch target code. Capture was tested in the alpha1 single-account trial; see [testing](docs/testing.md) and the [alpha2 preview notes](docs/releases/v0.2.0-alpha.2.md).

## Current preview / 当前预览版

**中文：** `v0.2.0-alpha.2` 增加本地账号目录清单、密钥环元数据、显式 `init --force`、已验证身份检查、稀疏本地历史提示、全会话导出和捕获诊断。MCP 共九个工具，包括 `list_accounts` 与 `export_all_chats`；CLI 支持 `--version`/`-v` 及 `export --all-sessions`/`--all`。本机 Go 测试/vet、Luna 四协议/九工具合成冒烟及一次仅检查状态/身份的真实 MCP 试用通过。提交 `f174842` 的三系统 CI 通过完整程序启动、Linux race 检查及独立四协议/九工具 stdio 冒烟（[运行记录](https://github.com/axiomzero-qed/WxMori/actions/runs/39072189240)）。当前 Luna 实测未运行新捕获、重新登录或导出；真实 WAL 提交帧回放仍缺少样本验证。详见 [alpha2 发布说明](docs/releases/v0.2.0-alpha.2.md)。
**English:** `v0.2.0-alpha.2` adds local account inventory, keyring metadata, explicit `init --force`, verified identity inspection, sparse-history hints, batch export, and capture diagnostics. Its MCP server has nine tools, including `list_accounts` and `export_all_chats`; the CLI supports `--version`/`-v` and `export --all-sessions`/`--all`. Local Go tests/vet, Luna synthetic smoke across four protocols and nine tools, and a live MCP trial limited to status/identity checks passed. The three-system CI run for commit `f174842` passed full executable startup, Linux race checks, and independent stdio smoke across four protocols/nine tools ([run](https://github.com/axiomzero-qed/WxMori/actions/runs/39072189240)). The current Luna trial did not run new capture, re-login, or export; real committed-WAL replay remains untested. See the [alpha2 release notes](docs/releases/v0.2.0-alpha.2.md).


## Install / 安装

**中文：** 从 [Releases](https://github.com/axiomzero-qed/WxMori/releases) 下载对应系统的压缩包，解压并核对 SHA-256。Windows 本地命令可将 `wxmori` 替换为 `.\wxmori.exe`，或把程序目录加入 PATH。MCP 的 `command` 使用程序完整路径。运行不需要模型账户或 API 密钥。源码构建与检查：

**English:** Download the archive for your system from [Releases](https://github.com/axiomzero-qed/WxMori/releases), extract it, and check its SHA-256. On Windows, use `.\wxmori.exe` in place of `wxmori`, or add its directory to PATH. Use the executable's full path as the MCP command. No model account or API key is required. Source build and checks:

```sh
go build -trimpath -o bin/wxmori.exe ./cmd/wxmori
go test ./...
python scripts/smoke_test.py --command bin/wxmori.exe
```

## Synthetic demo / 合成数据示例

**中文：** 此示例创建虚构消息和配置，无需微信数据。运行后使用打印出的配置路径执行后续命令。

**English:** This demo creates fictional messages and a configuration without needing WeChat data. Use the printed config path for the commands that follow.

```sh
wxmori demo --dir ./demo-data
wxmori --config ./demo-data/config.json sessions --keyword 微忆
wxmori --config ./demo-data/config.json export --session wxmori-demo@chatroom --months 2026-09
```

## Real data setup / 真实数据配置

**中文：** `wxmori init` 只读取本地目录元数据并写入配置，不打开 SQLite 数据库。`data_dir` 指向所选账号的 `db_storage`；已有解密 SQLite 数据则设置 `decrypted_dir`。两者必须且只能设置一个。相对路径以配置文件目录为基准；配置默认位于用户配置目录的 `wxmori/config.json`，也可用 `--config PATH` 或 `WXMORI_CONFIG` 指定。

**English:** `wxmori init` reads local directory metadata and writes configuration; it does not open SQLite databases. `data_dir` points to the selected account's `db_storage`; use `decrypted_dir` for existing decrypted SQLite data. Set exactly one of these fields. Relative paths are resolved from the config directory. The default config is under the user config directory at `wxmori/config.json`; use `--config PATH` or `WXMORI_CONFIG` to select another.

**中文：** 以下接口已在 alpha2 中提供：`wxmori accounts [--root xwechat_files]` 仅列本地目录元数据；多账号时据此选择目录，再用 `--data-dir` 初始化。配置已存在时返回 `CONFIG_EXISTS`；显式 `init --force` 才原子替换配置。若 MCP `serve` 已运行，替换后必须重启服务以重新载入配置，不会自动切换账号。捕获或解密后可用 `wxmori key list` 查看已保护密钥环中的账号绑定、标签、捕获时间和密钥数量；该命令绝不显示密钥值。`wxmori whoami` 或 `status --inspect` 会建立已验证快照；默认 `status` 仅快速检查目录和密钥可用性。本人身份只有在 `contact.username` 与目录推导 ID 完全一致时才标记为已验证，否则为未知，不按昵称猜测。目录名和文件变化均不证明登录身份或历史已同步。

**English:** The following interfaces are available in alpha2: `wxmori accounts [--root xwechat_files]` lists local account-directory metadata only; use it to select a directory before initializing with `--data-dir`. An existing config returns `CONFIG_EXISTS`; only explicit `init --force` replaces it atomically. If MCP `serve` is running, restart it after replacement to reload config; it never switches accounts automatically. After capture or decryption, `wxmori key list` shows the protected keyring's account binding, label, capture time, and key count; it never prints key values. `wxmori whoami` or `status --inspect` builds a verified snapshot, while default `status` checks directory/key readiness cheaply. Self-identity is marked verified only when `contact.username` exactly matches the directory-derived ID; otherwise it remains unknown and is never guessed from a nickname. Directory labels and file changes do not prove login identity or history sync.

**中文：** 当前版本自动捕获并验证每库密钥后，会将 DPAPI `CurrentUser` 保护的密钥环保存在配置文件旁的 `.wxmori-keys/`；新进程可自动加载，无需修改配置。手动兼容回退仍支持 `WXMORI_MASTER_KEY` 或 `key_file`（JSON：`{"master_key":"64 位十六进制字符"}`）。请勿将密钥放进命令参数、MCP 参数或聊天。最小配置示例：

**English:** After capture verifies per-database keys, the current version stores a DPAPI `CurrentUser`-protected keyring beside the config file under `.wxmori-keys/`; new processes load it automatically without a config edit. Manual compatibility fallback remains available through `WXMORI_MASTER_KEY` or `key_file` (JSON: `{"master_key":"64 hexadecimal characters"}`). Never put keys in command arguments, MCP arguments, or chats. Minimal config:

```json
{
  "data_dir": "C:/Users/you/Documents/xwechat_files/account-id/db_storage",
  "output_dir": "C:/Users/you/Documents/wxmori-exports",
  "timezone": "Asia/Shanghai"
}
```

常用命令 / Common commands:

```sh
wxmori demo --dir ./demo-data
wxmori init
wxmori --config ./demo-data/config.json status
wxmori --config ./demo-data/config.json serve
wxmori sessions --keyword demo
wxmori export --session demo --months 2026-09
wxmori decrypt --out ./decrypted-copy
```

**中文：** CLI 捕获命令为 `wxmori --config PATH key capture --timeout 300 --mode auto`，默认 300 秒、上限 600 秒；`--progress human` 默认在 stderr 单行更新进度，最终 JSON 写到 stdout。`--progress json` 在状态变化或每 5 秒输出 JSON，最后 30 秒会提醒即将超时。Ctrl+C 停止；`--mode read_only` 选择只读轮询。MCP 的 `capture_key` 支持 `start`、`status`、`stop` 和可选 `mode: auto|read_only`。状态中的 `ready_for_login` 为真后，才请用户在本机重新登录；启动成功不代表捕获成功。详情见 [MCP 集成](docs/mcp.md) 和 [安全与隐私](docs/security.md)。

**English:** Start CLI capture with `wxmori --config PATH key capture --timeout 300 --mode auto`; timeout defaults to 300 seconds and is capped at 600. `--progress human` is the default, updating one stderr line and printing final JSON to stdout. `--progress json` prints on state changes or every five seconds, with a final-30-second warning. Ctrl+C stops it. Use `--mode read_only` for polling only. MCP `capture_key` supports `start`, `status`, `stop`, and optional `mode: auto|read_only`. Ask the user to log in locally only after `ready_for_login` becomes true; a successful start does not mean capture succeeded. See [MCP integration](docs/mcp.md) and [security and privacy](docs/security.md).

**中文：** `decrypt` 只将聊天所需的 `contact.db` 和 `message_N.db` 保存到新明文目录，辅助全文索引和媒体库不在本版范围内。读取时使用私有临时副本和 SQLite 索引，不修改源数据。当前预览版可验证并回放加密 WAL 中已提交的数据；请正常关闭微信以取得稳定副本，绝不删除源 WAL。正常退出或错误时清理临时目录；强制终止或断电可能留下明文。以修改时间和大小检测源变化。

**English:** `decrypt` saves only the chat databases, `contact.db` and `message_N.db`, to a new plaintext directory; auxiliary full-text indexes and media databases are outside this release. Reads use private temporary copies and a SQLite index without modifying originals. The current preview release can validate and replay committed data from encrypted WALs; close WeChat normally for a stable copy, and never delete a source WAL. Normal exits and errors clean temporary data; a forced kill or power loss may leave plaintext behind. Changes are detected by modification time and size.

## MCP setup / MCP 配置

**中文：** 下方是通用 JSON 模板；宿主配置格式可能不同，请按宿主文档调整。不要在宿主配置中写密钥。

**English:** The following is a generic JSON template. Host configuration formats may differ, so adapt it to the host's documentation. Do not put keys in host configuration.

```json
{
  "mcpServers": {
    "wxmori": {
      "command": "C:/tools/wxmori.exe",
      "args": ["--config", "C:/path/config.json", "serve"]
    }
  }
}
```

## Upgrade from v0.2.0-alpha.1 to v0.2.0-alpha.2 / 从 v0.2.0-alpha.1 升级到 v0.2.0-alpha.2

**中文：** 升级前备份现有 `config.json`，并备份其同目录下的 `.wxmori-keys` 文件夹（如果存在）。下载并解压 `v0.2.0-alpha.2`，将宿主 MCP 配置中的 `command` 改为新程序路径；保留原来的 `--config` 参数和配置文件路径。示例：

```json
{
  "mcpServers": {
    "wxmori": {
      "command": "C:/Tools/WxMori/wxmori.exe",
      "args": ["--config", "C:/path/config.json", "serve"]
    }
  }
}
```

**中文：** 使用原账号和配置时，无需再次运行 `init` 或重新捕获密钥。重启 WxMori MCP 服务以加载新程序；如果宿主不能单独重启该服务，则重启 WorkBuddy。确认工具发现列出 9 个工具后，先调用 `wxmori_status` 并传 `{ "inspect": true }`，再调用 `list_sessions`；确认会话正确后，才按需调用 `export_chat`。只有明确要更换数据源时才重新配置；`init --force` 会替换配置文件，不会自动备份。不要混用不同账号的密钥环。

**English:** Before upgrading, back up the existing `config.json` and the `.wxmori-keys` folder beside it, if present. Download and extract `v0.2.0-alpha.2`, then change the MCP host's `command` to the new executable while keeping the existing `--config` argument and config-file path. The example above uses generic paths.

**English:** When keeping the same account and config, you do not need to run `init` again or capture keys again. Restart the WxMori MCP service to load the new executable; if the host cannot restart that service by itself, restart WorkBuddy. Confirm tool discovery lists 9 tools, call `wxmori_status` with `{ "inspect": true }`, then call `list_sessions`; after confirming the sessions, call `export_chat` only as needed. Reconfigure only when you intentionally choose a different data source; `init --force` replaces the config file and does not make a backup. Do not mix keyrings between accounts.

## Data and status / 数据与状态

**中文：** 当前 `v0.2.0-alpha.2` 预览版的 MCP 有九个工具：`wxmori_status`、`list_sessions`、`get_messages`、`search_messages`、`get_contacts`、`export_chat`、实验性的 `capture_key`、`list_accounts` 和 `export_all_chats`。日期 `from` 包含、`to` 不包含，按配置时区解释，默认 `Asia/Shanghai`。搜索会将空白分隔的关键词按 AND 组合，并进行字面子串匹配。会话列表和联系人默认最多 50 条、上限 200；消息和搜索默认最多 100 条、上限 500；偏移量范围为 0 至 1,000,000。

**English:** The current `v0.2.0-alpha.2` preview has nine MCP tools: `wxmori_status`, `list_sessions`, `get_messages`, `search_messages`, `get_contacts`, `export_chat`, experimental `capture_key`, `list_accounts`, and `export_all_chats`. Date `from` is inclusive and `to` exclusive, interpreted in the configured timezone, default `Asia/Shanghai`. Search combines whitespace-separated terms with AND and matches literal substrings. Session and contact lists default to 50 items, max 200; message and search results default to 100, max 500; offsets range from 0 to 1,000,000.

**中文：** 消息预览每条最多 8192 字节、每页约 64 KiB；导出读取全部匹配消息，单条解码上限 4 MiB。保留系统消息，附件以占位信息表示，不导出其二进制。导出按月生成 TXT 或 JSONL 文件，并返回路径、条数、SHA-256 和时间戳。未识别的类型或发送者会有占位符和警告。

**English:** Previews allow 8,192 bytes per body and about 64 KiB per page. Exports read all matching messages, with a 4 MiB decoded limit per message. System messages are retained; attachments have placeholders and their binaries are not exported. Monthly TXT or JSONL files return paths, counts, SHA-256 hashes, and a timestamp. Unknown types or senders are reported with placeholders and warnings.

**中文：** 当前预览版新增 `export --all-sessions`（或 `--all`）及 MCP `export_all_chats`，按会话和月份分文件写入 `chat-<12位ID哈希>-<清理后的名称>` 子目录；每批文件以原子方式发布，并返回完整清单（路径、条数和 SHA-256）。MCP 文件预览最多 50 项。现有 `export_chat` 仍要求指定会话。默认 `status` 保持轻量；`whoami`、`status --inspect` 或 `wxmori_status` 的 `{inspect:true}` 会构建已验证快照。只有全局会话数不超过 3 且消息数不超过 10 才报告 `SPARSE_LOCAL_HISTORY`；小于 1 MiB 的消息库只是文件大小提示，不能证明历史缺失或未同步。详见[故障排查](docs/troubleshooting.md)。

**English:** The current preview adds `export --all-sessions` (or `--all`) and MCP `export_all_chats`. It writes separate per-session/per-month files under `chat-<12-hex-ID-hash>-<sanitized-name>` directories, publishes each batch atomically, and returns a full manifest of paths, counts, and SHA-256 hashes. MCP file previews are capped at 50. Existing `export_chat` still requires a session. Default `status` remains lightweight; `whoami`, `status --inspect`, or `wxmori_status` with `{inspect:true}` builds a verified snapshot. `SPARSE_LOCAL_HISTORY` is reported only when global counts are both at most 3 sessions and at most 10 messages. A message-store file smaller than 1 MiB is only a size hint; it does not prove that history is missing or unsynced. See [troubleshooting](docs/troubleshooting.md).

**中文：** alpha1 的七工具 CI 通过；一次 `v0.2.0-dev` 单账号试验验证了 12/12 核心库密钥，并完成读取和 TXT/JSONL 导出。样本当前代次 WAL 提交帧为 0；完整证据及限制见[测试记录](docs/testing.md)与 [Luna 调用记录](docs/luna-call-trial.md)。
**English:** Alpha1 seven-tool CI passed. A single-account `v0.2.0-dev` trial verified keys for all 12 core databases and completed reads and TXT/JSONL exports. Its WALs had zero current-generation commits; see [testing notes](docs/testing.md) and the [Luna trial](docs/luna-call-trial.md) for evidence and limits.

See [docs/architecture.md](docs/architecture.md), [docs/mcp.md](docs/mcp.md), [docs/security.md](docs/security.md), [troubleshooting](docs/troubleshooting.md), [CONTRIBUTING.md](CONTRIBUTING.md), and [SECURITY.md](SECURITY.md) for details. / 详情见上述文档及[故障排查](docs/troubleshooting.md)。
