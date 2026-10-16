# WxMori / 微忆

## About / 项目简介

**中文：** WxMori（微忆）是独立的 Go 命令行工具和本地 MCP 服务，用于读取微信 4.x 聊天数据库。它以单个二进制文件运行，通过官方 `github.com/modelcontextprotocol/go-sdk` 使用 stdio 通信，采用 MIT 许可证。首个预览版为 `v0.1.0-alpha.1`；仅从源码构建需要 Go 1.26 或更新版本。

**English:** WxMori is an independent Go CLI and local MCP server for reading WeChat 4.x chat databases. It runs as a single binary using the official `github.com/modelcontextprotocol/go-sdk` over stdio, under the MIT license. The first preview is `v0.1.0-alpha.1`; Go 1.26 or later is needed only to build from source.

**中文：** 当前版本支持用户提供主密钥的 Windows 微信 4.x 加密数据库，也支持 Windows、Linux 和 macOS 上已有的解密 SQLite 数据。没有自动提取密钥或进程钩子；不宣称兼容所有微信版本或 MCP 宿主。

**English:** The current version supports encrypted Windows WeChat 4.x databases when the user supplies the master key, and existing decrypted SQLite data on Windows, Linux, and macOS. Automatic key extraction and process hooks are not included. Compatibility with every WeChat version or MCP host is not claimed.

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

**中文：** `wxmori init` 只用于发现候选账号并写入不含密钥的配置。若有多个账号，需用 `--data-dir` 明确选择一个；初始 alpha 不合并账号。`data_dir` 指向所选账号的 `db_storage`；已有解密 SQLite 数据则设置 `decrypted_dir`。两者必须且只能设置一个。相对路径以配置文件目录为基准；配置默认位于用户配置目录的 `wxmori/config.json`，也可用 `--config PATH` 或 `WXMORI_CONFIG` 指定。

**English:** `wxmori init` discovers candidate accounts and writes a config without the key. If several accounts are found, select one explicitly with `--data-dir`; the initial alpha does not merge accounts. `data_dir` points to the selected account's `db_storage`; use `decrypted_dir` for existing decrypted SQLite data. Set exactly one of these fields. Relative paths are resolved from the config directory. The default config is under the user config directory at `wxmori/config.json`; use `--config PATH` or `WXMORI_CONFIG` to select another.

**中文：** 主密钥仅通过本地环境变量 `WXMORI_MASTER_KEY` 或 `key_file` 提供。密钥文件是包含 `{"master_key":"64 位十六进制字符"}` 的 JSON。请勿将密钥放进命令参数、MCP 参数或聊天。配置示例：

**English:** Supply the master key locally through `WXMORI_MASTER_KEY` or `key_file`. The key file is JSON containing `{"master_key":"64 hexadecimal characters"}`. Never put the key in command arguments, MCP arguments, or chats. Example config:

```json
{
  "data_dir": "C:/Users/you/Documents/xwechat_files/account-id/db_storage",
  "key_file": "C:/Users/you/.config/wxmori/key.json",
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

**中文：** `decrypt` 只将聊天所需的 `contact.db` 和 `message_N.db` 保存到新明文目录，辅助全文索引和媒体库不在本版范围内。读取时使用私有临时副本和 SQLite 索引，不修改源数据。正常退出或错误时清理临时目录；强制终止或断电可能留下明文。以修改时间和大小检测源变化。非空加密 WAL 不支持：请正常关闭微信，让微信完成 checkpoint 后重试；请勿删除源 WAL。

**English:** `decrypt` saves only the chat databases, `contact.db` and `message_N.db`, to a new plaintext directory; auxiliary full-text indexes and media databases are outside this release. Reads use private temporary copies and a SQLite index without modifying originals. Normal exits and errors clean temporary data; a forced kill or power loss may leave plaintext behind. Changes are detected by modification time and size. Non-empty encrypted WAL is unsupported: close WeChat normally to let it checkpoint, then retry. Never delete a source WAL.

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

**中文：** MCP 工具包括 `wxmori_status`、`list_sessions`、`get_messages`、`search_messages`、`get_contacts` 和 `export_chat`。日期 `from` 包含、`to` 不包含，按配置时区解释，默认 `Asia/Shanghai`。搜索会将空白分隔的关键词按 AND 组合，并进行字面子串匹配。会话列表和联系人默认最多 50 条、上限 200；消息和搜索默认最多 100 条、上限 500；偏移量范围为 0 至 1,000,000。

**English:** MCP tools are `wxmori_status`, `list_sessions`, `get_messages`, `search_messages`, `get_contacts`, and `export_chat`. Date `from` is inclusive and `to` exclusive, interpreted in the configured timezone, default `Asia/Shanghai`. Search combines whitespace-separated terms with AND and matches literal substrings. Session and contact lists default to 50 items, max 200; message and search results default to 100, max 500; offsets range from 0 to 1,000,000.

**中文：** 消息预览每条最多 8192 字节、每页约 64 KiB；导出读取全部匹配消息，单条解码上限 4 MiB。保留系统消息，附件以占位信息表示，不导出其二进制。导出按月生成 TXT 或 JSONL 文件，并返回路径、条数、SHA-256 和时间戳。未识别的类型或发送者会有占位符和警告。

**English:** Previews allow 8,192 bytes per body and about 64 KiB per page. Exports read all matching messages, with a 4 MiB decoded limit per message. System messages are retained; attachments have placeholders and their binaries are not exported. Monthly TXT or JSONL files return paths, counts, SHA-256 hashes, and a timestamp. Unknown types or senders are reported with placeholders and warnings.

**中文：** 合成回归、四个 MCP 协议版本与 GPT-6 Luna 实际调用已通过。范围见[测试记录](docs/testing.md)和 [Luna 调用记录](docs/luna-call-trial.md)。商业宿主和其他模型仍待实测。

**English:** Synthetic regression, four MCP protocol versions, and actual GPT-6 Luna calls passed. See [validation scope](docs/testing.md) and the [Luna trial](docs/luna-call-trial.md). Commercial hosts and other models still need testing.

See [docs/architecture.md](docs/architecture.md), [docs/mcp.md](docs/mcp.md), [docs/security.md](docs/security.md), [CONTRIBUTING.md](CONTRIBUTING.md), and [SECURITY.md](SECURITY.md) for details. / 详情见上述文档。
