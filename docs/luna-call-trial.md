# WxMori MCP 调用试用 / MCP Call Trial

**模型 / Model:** gpt-6-luna
**日期 / Date:** 2026-10-17
**范围 / Scope:** 仅使用 `demo-data` 合成数据，通过真实子进程 stdio JSON-RPC 调用 MCP 工具。
Only synthetic `demo-data` was used, through real subprocess stdio JSON-RPC MCP calls.

本次试用不是在 Claude、Cursor 或其他商业宿主内完成的，也不能据此声称兼容所有模型或宿主。
This trial was not run inside Claude, Cursor, or another commercial host; it does not establish compatibility with all models or hosts.

## 实测步骤 / Steps observed

- 用 `tools/list` 发现工具和扁平参数 schema。中文请求“把微忆 Demo 群 2026 年 9 月的聊天记录导出来”先通过 `list_sessions` 查到群 ID `wxmori-demo@chatroom`，再用 `export_chat` 导出 2026-09。结果为 9 条；文件存在，9 行，SHA-256：`a922d4ed211a0b26c6a8a4c2fbf91902b7a1beb938e0de98816b00890cae73c8`。
  `tools/list` revealed the flat input schema. For the Chinese request to export the September 2026 “微忆 Demo” group, `list_sessions` returned its ID and `export_chat` produced 9 records. The file exists with 9 lines and the SHA-256 shown above.
- `search_messages` 查询 `Bob testing` 返回 1 条匹配。随后按英文请求以 `format: jsonl` 导出同一群 2026-09：9 条，文件存在，9 行，SHA-256：`ebb6b347e169e78b975e5a54de30ef555e9dd406b6c390477291e0520f05c696`。
  `search_messages` found one match for `Bob testing`. Following the English request, `export_chat` wrote September 2026 as JSONL: 9 records, 9 lines, with the SHA-256 shown above.
- 对“同名 Demo”执行导出得到 `AMBIGUOUS_SESSION`，提示从候选 ID 中选择。按虚构用户选择 `twin-b@chatroom` 重试成功，返回 0 条及空月份 `2026-09`，没有文件。
  Exporting “同名 Demo” returned `AMBIGUOUS_SESSION` and candidate IDs. After the fictional user selected `twin-b@chatroom`, retrying succeeded with 0 records and `2026-09` listed as empty; no file was created.
- 传入月份 `2026-13` 得到 `INVALID_MONTH`，提示格式 `YYYY-MM`；改为 `2026-09` 后导出成功，9 条。
  Passing month `2026-13` returned `INVALID_MONTH` with the expected `YYYY-MM` hint. Correcting it to `2026-09` exported 9 records.
- `get_messages` 以 `limit: 2` 读取前两页：offset 0 返回 2 条、`next_offset: 2`，消息 ID 为 101、102；按提示 offset 2 再读，返回 2 条、`next_offset: 4`，消息 ID 为 103、104。两页没有重叠。
  `get_messages` with `limit: 2` returned records 101–102 at offset 0 and records 103–104 at offset 2 (`next_offset: 4`); the pages did not overlap.
- 查询空月份 `2025-02` 返回 0 条和 `empty_months`。省略必需的 `session_id`，以及把 `limit` 传为字符串，都返回 `INVALID_ARGUMENTS`，提示对照 `tools/list` 的必填字段和类型。
  Querying the empty month `2025-02` returned 0 records and `empty_months`. Omitting required `session_id` or passing `limit` as a string returned `INVALID_ARGUMENTS`, with a hint to follow the required fields and types from `tools/list`.

## 静态副本调用 / Static-copy call

使用 GPT-6 Luna，通过 Python `Client` 对本机已校验的静态 SQLite 副本进行真实子进程 stdio MCP 调用。`list_sessions` 对目标关键词返回 5 个候选；按要求不猜测目标会话，因此停止该分支，没有执行导出或生成文件。错误码：无。
GPT-6 Luna made a real subprocess stdio MCP call through the Python `Client` using a locally verified static SQLite copy. `list_sessions` returned 5 candidates for the target keyword. The branch stopped without guessing, so no export was attempted and no files were generated. Error codes: none.

这是一个本机静态数据样本，不代表实时数据，也不保证包含最新消息。
This was one local static data sample; it does not represent live data or guarantee the latest messages.

## 静态副本导出验收 / Static-copy export acceptance

随后按用户提供的目标会话直接调用 `export_chat`，导出 2026-09 的 txt 和 JSONL。两次调用均成功，各含 381 条；两个文件均存在，工具返回的 SHA-256 与本机重算值一致，UTF-8 解码成功。txt 时间戳 header 数为 381，与消息数相同；未发现 `<appmsg` 或 `<sysmsg` 标签。读取结果中有 235 条正文含换行，235 条都能在 txt 导出中完整匹配，确认换行保留。JSONL 行数为 381，与消息数相同。
`export_chat` was then called directly for the user-supplied target session, exporting September 2026 as txt and JSONL. Both calls succeeded with 381 records each. Both files existed, their SHA-256 values matched the locally recomputed hashes, and both decoded as UTF-8. The txt contained 381 timestamp headers, matching its message count, with no `<appmsg` or `<sysmsg` tags. Of the 235 returned message bodies containing line breaks, all 235 matched the txt export in full, confirming line breaks were preserved. JSONL contained 381 lines, matching its message count.

该验收只针对一个本机静态 SQLite 副本样本，不是实时数据，也不保证包含最新消息。报告省略了目标会话身份、正文和导出路径。
This acceptance check covers one local static SQLite copy only; it is not live data and does not guarantee the latest messages. The report omits the target session identity, message text, and export paths.

## 观察 / Findings

工具发现结果、分页字段和错误 hint 足以引导这些步骤；本次调用不需要额外源码知识。名称查找、候选 ID、错误码及修正方式都能从工具响应中获得。导出文件的实际存在、条数和 SHA-256 已逐个核对。
Tool discovery, pagination fields, and error hints were enough to guide these steps; no additional source knowledge was needed. Name lookup, candidate IDs, error codes, and recovery steps came from tool responses. Export file existence, row counts, and SHA-256 values were checked.

试用中没有发现仍需上报的产品 bug。初次 PowerShell 命令行传 JSON 参数时遇到本地引号转义问题，改用提供的 Python `Client` 进行同一真实 stdio MCP 调用后继续；这属于试验调用方式问题，不是工具行为。
No product bug requiring escalation was found. The initial PowerShell command had a local JSON quoting issue; the same real stdio MCP calls continued through the provided Python `Client`. This was a test invocation issue, not tool behavior.

## Experimental capture control / 实验性捕获控制

**中文：** 此次实测运行时版本为 `v0.2.0-dev`（候选版本号仅在实测后更新）。Luna 使用 Python stdio MCP 宿主完成四个协议版本、七工具冒烟及实时捕获调用。一次 Weixin 4.1.15.56 单账号重新登录试验中，调试器捕获并通过 HMAC 验证 12/12 核心数据库密钥；微信完整退出后，新的 MCP 进程在 `master_key_available=false` 时从 DPAPI 密钥环加载 12 个密钥，状态 ready，按用户指定会话精确查询返回一个结果。12 个 WAL 文件各有旧代次尾帧被排除，当前代次提交帧为 0。TXT 和 JSONL 导出各 381 条且哈希匹配、UTF-8 有效；TXT 有 381 个时间戳标题，235 条多行正文全部保留，JSONL 有 381 行有效记录。真实快照的源文件大小/修改时间指纹保持稳定；源哈希仅在合成快照测试中验证未变。已提交帧回放仅由合成测试覆盖。调用前仍须轮询 `ready_for_login: true` 再请用户登录。

**English:** The runtime tested was `v0.2.0-dev` (the candidate version string changed after the trial). Luna used a Python stdio MCP host for four-protocol/seven-tool smoke calls and live capture. In one Weixin 4.1.15.56 single-account re-login trial, debugger capture obtained HMAC-verified keys for all 12 core databases. After WeChat fully exited, a fresh MCP process loaded all 12 from DPAPI with `master_key_available=false`; status was ready, and exact lookup of the user-specified session returned one result. Each of the 12 WAL files had an excluded stale-generation tail; there were zero current-generation committed frames. TXT and JSONL exports each contained 381 records with matching hashes and valid UTF-8; TXT had 381 timestamp headers and preserved all 235 multiline bodies, while JSONL had 381 valid rows. Real-snapshot source size/mtime fingerprints remained stable. Unchanged source hashes were verified in synthetic snapshot tests only. Committed-frame replay is covered only by synthetic tests. Before a capture call, continue polling until `ready_for_login: true` before asking the user to log in.

**中文：** 报告不记录账号身份、个人路径、密钥或转储。捕获和 WAL 回放仍是实验功能；这次单账号结果不证明其他微信版本、模型或宿主兼容，也不保证普遍可捕获。当前发布 CI 待完成。

**English:** The report omits account identity, personal paths, keys, and dumps. Capture and WAL replay remain experimental; this single-account result does not establish compatibility with other WeChat versions, models, or hosts, or guarantee general capture reliability. Release CI is pending.
