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

**中文：** 下表描述当前预览版 `v0.2.0-alpha.2` 的九工具接口。Luna 已通过四协议/九工具合成冒烟；alpha2 真实本机试用只检查了状态、身份和已有密钥，没有捕获、重新登录或导出。

**English:** The table describes the nine-tool interface in the current `v0.2.0-alpha.2` preview. Luna passed a synthetic smoke across four protocols and all nine tools. The live alpha2 trial covered status, identity, and existing keys only; it did not capture, re-login, or export.

| Tool / 工具 | Arguments / 参数 |
| --- | --- |
| `wxmori_status` | `{inspect?: boolean}` |
| `list_sessions` | `{keyword?, limit?, offset?}` |
| `get_messages` | `{session_id, from?, to?, limit?, offset?}` |
| `search_messages` | `{query, session_id?, from?, to?, limit?, offset?}` |
| `get_contacts` | `{query?, limit?, offset?}` |
| `export_chat` | `{session_id, months?: string[], from?, to?, format?: "txt"|"jsonl"}` |
| `capture_key` | `{action: "start"|"status"|"stop", timeout_seconds?: int, mode?: "auto"|"read_only"}` |
| `list_accounts` | `{root?: string}` |
| `export_all_chats` | `{months?: string[], from?, to?, format?: "txt"|"jsonl", files_preview?: int}` |

**中文：** `capture_key` 是实验性 Windows 64 位功能。`start` 异步启动并默认等待 300 秒，最大 600 秒；`mode` 可选 `auto`（默认）或 `read_only`。在支持的 Windows AMD64 上，`auto` 动态识别 AES 密钥设置函数并使用临时硬件执行断点；这会短暂暂停微信线程并临时设置调试寄存器，结束、停止或超时时恢复并分离。它不修改目标代码或注入 DLL。`read_only` 只轮询可读内存；其他 Windows 64 位环境回退到轮询。调用者必须先轮询到 `status.ready_for_login: true`，再请用户在本机重新登录。终态为 `completed`、`partial`、`timeout`、`stopped` 或 `failed`。启动成功只表示任务已开始，不表示捕获成功。`key_setup_events` 只表明观察到密钥设置相关事件，不证明用户已登录；`candidate_count` 表示验证尝试次数。未验证候选不得直接报告账号不匹配。只有发现其他本地账号的 `contact` 和 `message` 首页密钥在两个不同盐下通过 HMAC，而所选账号持续 10 秒仍无匹配时，才报告 `KEY_ACCOUNT_MISMATCH` 并停止/分离；最多报告 8 个本地替代项，不保存其他账号密钥，也不自动切换。`stop` 请求停止当前捕获。

**English:** `capture_key` is experimental on Windows 64-bit. `start` launches asynchronously with a default timeout of 300 seconds and a maximum of 600; optional `mode` is `auto` (default) or `read_only`. On supported Windows AMD64, `auto` dynamically recognizes the AES key-setup function and uses temporary hardware execution breakpoints. This briefly pauses WeChat threads and temporarily sets debug registers, which are restored on completion, stop, or timeout before detach. It does not patch target code or inject a DLL. `read_only` polls readable memory only; other Windows 64-bit environments fall back to polling. The caller must poll until `status.ready_for_login: true` before asking the user to log in again locally. Terminal states are `completed`, `partial`, `timeout`, `stopped`, and `failed`. A successful start means only that the job began, not that capture succeeded. `key_setup_events` means a key-setup-related event was observed, not that login was proven; `candidate_count` counts verification attempts. Unverified candidates alone do not prove an account mismatch. `KEY_ACCOUNT_MISMATCH` is reported and capture stopped/detached only after HMAC-valid `contact` and `message` pages from another local account match under two distinct salts while the selected account has had no match for 10 seconds. At most eight local alternatives are reported; foreign-account keys are not saved and the account is never switched automatically. `stop` requests that the current capture stop.

**中文：** 捕获使用 `init` 选定的现有加密 `data_dir`，不通过 MCP 接收账号路径、密钥、微信密码或二维码令牌。仅保存通过所选数据库页 HMAC 验证的每库密钥；输出、MCP 结果、日志和转储不包含密钥。DPAPI `CurrentUser` 密钥环保存在配置文件旁 `.wxmori-keys/`，无需改配置即可供 `decrypt` 和 `serve` 使用。手动 `key_file` 或 `WXMORI_MASTER_KEY` 仍可作为兼容回退。密钥环与账号目录及盐绑定，不要在账号之间混用。

**English:** Capture uses the existing encrypted `data_dir` selected by `init`; MCP does not accept account paths, keys, WeChat passwords, or QR tokens. Only per-database keys verified against the selected database page HMAC are saved; keys are excluded from tool results, logs, and dumps. The DPAPI `CurrentUser` keyring is stored beside the config file under `.wxmori-keys/` and is available to `decrypt` and `serve` without a config edit. Manual `key_file` or `WXMORI_MASTER_KEY` remains a compatibility fallback. The keyring is bound to the account directory and database salt; do not mix accounts.

**中文：** `list_accounts` 只返回账号目录元数据；`wxmori_status` 默认快速检查，传 `{inspect:true}` 才构建已验证快照。`whoami` 仅在解密后的 `contact.username` 与目录推导 ID 完全相等时确认本人身份，否则报告未知，不按昵称推测。`export_all_chats` 为每个会话/月生成独立 TXT/JSONL，写入 `chat-<12位ID哈希>-<清理后的名称>` 目录，每批以原子方式发布，并返回路径、条数和 SHA-256 清单；`files_preview` 最多返回 50 项。

**English:** `list_accounts` returns account-directory metadata only. `wxmori_status` is a cheap readiness check by default; `{inspect:true}` builds a verified snapshot. `whoami` confirms identity only when decrypted `contact.username` exactly matches the directory-derived ID; otherwise it reports unknown and never guesses from a nickname. `export_all_chats` writes separate TXT/JSONL files per session and month under `chat-<12-hex-ID-hash>-<sanitized-name>` directories, publishes each batch atomically, and returns a manifest of paths, counts, and SHA-256 hashes; `files_preview` is capped at 50 entries.

**中文：** CLI 命令 `wxmori --config PATH key capture --timeout 300 --mode auto --progress human` 默认 300 秒、上限 600 秒；`human` 在 stderr 单行更新，最终 JSON 输出到 stdout。`--progress json` 在状态变化或每 5 秒输出，最后 30 秒提醒即将超时。`--mode read_only` 选择轮询，Ctrl+C 停止。数据库文件保持只读。程序校验加密 WAL 校验和、盐代次、每页 HMAC 与提交边界，并只在私有副本回放已提交帧；正常关闭微信仍有助于取得稳定副本。当前真实样本的 WAL 当前代次提交帧为 0，因此真实提交帧回放仅由合成测试覆盖。已发布 alpha1 仍不支持非空加密 WAL。请勿删除 WAL。若访问被拒绝，请确保 WxMori 和微信在同一 Windows 用户会话运行，并遵循安全策略；不要关闭安全软件。若安全软件拦截程序，请按厂商流程逐文件提交审查；不要默认认定为误报。

**English:** CLI capture runs with `wxmori --config PATH key capture --timeout 300 --mode auto --progress human`; timeout defaults to 300 seconds and is capped at 600. `human` updates one stderr line and prints final JSON to stdout. `--progress json` prints on state changes or every five seconds, with a final-30-second warning. `--mode read_only` selects polling; Ctrl+C stops it. Database files remain read-only. The program validates encrypted-WAL checksums, salt generations, per-page HMACs, and commit boundaries, replaying committed frames only in a private copy; closing WeChat normally is still recommended for a stable copy. The 12 WALs in the actual trial contained no current-generation committed frames, so real committed-frame replay is covered only by synthetic tests. Published alpha1 still does not support non-empty encrypted WAL. Never delete the WAL. If access is denied, ensure WxMori and WeChat run in the same Windows user session and follow security policy; do not disable security software. If security software blocks the executable, submit the file for vendor review; do not presume a false positive.

**中文：** `session_id` 接受精确 ID 或唯一显示名称；名称不唯一时返回候选项。成功结果为结构化 JSON，并复制为文本内容；错误包含 `code`、`message`、`hint`。日期 `from` 包含、`to` 不包含，按配置时区处理；月份格式为 `YYYY-MM`。搜索关键词按空白切分后执行 AND 字面子串匹配。

**English:** `session_id` accepts an exact ID or unique display name; ambiguous names return candidates. Successful results are structured JSON duplicated as text content; errors include `code`, `message`, and `hint`. Date `from` is inclusive and `to` exclusive in the configured timezone; months use `YYYY-MM`. Search splits terms on whitespace and matches literal substrings with AND semantics.

**中文：** 会话和联系人默认每页最多 50 条、上限 200；消息和搜索默认 100 条、上限 500；`offset` 范围为 0 至 1,000,000。普通结果的单条正文最多 8192 字节，每页正文约 64 KiB。完整 TXT/JSONL 导出每条消息最多 4 MiB，按月写入配置的 `output_dir`，并返回路径、数量、SHA-256 和时间戳。系统消息会保留，附件以占位信息表示，不导出附件二进制。

**English:** Session and contact pages default to 50 items, max 200; message and search pages default to 100, max 500; `offset` ranges from 0 to 1,000,000. Ordinary results include up to 8,192 body bytes per message and about 64 KiB per page. Full TXT/JSONL exports support up to 4 MiB per message, write monthly files under configured `output_dir`, and return paths, counts, SHA-256, and timestamp. System messages are preserved; attachments use placeholders and their binaries are not exported.

## Deployment smoke check / 部署冒烟检查

**中文：** 已发布 alpha1 的七工具 CI 和单账号捕获/导出属于历史证据。alpha2 本机 Go 测试/vet、Luna 四协议/九工具合成冒烟及仅检查状态/身份的真实 MCP 调用通过。提交 `f174842` 的三系统 CI 通过完整程序启动、Linux race 检查及独立四协议/九工具 stdio 冒烟（[运行记录](https://github.com/axiomzero-qed/WxMori/actions/runs/39072189240)）。alpha2 真实登录捕获和真实已提交帧 WAL 回放仍未验证。

**English:** The published alpha1 seven-tool CI and single-account capture/export trial are historical evidence. Alpha2 local Go tests/vet, Luna's synthetic smoke across four protocols/nine tools, and a live MCP call limited to status/identity checks passed. Commit `f174842` passed three-system CI for executable startup, Linux race checks, and independent stdio smoke across four protocols/nine tools ([run](https://github.com/axiomzero-qed/WxMori/actions/runs/39072189240)). Real alpha2 login capture and real committed-WAL replay remain unverified.
