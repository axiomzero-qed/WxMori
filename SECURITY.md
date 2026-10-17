# Security policy / 安全策略

## Supported versions / 支持版本

**中文：** 当前候选版 `v0.2.0-alpha.1` 的 Windows 登录时捕获仍属实验性。一次 Weixin 4.1.15.56 单账号 Luna/Python stdio 试验成功捕获并验证 12/12 核心数据库密钥，并通过读取与导出；默认调试器方式会短暂暂停线程并临时调整、恢复调试寄存器，`read_only` 方式轮询可读内存。当前代次 WAL 提交帧实测为 0，提交帧回放只由合成测试覆盖。其他微信版本和宿主未验证。报告问题时请注明 WxMori 和微信的确切版本。

**English:** The current candidate `v0.2.0-alpha.1` has experimental Windows login-time capture. One Luna/Python stdio trial on a single Weixin 4.1.15.56 account captured and verified keys for all 12 core databases, then completed reads and exports. The default debugger mode briefly pauses threads and temporarily adjusts then restores debug registers; `read_only` polls readable memory. The sampled WALs had zero current-generation commit frames, so committed-frame replay is covered only by synthetic tests. Other WeChat versions and hosts are untested. Include exact WxMori and WeChat versions when reporting issues.

## Reporting / 报告问题

**中文：** 请勿通过公开 issue 披露未修复的安全问题，也不要附上密钥、真实数据库、聊天记录或个人信息。若 GitHub 仓库启用了私密安全报告功能，请通过该功能提交；否则请联系维护者并先提供不含敏感数据的概要。当前未承诺具体响应时限。

**English:** Do not disclose an unfixed security issue in a public issue or attach keys, real databases, chat logs, or personal information. If private security reporting is enabled for the GitHub repository, use it; otherwise contact the maintainer with a summary that contains no sensitive data. No specific response time is promised.

**中文：** 报告应包含受影响版本、影响范围、最小复现步骤和建议修复方式。请勿复制密钥或消息正文；可提供脱敏日志和错误代码。

**English:** Include the affected version, impact, minimal reproduction steps, and a suggested fix. Do not copy keys or message bodies; redacted logs and error codes are useful.
