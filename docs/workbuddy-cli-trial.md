# WorkBuddy CLI Trial / WorkBuddy 命令行实测

**中文：** 日期：2026-10-18。以下内容是用户对 `v0.2.0-alpha.1` Windows 命令行试用的脱敏报告，不是独立 MCP、WorkBuddy 工具调用或模型试用结果。未记录账号标识、手机号、个人路径、密钥或聊天正文。

**English:** Date: 2026-10-18. This is a sanitized user report of a `v0.2.0-alpha.1` Windows CLI trial. It is not an independent MCP, WorkBuddy tool-call, or model trial. Account identifiers, phone numbers, personal paths, keys, and message bodies are omitted.

## Observed results / 实测结果

**中文：** 在 Weixin 4.1.15.56、多账号目录环境中的一个新本地账号上，`auto` 登录捕获约 33 秒后完成，2/2 个所需数据库密钥通过 HMAC 验证。捕获进程结束后，新 CLI `status` 进程从 DPAPI 密钥环加载密钥并报告 ready。保留 WAL 时完成会话查询和导出；本次样本仅有 1 条消息。两个 WAL 均通过校验并排除旧代次尾帧，但当前代次提交帧数为 0，因此本次试用没有验证真实的已提交 WAL 帧回放。测试中未观察到安全软件拦截。

**English:** On one new local account in a multi-account-directory environment running Weixin 4.1.15.56, `auto` login capture completed in about 33 seconds and HMAC-verified keys for both required databases (2/2). A fresh CLI `status` process loaded the keys from the DPAPI keyring and reported ready. Session lookup and export succeeded with WALs retained; this sample contained only one message. Both WALs validated and their stale-generation tails were excluded, but the current generation had zero committed frames, so this trial did not test replay of real committed WAL frames. No security-software block was observed during this trial.

## Configuration and diagnostic caveats / 配置与诊断注意事项

**中文：** 用户报告中，配置指向不对应的账号时捕获保持 0 个已验证密钥；这不能单独证明账号不匹配，也可能是登录时未捕获到可验证候选。只有捕获到密钥候选且其对所选账号数据库均未通过 HMAC 时，才能进一步诊断；若没有任何通过验证的候选，应使用中性提示，避免误导用户反复登录。

**English:** In the report, capture remained at zero verified keys when the configuration pointed to a different account directory. That observation alone does not prove an account mismatch; capture may have failed to observe a verifiable candidate at login. Diagnose a mismatch only after a key candidate is recovered and fails HMAC against all databases in the selected account. If no candidate verifies, use a neutral message that does not send the user through repeated logins on an unsupported assumption.

**中文：** 用户报告还记录 `init` 在微信运行时失败，并推测是 SQLite 锁；锁冲突并未被证实。随后主开发者与 Luna 用合成输入复现：包含共享拒绝的虚拟数据库目录仍可完成初始化；重复使用已存在的配置目标会触发独占创建（`O_EXCL`）失败，并返回通用 `OPERATION_FAILED`。这说明至少该复现中的通用错误来自配置目标冲突，而非 SQLite 锁；不证明用户报告中的失败原因相同。

**English:** The report also describes `init` failing while WeChat was running and hypothesizes a SQLite lock; that hypothesis was not confirmed. In a later synthetic reproduction by the primary developer and Luna, initialization succeeded with a dummy database directory marked share-denied. Reusing an existing config destination instead triggered exclusive-create (`O_EXCL`) failure and the generic `OPERATION_FAILED`. Thus the generic error in that reproduction came from a config-destination conflict, not a SQLite lock; this does not establish the cause of the user's original failure.

## Scope / 测试范围

**中文：** 本记录只覆盖用户报告的命令行流程和上述合成复现。未测试 MCP `serve`/`capture_key`、`read_only` 捕获、`decrypt`、多消息分片、群聊及其他消息类型、大数据量性能，或含当前代次已提交帧的加密 WAL。该单账号小样本不代表其他微信版本、账号、模型或宿主的兼容性。

**English:** This note covers only the user-reported CLI workflow and the synthetic reproduction above. It does not cover MCP `serve`/`capture_key`, `read_only` capture, `decrypt`, multiple message shards, group chats or other message types, large-dataset performance, or encrypted WALs containing committed frames of the current generation. This small single-account sample does not establish compatibility with other WeChat versions, accounts, models, or hosts.
