# Development guidance / 开发约定

- Keep project explanations in paired Chinese and English; preserve original third-party licenses. / 说明文件保持中英对照，第三方许可证保留原文。
- Delegate simple documentation and actual MCP calling trials to GPT-6 Luna when available. Keep sensitive implementation and final review with the primary agent. If Luna is unavailable, disclose that rather than claiming a trial. / 可用时把简单文档和实际 MCP 调用测试交给 GPT-6 Luna；关键实现和最终检查由主代理负责。模型不可用时如实说明。
- Keep runtime independent of model vendors and hosts, using the official MCP SDK and flat schemas. / 运行时不依赖模型厂商或特定宿主，使用官方 MCP SDK 和扁平参数。
- Never commit private data, keys, databases, exports, personal machine paths, or WeFlow code/binaries. / 禁止提交真实聊天、密钥、数据库、导出、个人机器路径或 WeFlow 代码与二进制。
- Authenticate every encrypted page and verify SQLite integrity. Never silently ignore WAL or undecodable messages. / 验证每个加密页和 SQLite 完整性，不静默忽略 WAL 或解码失败的消息。
- Run Go tests, go vet, and the independent MCP smoke test after behavior changes. Record model and host tests separately. / 行为修改后运行 Go 测试、静态检查和独立 MCP 测试，模型与宿主实测分开记录。
