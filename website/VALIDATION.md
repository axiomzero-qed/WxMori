# Website validation / 官网验证

2026-10-18（Asia/Shanghai）

- 中文：无依赖构建及产物检查通过，覆盖双语路由、站内链接、锚点、CSP、无 JavaScript 下载入口，以及六个 ZIP 的大小与 SHA-256。发布为 `v0.2.0-alpha.2`，包含预览版；原始发布文件和校验侧文件分别核对，若 GitHub 提供摘要也须一致。
- English: Dependency-free build and output checks passed for bilingual routes, local links, anchors, CSP, no-JavaScript download routes, and sizes/SHA-256 of all six ZIP archives. The mirrored release is `v0.2.0-alpha.2`, including previews. Archives are verified against their checksum sidecars and GitHub-provided digests when available.

- 中文：Edge 桌面实际检查中英文切换、六项下载列表、哈希复制与 FAQ 展开；未发现控制台错误。320px 与 390px 视口检查未发现整页横向溢出，检查后恢复正常视口。
- English: Edge desktop checks covered language switching, six download rows, hash copying, and FAQ expansion, with no observed console errors. DOM measurements at 320px and 390px found no document-level horizontal overflow; the viewport was reset afterwards.

- 中文：从实际部署的 Cloudflare 网站下载 Windows x64 ZIP，大小 10,878,218 字节，SHA-256 为 `7eecbdda76bbb482a89d01d44ecc155fe8e6a5dfaa1891dc1d4264ede7a9b7c1`；与 GitHub 原版一致，ZIP 完整性检查通过，共 64 个文件。
- English: The Windows x64 ZIP was downloaded from the deployed Cloudflare site: 10,878,218 bytes, SHA-256 `7eecbdda76bbb482a89d01d44ecc155fe8e6a5dfaa1891dc1d4264ede7a9b7c1`, matching the GitHub original. ZIP integrity passed for all 64 entries.

- 中文：仓库 `go test ./...`、`go vet ./...` 通过；另行构建后，独立 Python stdio 冒烟在四个 MCP 协议版本下均发现九个工具并通过。本次未修改 Go 运行时，也未对真实聊天进行读取或捕获。
- English: Repository Go tests and vet passed. After a separate build, the independent Python stdio smoke passed with nine tools across four MCP protocol versions. No Go runtime changes, real-chat reads, or key capture were part of this website work.

- 中文：已部署 Cloudflare Workers Static Assets 并绑定 `wxmori.com` 与 `www.wxmori.com`；注册商已保存 Cloudflare 名称服务器，Cloudflare API 确认 zone 为 active。正式域名已通过正常 HTTPS 证书验证，浏览器实际切换中英文并显示六项下载列表，且从正式域名下载 Windows x64 ZIP 并再次校验通过。完整本地部署记录及截图保存在忽略目录 `deployment/`。
- English: Deployed to Cloudflare Workers Static Assets with `wxmori.com` and `www.wxmori.com` bound. The registrar saved Cloudflare nameservers and the Cloudflare API reported the zone active. The custom domain passed normal HTTPS certificate verification. Browser checks confirmed both languages and six download rows; the Windows x64 ZIP was downloaded from the custom domain and verified again. Local deployment evidence and screenshots are in the ignored `deployment/` directory.
