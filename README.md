# WxMori / 微忆

## About / 项目简介

**中文：** WxMori（微忆）是独立的 Go 命令行工具和本地 MCP 服务，用于读取微信 4.x 聊天数据库。当前开发版本为 `v0.2.0-alpha.1`，通过官方 `github.com/modelcontextprotocol/go-sdk` 使用 stdio 通信，采用 MIT 许可证。仅从源码构建需要 Go 1.26 或更新版本。

**English:** WxMori is an independent Go CLI and local MCP server for reading WeChat 4.x chat databases. The current development version is `v0.2.0-alpha.1`; it uses the official `github.com/modelcontextprotocol/go-sdk` over stdio, under the MIT license. Go 1.26 or later is needed only to build from source.

**中文：** 当前候选版本 `v0.2.0-alpha.1` 新增 Windows AMD64 登录时实验性自动取密钥和加密 WAL 私有副本回放。一次 Weixin 4.1.15.56 单账号 Luna/Python stdio 实测成功验证 12/12 核心数据库密钥，并完成保留 WAL 时的读取与 TXT/JSONL 导出；该样本当前代次提交帧为 0，提交帧回放目前仅有合成测试覆盖。自动捕获仅在该版本/账号样本验证；其他微信版本和宿主尚未验证。已有解密 SQLite 数据和手动密钥输入支持 Windows、Linux、macOS。已发布 `v0.1.0-alpha.1` 的能力不同，详见历史发布记录。

**English:** Candidate `v0.2.0-alpha.1` adds experimental login-time key capture on Windows AMD64 and encrypted-WAL replay in private copies. One Luna/Python stdio trial on a single Weixin 4.1.15.56 account verified keys for all 12 core databases, then passed reads and TXT/JSONL exports with retained WALs. That sample had zero current-generation committed frames, so committed-frame replay has synthetic test coverage only. Automatic capture has been tested only on this version/account sample; other WeChat versions and hosts are unverified. Existing decrypted SQLite data and manually supplied keys work on Windows, Linux, and macOS. See the release history for differences in published `v0.1.0-alpha.1`.

**中文：** 登录时自动取密钥仅支持 Windows AMD64，仍属实验功能；不使用 WeFlow DLL 或代码。默认 `auto` 模式动态识别 AES 密钥设置函数，并通过 Windows 调试器硬件执行断点监测；不修改目标代码或注入 DLL，但会短暂暂停微信线程并临时设置、随后恢复调试寄存器。`read_only` 模式只轮询可读内存。此捕获仅在一个 Weixin 4.1.15.56 单账号样本完成实测。加密 WAL 在私有副本中校验并回放。详见 [测试记录](docs/testing.md) 和 [发布说明](docs/releases/v0.2.0-alpha.1.md)。

**English:** Automatic login-time key capture is experimental and supported only on Windows AMD64; no WeFlow DLL or code is used. Default `auto` mode dynamically recognizes the AES key-setup function and monitors it with Windows debugger hardware execution breakpoints. It does not patch target code or inject a DLL, but briefly pauses WeChat threads and temporarily sets then restores debug registers. `read_only` mode polls readable memory only. Capture has been tested with one Weixin 4.1.15.56 single-account sample. Encrypted WALs are validated and replayed in private copies. See [testing](docs/testing.md) and the [release notes](docs/releases/v0.2.0-alpha.1.md).

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

**中文：** `wxmori init` 只发现候选账号并写入不含密钥的配置。若有多个账号，需用 `--data-dir` 明确选择一个；不要混用账号。`data_dir` 指向所选账号的 `db_storage`；已有解密 SQLite 数据则设置 `decrypted_dir`。两者必须且只能设置一个。相对路径以配置文件目录为基准；配置默认位于用户配置目录的 `wxmori/config.json`，也可用 `--config PATH` 或 `WXMORI_CONFIG` 指定。

**English:** `wxmori init` discovers candidate accounts and writes a config without keys. If several accounts are found, select one explicitly with `--data-dir`; do not mix accounts. `data_dir` points to the selected account's `db_storage`; use `decrypted_dir` for existing decrypted SQLite data. Set exactly one of these fields. Relative paths are resolved from the config directory. The default config is under the user config directory at `wxmori/config.json`; use `--config PATH` or `WXMORI_CONFIG` to select another.

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

**中文：** CLI 捕获命令为 `wxmori --config PATH key capture --timeout 180 --mode auto`，在前台显示有界进度，Ctrl+C 停止；`--mode read_only` 选择只读轮询。MCP 的 `capture_key` 支持 `start`、`status`、`stop` 和可选 `mode: auto|read_only`。状态中的 `ready_for_login` 为真后，才请用户在本机重新登录；启动成功不代表捕获成功。详情见 [MCP 集成](docs/mcp.md) 和 [安全与隐私](docs/security.md)。

**English:** Start CLI capture with `wxmori --config PATH key capture --timeout 180 --mode auto`; it runs in the foreground with bounded progress and stops on Ctrl+C. Use `--mode read_only` for polling only. MCP `capture_key` supports `start`, `status`, `stop`, and optional `mode: auto|read_only`. Ask the user to log in locally only after `ready_for_login` becomes true; a successful start does not mean capture succeeded. See [MCP integration](docs/mcp.md) and [security and privacy](docs/security.md).

**中文：** `decrypt` 只将聊天所需的 `contact.db` 和 `message_N.db` 保存到新明文目录，辅助全文索引和媒体库不在本版范围内。读取时使用私有临时副本和 SQLite 索引，不修改源数据。当前候选版可验证并回放加密 WAL 中已提交的数据；请正常关闭微信以取得稳定副本，绝不删除源 WAL。正常退出或错误时清理临时目录；强制终止或断电可能留下明文。以修改时间和大小检测源变化。

**English:** `decrypt` saves only the chat databases, `contact.db` and `message_N.db`, to a new plaintext directory; auxiliary full-text indexes and media databases are outside this release. Reads use private temporary copies and a SQLite index without modifying originals. The current candidate can validate and replay committed data from encrypted WALs; close WeChat normally for a stable copy, and never delete a source WAL. Normal exits and errors clean temporary data; a forced kill or power loss may leave plaintext behind. Changes are detected by modification time and size.

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

## Data and status / 数据与状态

**中文：** 当前 MCP 有七个工具：`wxmori_status`、`list_sessions`、`get_messages`、`search_messages`、`get_contacts`、`export_chat` 和实验性的 `capture_key`。日期 `from` 包含、`to` 不包含，按配置时区解释，默认 `Asia/Shanghai`。搜索会将空白分隔的关键词按 AND 组合，并进行字面子串匹配。会话列表和联系人默认最多 50 条、上限 200；消息和搜索默认最多 100 条、上限 500；偏移量范围为 0 至 1,000,000。

**English:** The current MCP has seven tools: `wxmori_status`, `list_sessions`, `get_messages`, `search_messages`, `get_contacts`, `export_chat`, and experimental `capture_key`. Date `from` is inclusive and `to` exclusive, interpreted in the configured timezone, default `Asia/Shanghai`. Search combines whitespace-separated terms with AND and matches literal substrings. Session and contact lists default to 50 items, max 200; message and search results default to 100, max 500; offsets range from 0 to 1,000,000.

**中文：** 消息预览每条最多 8192 字节、每页约 64 KiB；导出读取全部匹配消息，单条解码上限 4 MiB。保留系统消息，附件以占位信息表示，不导出其二进制。导出按月生成 TXT 或 JSONL 文件，并返回路径、条数、SHA-256 和时间戳。未识别的类型或发送者会有占位符和警告。

**English:** Previews allow 8,192 bytes per body and about 64 KiB per page. Exports read all matching messages, with a 4 MiB decoded limit per message. System messages are retained; attachments have placeholders and their binaries are not exported. Monthly TXT or JSONL files return paths, counts, SHA-256 hashes, and a timestamp. Unknown types or senders are reported with placeholders and warnings.

**中文：** `v0.2.0-alpha.1` 的本机 `go test ./...`、`go vet ./...` 通过。Luna 经 Python stdio MCP 在单账号登录试验中捕获并验证 12/12 核心数据库密钥；新进程从 DPAPI 密钥环重载 12 个密钥，并在保留加密 WAL 时完成读取及导出。12 个 WAL 文件各有旧代次尾帧被排除，当前代次提交帧为 0；提交帧回放仅由合成测试覆盖。TXT、JSONL 各 381 条，摘要和 UTF-8 核验通过，TXT 的 381 个时间戳标题及 235 条多行正文完整，JSONL 381 行有效。真实快照的源文件大小/修改时间指纹稳定；源哈希未在该实测中验证。自动捕获实测只覆盖一个 Windows/Weixin 版本、账号和 Luna/Python stdio 宿主。当前发布 CI 仍待通过。详情见[测试记录](docs/testing.md)和 [Luna 调用记录](docs/luna-call-trial.md)。

**English:** Local `go test ./...` and `go vet ./...` pass for `v0.2.0-alpha.1`. In a single-account login trial through Luna's Python stdio MCP host, capture verified keys for all 12 core databases. A fresh process reloaded all 12 from DPAPI and read/exported data while preserving encrypted WALs. Each of the 12 WAL files had an excluded stale-generation tail; there were zero current-generation committed frames, so committed-frame replay is covered by synthetic tests only. TXT and JSONL each contained 381 records with matching export hashes and valid UTF-8; TXT had 381 timestamp headers and preserved all 235 multiline bodies, and JSONL had 381 valid rows. Real-snapshot source size/mtime fingerprints remained stable; source hashes were not verified in that trial. Automatic capture covers only one Windows/Weixin version, account, and Luna/Python stdio host. Release CI is still pending. See [validation scope](docs/testing.md) and the [Luna trial](docs/luna-call-trial.md).

See [docs/architecture.md](docs/architecture.md), [docs/mcp.md](docs/mcp.md), [docs/security.md](docs/security.md), [CONTRIBUTING.md](CONTRIBUTING.md), and [SECURITY.md](SECURITY.md) for details. / 详情见上述文档。
