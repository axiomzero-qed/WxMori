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

**中文：** `session_id` 接受精确 ID 或唯一显示名称；名称不唯一时返回候选项。成功结果为结构化 JSON，并复制为文本内容；错误包含 `code`、`message`、`hint`。日期 `from` 包含、`to` 不包含，按配置时区处理；月份格式为 `YYYY-MM`。搜索关键词按空白切分后执行 AND 字面子串匹配。

**English:** `session_id` accepts an exact ID or unique display name; ambiguous names return candidates. Successful results are structured JSON duplicated as text content; errors include `code`, `message`, and `hint`. Date `from` is inclusive and `to` exclusive in the configured timezone; months use `YYYY-MM`. Search splits terms on whitespace and matches literal substrings with AND semantics.

**中文：** 会话和联系人默认每页最多 50 条、上限 200；消息和搜索默认 100 条、上限 500；`offset` 范围为 0 至 1,000,000。普通结果的单条正文最多 8192 字节，每页正文约 64 KiB。完整 TXT/JSONL 导出每条消息最多 4 MiB，按月写入配置的 `output_dir`，并返回路径、数量、SHA-256 和时间戳。系统消息会保留，附件以占位信息表示，不导出附件二进制。

**English:** Session and contact pages default to 50 items, max 200; message and search pages default to 100, max 500; `offset` ranges from 0 to 1,000,000. Ordinary results include up to 8,192 body bytes per message and about 64 KiB per page. Full TXT/JSONL exports support up to 4 MiB per message, write monthly files under configured `output_dir`, and return paths, counts, SHA-256, and timestamp. System messages are preserved; attachments use placeholders and their binaries are not exported.

## Deployment smoke check / 部署冒烟检查

**中文：** 用合成数据运行 `wxmori demo --dir ./demo-data`，再使用打印出的配置路径运行 `status` 并从 MCP 宿主启动 `serve`。检查服务初始化、六个工具发现、结构化状态结果、空数据查询的有界结果，以及正常退出时 stdio 关闭。此流程只检查合成数据路径，不证明所有宿主或真实数据库均兼容。合成数据回归检查与 GPT-6 Luna 实际调用测试目前待运行。

**English:** Run `wxmori demo --dir ./demo-data`, then run `status` with the printed config path and launch `serve` from the MCP host. Check server initialization, discovery of all six tools, structured status output, bounded empty-query results, and stdio closure on normal exit. This covers only the synthetic-data path and does not prove compatibility with every host or real database. Synthetic-data regression checks and GPT-6 Luna tool-call tests are currently pending.
