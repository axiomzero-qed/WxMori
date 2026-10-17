# MCP integration / MCP 集成

## Server launch / 启动服务

**中文：** 服务使用 stdio 和官方 `github.com/modelcontextprotocol/go-sdk`。配置通过 `--config PATH`、`WXMORI_CONFIG` 或用户配置目录下的默认路径读取。数据库位置与密钥仅在本地配置，不作为 MCP 工具参数传递。

**English:** The server uses stdio and the official `github.com/modelcontextprotocol/go-sdk`. Configuration comes from `--config PATH`, `WXMORI_CONFIG`, or the default path under the user config directory. Database locations and keys are configured locally and are not MCP tool arguments.

通用 JSON 模板 / Generic JSON template:

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

**中文：** 各宿主的 JSON 或 TOML 字段与启动方式可能不同；请按宿主文档调整模板。不要把密钥放入宿主配置。

**English:** JSON or TOML fields and launch methods differ by host; adapt this template to the host documentation. Do not place keys in host configuration.

## Tool contract / 工具约定

| Tool / 工具 | Arguments / 参数 |
| --- | --- |
| `wxmori_status` | `{}` |
| `list_sessions` | `{keyword?, limit?, offset?}` |
| `get_messages` | `{session_id, from?, to?, limit?, offset?}` |
| `search_messages` | `{query, session_id?, from?, to?, limit?, offset?}` |
| `get_contacts` | `{query?, limit?, offset?}` |
| `export_chat` | `{session_id, months?: string[], from?, to?, format?: "txt"|"jsonl"}` |
| `capture_key` | `{action: "start"|"status"|"stop", timeout_seconds?: int}` |

**中文：** `capture_key` 是工作分支的实验性 Windows 64 位功能。`start` 异步启动并默认等待 180 秒，最大 600 秒；调用者应让用户在本机完成微信登录，并用 `status` 轮询。终态为 `completed`、`partial`、`timeout`、`stopped` 或 `failed`。启动成功只表示任务已开始，不表示捕获成功。`stop` 请求停止当前捕获。

**English:** `capture_key` is an experimental Windows 64-bit feature on the working branch. `start` launches asynchronously with a default timeout of 180 seconds and a maximum of 600; the caller should ask the user to log in to WeChat locally and poll with `status`. Terminal states are `completed`, `partial`, `timeout`, `stopped`, and `failed`. A successful start means only that the job began, not that capture succeeded. `stop` requests that the current capture stop.

**中文：** 捕获使用 `init` 选定的现有加密 `data_dir`，不通过 MCP 接收账号路径、密钥、微信密码或二维码令牌。仅保存通过所选数据库页 HMAC 验证的每库密钥；输出、MCP 结果、日志和转储不包含密钥。自动 DPAPI 密钥环无需改配置即可供 `decrypt` 和 `serve` 使用，手动 `key_file` 或 `WXMORI_MASTER_KEY` 仍可作为兼容回退。密钥环绑定账号目录和数据库盐，不要在账号之间混用。

**English:** Capture uses the existing encrypted `data_dir` selected by `init`; MCP does not accept account paths, keys, WeChat passwords, or QR tokens. Only per-database keys verified against the selected database page HMAC are saved; keys are excluded from tool results, logs, and dumps. The automatically protected DPAPI keyring is available to `decrypt` and `serve` without a config edit. Manual `key_file` or `WXMORI_MASTER_KEY` remains as a backward-compatible fallback. The keyring is bound to the account directory and database salt; do not mix accounts.

**中文：** CLI 命令 `wxmori --config PATH key capture --timeout 180` 在前台运行并显示有界进度，Ctrl+C 会停止。非空加密 WAL 仍不支持：捕获后请正常关闭微信再导出；不得删除 WAL。若进程访问被拒绝，请确保 WxMori 和微信在同一 Windows 用户会话中运行，并遵守设备的权限策略；不要关闭安全软件。实际微信登录捕获试验仍待完成。

**English:** The CLI command `wxmori --config PATH key capture --timeout 180` runs in the foreground with bounded progress; Ctrl+C stops it. Non-empty encrypted WAL remains unsupported: close WeChat normally after capture before exporting, and never delete the WAL. If process access is denied, ensure WxMori and WeChat run in the same Windows user session and follow the device's permission policy; do not disable security software. The real-WeChat login capture trial is still pending.

**中文：** `session_id` 接受精确 ID 或唯一显示名称；名称不唯一时返回候选项。成功结果为结构化 JSON，并复制为文本内容；错误包含 `code`、`message`、`hint`。日期 `from` 包含、`to` 不包含，按配置时区处理；月份格式为 `YYYY-MM`。搜索关键词按空白切分后执行 AND 字面子串匹配。

**English:** `session_id` accepts an exact ID or unique display name; ambiguous names return candidates. Successful results are structured JSON duplicated as text content; errors include `code`, `message`, and `hint`. Date `from` is inclusive and `to` exclusive in the configured timezone; months use `YYYY-MM`. Search splits terms on whitespace and matches literal substrings with AND semantics.

**中文：** 会话和联系人默认每页最多 50 条、上限 200；消息和搜索默认 100 条、上限 500；`offset` 范围为 0 至 1,000,000。普通结果的单条正文最多 8192 字节，每页正文约 64 KiB。完整 TXT/JSONL 导出每条消息最多 4 MiB，按月写入配置的 `output_dir`，并返回路径、数量、SHA-256 和时间戳。系统消息会保留，附件以占位信息表示，不导出附件二进制。

**English:** Session and contact pages default to 50 items, max 200; message and search pages default to 100, max 500; `offset` ranges from 0 to 1,000,000. Ordinary results include up to 8,192 body bytes per message and about 64 KiB per page. Full TXT/JSONL exports support up to 4 MiB per message, write monthly files under configured `output_dir`, and return paths, counts, SHA-256, and timestamp. System messages are preserved; attachments use placeholders and their binaries are not exported.

## Deployment smoke check / 部署冒烟检查

**中文：** 用合成数据运行 `wxmori demo --dir ./demo-data`，再使用打印出的配置路径运行 `status` 并从 MCP 宿主启动 `serve`。检查服务初始化、七个工具发现、结构化状态结果、空数据查询的有界结果，以及正常退出时 stdio 关闭。此流程只检查合成数据路径，不证明所有宿主或真实数据库均兼容。工作分支 `go test ./...` 和 `go vet ./...` 已通过；独立完整程序 stdio 测试、新捕获工具的 GPT-6 Luna 实际调用和真实微信登录捕获仍待完成。

**English:** Run `wxmori demo --dir ./demo-data`, then run `status` with the printed config path and launch `serve` from the MCP host. Check server initialization, discovery of all seven tools, structured status output, bounded empty-query results, and stdio closure on normal exit. This covers only the synthetic-data path and does not prove compatibility with every host or real database. Working-branch `go test ./...` and `go vet ./...` have passed; an independent full-program stdio test, an actual GPT-6 Luna call for the new capture tool, and the real-WeChat login capture trial remain pending.
