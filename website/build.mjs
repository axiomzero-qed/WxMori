import { createHash } from 'node:crypto';
import { cp, mkdir, readFile, writeFile, rm, rename, copyFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.join(root, 'dist');
const staging = path.join(root, '.dist-tmp');
const cache = path.join(root, '.release-cache');
const repository = 'axiomzero-qed/WxMori';
const api = `https://api.github.com/repos/${repository}/releases`;
const targets = ['windows-amd64', 'windows-arm64', 'darwin-arm64', 'darwin-amd64', 'linux-amd64', 'linux-arm64'];
const labels = { windows: 'Windows', darwin: 'macOS', linux: 'Linux' };
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const escape = text => String(text).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

async function request(url, json = false) {
  const headers = { 'User-Agent': 'WxMori-website', Accept: json ? 'application/vnd.github+json' : '*/*' };
  if (json && process.env.GH_TOKEN) headers.Authorization = `Bearer ${process.env.GH_TOKEN}`;
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const response = await fetch(url, { headers, signal: AbortSignal.timeout(120000) });
      if (!response.ok) throw new Error(`HTTP ${response.status}: ${new URL(url).pathname}`);
      return json ? await response.json() : Buffer.from(await response.arrayBuffer());
    } catch (error) {
      if (attempt === 2) throw error;
    }
  }
}

const selected = process.env.WXMORI_RELEASE;
if (selected && !/^v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$/.test(selected)) throw new Error('Invalid release tag');
// /latest excludes previews. Select the most recently published non-draft release instead.
const releases = selected ? [await request(`${api}/tags/${encodeURIComponent(selected)}`, true)] : await request(`${api}?per_page=100`, true);
const release = releases.filter(r => !r.draft && r.published_at).sort((a, b) => Date.parse(b.published_at) - Date.parse(a.published_at))[0];
if (!release || !/^v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$/.test(release.tag_name)) throw new Error('No valid published release');
const version = release.tag_name;
const prefix = `/downloads/${version}/`;
await mkdir(cache, { recursive: true });
// These two fixed generated directories are always children of this website checkout.
for (const target of [staging, output]) if (path.dirname(target) !== root) throw new Error('Unsafe build directory');
await rm(staging, { recursive: true, force: true });
await cp(path.join(root, 'public'), staging, { recursive: true });
const downloads = path.join(staging, 'downloads', version);
await mkdir(downloads, { recursive: true });

async function mirror(target) {
  const name = `wxmori-${version}-${target}.zip`;
  const asset = release.assets.find(a => a.name === name);
  const checksum = release.assets.find(a => a.name === `${name}.sha256`);
  if (!asset || !checksum || asset.state !== 'uploaded' || checksum.state !== 'uploaded') throw new Error(`Incomplete release: ${name}`);
  if (asset.size <= 0 || asset.size > 25 * 1024 * 1024) throw new Error(`Cloudflare static asset size limit: ${name}`);
  for (const file of [asset, checksum]) {
    const expected = `https://github.com/${repository}/releases/download/${version}/${file.name}`;
    if (file.browser_download_url !== expected) throw new Error(`Unexpected asset source: ${file.name}`);
  }
  const sidecar = (await request(checksum.browser_download_url)).toString('utf8').trim();
  const match = /^([a-fA-F0-9]{64})\s+\*?([^\r\n]+)$/.exec(sidecar);
  if (!match || match[2] !== name) throw new Error(`Invalid checksum: ${name}`);
  const sha256 = match[1].toLowerCase();
  if (asset.digest && asset.digest !== `sha256:${sha256}`) throw new Error(`GitHub digest mismatch: ${name}`);
  const cached = path.join(cache, name);
  let bytes;
  try { bytes = await readFile(cached); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  if (!bytes || bytes.length !== asset.size || hash(bytes) !== sha256) {
    bytes = await request(asset.browser_download_url);
    if (bytes.length !== asset.size || hash(bytes) !== sha256) throw new Error(`Download verification failed: ${name}`);
    await writeFile(cached, bytes);
  }
  await copyFile(cached, path.join(downloads, name));
  await writeFile(path.join(downloads, `${name}.sha256`), `${sha256}  ${name}\n`);
  const [platform, arch] = target.split('-');
  console.log(`Verified / 校验通过: ${name}`);
  return { name, url: `${prefix}${name}`, size: bytes.length, sha256, label: `${labels[platform]} ${arch === 'amd64' ? 'x64' : 'ARM64'}`, platform, arch };
}

// Limit simultaneous large downloads while still keeping the build short.
const assets = [];
for (let i = 0; i < targets.length; i += 3) assets.push(...await Promise.all(targets.slice(i, i + 3).map(mirror)));
const manifest = { version, prerelease: !!release.prerelease, published_at: release.published_at, release_url: release.html_url, release_notes_url: '/releases/latest/', assets, checksums_url: `${prefix}SHA256SUMS.txt` };
await writeFile(path.join(downloads, 'SHA256SUMS.txt'), assets.map(a => `${a.sha256}  ${a.name}\n`).join(''));
await writeFile(path.join(staging, 'downloads', 'latest.json'), `${JSON.stringify(manifest, null, 2)}\n`);
await mkdir(path.join(staging, 'docs'), { recursive: true });
for (const document of ['mcp', 'security', 'troubleshooting']) {
  await copyFile(path.join(root, '..', 'docs', `${document}.md`), path.join(staging, 'docs', `${document}.txt`));
}
for (const [page, language] of [['index.html', 'zh'], ['en/index.html', 'en']]) {
  const filename = path.join(staging, page);
  let html = await readFile(filename, 'utf8');
  html = html.replace(/(data-version>)[^<]*/g, `$1${escape(version)}`);
  for (const document of ['mcp', 'security', 'troubleshooting']) html = html.replaceAll(`https://github.com/${repository}/blob/main/docs/${document}.md`, `/docs/${document}.txt`);
  const heading = language === 'zh' ? '本站直连下载（无需 JavaScript）' : 'Direct downloads (no JavaScript required)';
  const fallback = `<noscript><div class="fallback"><p>${heading}</p><ul>${assets.map(a => `<li><a href="${a.url}">${escape(a.label)}</a></li>`).join('')}</ul><a href="${manifest.checksums_url}">SHA256SUMS.txt</a></div></noscript>`;
  html = html.replace('<div class="download-list"', `${fallback}<div class="download-list"`);
  await writeFile(filename, html);
}
await writeFile(path.join(staging, '_redirects'), [
  ...assets.map(a => `/download/${a.platform}-${a.arch} ${a.url} 302`),
  '/download /#downloads 302',
].join('\n') + '\n');
await writeFile(path.join(staging, '_headers'), `/*\n  X-Content-Type-Options: nosniff\n  Referrer-Policy: strict-origin-when-cross-origin\n  X-Frame-Options: DENY\n  Permissions-Policy: camera=(), microphone=(), geolocation=()\n  Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'none'\n/downloads/latest.json\n  Cache-Control: public, max-age=300, must-revalidate\n${prefix}*\n  Cache-Control: public, max-age=31536000, immutable\n  Content-Disposition: attachment\n`);

// A local copy of the published notes also works without GitHub access.
const notes = release.body || 'No release notes / 暂无发布说明';
await mkdir(path.join(staging, 'releases', 'latest'), { recursive: true });
await writeFile(path.join(staging, 'releases', 'latest', 'index.html'), `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${escape(version)} · WxMori 发布说明 / Release notes</title><meta name="description" content="WxMori ${escape(version)} release notes / 微忆发布说明"><link rel="stylesheet" href="/assets/notes.css"><link rel="icon" href="/assets/mark.svg"><link rel="canonical" href="https://wxmori.com/releases/latest/"></head><body><main><nav><a href="/">← WxMori 中文</a><a href="/en/">English home →</a></nav><p class="eyebrow">RELEASE NOTES / 发布说明</p><h1>${escape(version)}</h1><p>GitHub 原版发布说明的本站副本，下载包与原版 SHA-256 一致。<br>A local copy of the published notes. Downloads match the original SHA-256.</p><a class="button" href="/#downloads">下载 / Downloads</a><pre>${escape(notes)}</pre><a href="${escape(release.html_url)}">GitHub 原始发布页 / Original release</a></main></body></html>`);
await writeFile(path.join(staging, 'assets', 'notes.css'), `:root{color-scheme:light;font:16px/1.8 system-ui,-apple-system,"Segoe UI","Microsoft YaHei",sans-serif;color:#1b3730;background:#faf9f5}body{margin:0}main{max-width:860px;margin:auto;padding:48px 24px}nav{display:flex;justify-content:space-between;gap:20px}a{color:#256a50}h1{font-size:clamp(2.4rem,8vw,4rem);letter-spacing:-.04em;margin:10px 0}.eyebrow{margin-top:64px;font-size:.8rem;letter-spacing:.15em}pre{white-space:pre-wrap;overflow-wrap:anywhere;font:inherit;margin:36px 0;padding:28px;background:white;border:1px solid #dce5dc;border-radius:16px}.button{display:inline-block;background:#214f3c;color:white;padding:10px 22px;text-decoration:none;border-radius:30px}a:focus-visible{outline:3px solid #4a8a68;outline-offset:5px}`);
await writeFile(path.join(staging, 'robots.txt'), 'User-agent: *\nAllow: /\nSitemap: https://wxmori.com/sitemap.xml\n');
await writeFile(path.join(staging, 'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${['/', '/en/', '/releases/latest/'].map(p => `<url><loc>https://wxmori.com${p}</loc></url>`).join('')}</urlset>\n`);
await writeFile(path.join(staging, '404.html'), '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>404 · WxMori</title><link rel="stylesheet" href="/assets/notes.css"></head><body><main><h1>404</h1><p>这一页还没有记忆。 / Nothing here yet.</p><p><a href="/">返回首页</a> · <a href="/en/">English home</a></p></main></body></html>');
await rm(output, { recursive: true, force: true });
await rename(staging, output);
console.log(`Ready / 构建完成: ${version}, ${assets.length} verified archives / 个已验证下载包`);
