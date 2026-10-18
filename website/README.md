# WxMori website / 官网维护

## 中文

使用 Node.js 22 或更新版本，无前端依赖。运行 `node build.mjs` 会从 GitHub 最新公开发布（包含 prerelease）获取六个 ZIP，并校验 SHA-256；随后运行 `node verify.mjs` 检查构建产物。用 `node preview.mjs` 启动本地预览，地址为 `http://127.0.0.1:4175`。

Cloudflare 使用 Workers Static Assets。部署前需要先登录 Wrangler，再运行 `npx wrangler@4.151.1 deploy --config wrangler.jsonc`。目前没有配置自动部署；每次发布新版本后都要重新构建并部署，网站才会更新。不要提交 `dist/`、`.release-cache/`、`deployment/` 或 `.wrangler/`。

网站根路径 `/` 为中文，`/en/` 为英文。稳定下载路由为 `/download/windows-amd64`，最新发布元数据为 `/downloads/latest.json`。`/docs/` 提供本站纯文本 MCP、安全与故障排查说明；压缩包内也包含完整文档。`WXMORI_RELEASE` 可指定已发布标签；不设置时选择最新公开发布。线上部署和 DNS 由仓库维护者核验。

## English

Use Node.js 22 or later; the frontend has no dependencies. Run `node build.mjs` to fetch the six ZIP archives from the latest public GitHub release, including prereleases, and verify their SHA-256 hashes. Then run `node verify.mjs` to check the build output. Start a local preview with `node preview.mjs` at `http://127.0.0.1:4175`.

Cloudflare deployment uses Workers Static Assets. Sign in to Wrangler first, then run `npx wrangler@4.151.1 deploy --config wrangler.jsonc`. Automatic deployment is not configured. Rebuild and deploy after each new release to update the site. Do not commit `dist/`, `.release-cache/`, `deployment/`, or `.wrangler/`.

The root path `/` serves Chinese and `/en/` serves English. The stable download route is `/download/windows-amd64`; release metadata is at `/downloads/latest.json`. `/docs/` serves plain-text MCP, security, and troubleshooting documentation without GitHub access; the archives also include full documentation. Set `WXMORI_RELEASE` to pin a published tag, or omit it to select the newest public release. The repository maintainer handles production and DNS verification.
