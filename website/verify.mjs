import assert from 'node:assert/strict';
import { readFile, stat } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), 'dist');
const manifest = JSON.parse(await readFile(path.join(root, 'downloads/latest.json'), 'utf8'));
assert.match(manifest.version, /^v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$/);
assert.equal(manifest.assets.length, 6);
assert.equal(new Set(manifest.assets.map(a => `${a.platform}-${a.arch}`)).size, 6);
const redirects = await readFile(path.join(root, '_redirects'), 'utf8');
const sums = await readFile(path.join(root, manifest.checksums_url), 'utf8');
for (const asset of manifest.assets) {
  assert.equal(asset.url, `/downloads/${manifest.version}/${asset.name}`);
  assert.match(asset.name, /^wxmori-v[\w.-]+-(windows|darwin|linux)-(amd64|arm64)\.zip$/);
  const bytes = await readFile(path.join(root, asset.url));
  assert.equal(bytes.length, asset.size);
  assert.ok(bytes.length <= 25 * 1024 * 1024);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), asset.sha256);
  assert.ok(sums.includes(`${asset.sha256}  ${asset.name}\n`));
  assert.ok(redirects.includes(`/download/${asset.platform}-${asset.arch} ${asset.url} 302`));
}
for (const [file, language] of [['index.html', 'zh-CN'], ['en/index.html', 'en'], ['releases/latest/index.html', 'zh-CN'], ['404.html', 'zh-CN']]) {
  const html = await readFile(path.join(root, file), 'utf8');
  assert.ok(html.includes(`<html lang="${language}">`));
  assert.doesNotMatch(html, /\sstyle=|<script(?![^>]*\bsrc=)/i, `${file}: CSP requires external scripts and styles`);
  const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map(m => m[1]);
  assert.equal(new Set(ids).size, ids.length, `${file}: duplicate IDs`);
  for (const [, target] of html.matchAll(/(?:href|src)="([^"]+)"/g)) {
    if (target.startsWith('#')) { assert.ok(ids.includes(target.slice(1)), `${file}: missing anchor ${target}`); continue; }
    if (!target.startsWith('/')) continue;
    const pathname = target.split('#')[0];
    if (pathname.startsWith('/download/')) continue;
    let local = path.join(root, pathname);
    if (pathname.endsWith('/')) local = path.join(local, 'index.html');
    assert.ok((await stat(local)).isFile(), `${file}: missing local asset ${target}`);
  }
  if (['index.html', 'en/index.html'].includes(file)) {
    assert.ok(html.includes(manifest.version));
    assert.ok(html.includes('<noscript>'));
    assert.ok(html.includes('hreflang="zh-CN"') && html.includes('hreflang="en"'));
    assert.match(html, /data-windows-download href="\/download\/windows-amd64"/);
  }
}
console.log('Passed / 通过: bilingual routes, local links, anchors, CSP, no-JS downloads, 6 ZIP sizes and SHA-256 checks');
