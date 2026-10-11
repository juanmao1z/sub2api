/** @file @brief Verify the assembled Sub2API review UI release contract. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { installNavigation, runtimeForPath } from './runtime/navigation.js';
import { Script } from 'node:vm';

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, '../../backend/internal/web/dist');
const manifestPath = path.join(output, 'integration/build-manifest.json');
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
const reference = JSON.parse(fs.readFileSync(path.join(root, 'reference-manifest.json'), 'utf8'));
const review = manifest.entries.review;

function outputPath(file) {
  return path.join(output, file.replace(/^\/+/, ''));
}

function sha256(file) {
  return createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

function assetBySuffix(suffix) {
  const asset = manifest.publishedAssets.find(item => item.file.endsWith(suffix));
  assert.ok(asset, `published asset ${suffix}`);
  return asset;
}

test('generated inline startup scripts are valid JavaScript', () => {
  const html = fs.readFileSync(path.join(output, 'index.html'), 'utf8');
  const scripts = [...html.matchAll(/<script nonce="[^"]*">([\s\S]*?)<\/script>/g)];
  assert.equal(scripts.length, 2);
  for (const [, code] of scripts) assert.doesNotThrow(() => new Script(code));
});

test('production manifest publishes one review entry for every route family', () => {
  assert.deepEqual(Object.keys(manifest.entries), ['review']);
  assert.equal(runtimeForPath('/'), 'review');
  for (const route of [
    '/home', '/login', '/register', '/forgot-password', '/reset-password',
    '/dashboard', '/keys', '/usage', '/recharge', '/purchase', '/payment/result',
    '/batch-image', '/infinite-canvas', '/tickets', '/admin', '/admin/users',
    '/admin/settings', '/privacy', '/terms', '/custom/cc-switch-guide',
  ]) assert.equal(runtimeForPath(route), 'review', route);
  assert.equal(review.script.startsWith(`${manifest.releasePrefix}/assets/`), true);
  assert.ok(review.styles.every(style => style.startsWith(`${manifest.releasePrefix}/`)));
});

test('generated production entry has themed startup shell and no review bootstrap', () => {
  const html = fs.readFileSync(path.join(output, 'index.html'), 'utf8');
  assert.ok(html.includes(`${manifest.releasePrefix}/bootstrap.js`));
  assert.ok(html.includes("localStorage.getItem('theme')"));
  assert.ok(html.includes('color-scheme: dark'));
  assert.ok(html.includes('startup-loading'));
  assert.ok(html.includes('startup-loading-brand'));
  assert.ok(html.includes('startup-loading-label'));
  assert.ok(html.includes('window.__APP_CONFIG__'));
  assert.ok(html.includes('__CSP_NONCE_VALUE__'));
  assert.ok(!html.includes('/review/session'));
  assert.ok(!html.includes('LOCAL_DEMO'));
  assert.ok(!html.includes('demoUser'));
});

test('formal Sub2API home assets retain their declared hashes', () => {
  assert.deepEqual(manifest.homeAssets.map(asset => asset.file).sort(), ['/logo.png', '/logo.svg']);
  for (const asset of manifest.homeAssets) assert.equal(sha256(outputPath(asset.file)), asset.sha256, asset.file);
});

test('all pinned compiled assets are published under the immutable release directory', () => {
  const pinned = reference.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'));
  assert.equal(pinned.length, 196);
  assert.equal(manifest.publishedAssets.length, pinned.length);
  assert.match(manifest.releasePrefix, /^\/integration\/[a-f0-9]{12}$/);
  for (const asset of manifest.publishedAssets) {
    assert.ok(asset.file.startsWith(`${manifest.releasePrefix}/`), asset.file);
    const file = outputPath(asset.file);
    assert.ok(fs.existsSync(file), asset.file);
    assert.equal(sha256(file), asset.sha256, asset.file);
  }
  for (const file of [review.script, ...review.styles, `${manifest.releasePrefix}/bootstrap.js`, `${manifest.releasePrefix}/entries.js`]) {
    assert.ok(fs.existsSync(outputPath(file)), file);
  }
});

test('compiled lazy imports resolve inside the published release directory', () => {
  const jsAssets = manifest.publishedAssets.filter(asset => asset.file.endsWith('.js'));
  for (const asset of jsAssets) {
    const sourcePath = outputPath(asset.file);
    const source = fs.readFileSync(sourcePath, 'utf8');
    for (const match of source.matchAll(/(?:from\s*|import\s*\()(["'])(\.[^"']+)\1/g)) {
      const target = path.resolve(path.dirname(sourcePath), match[2]);
      assert.ok(fs.existsSync(target), `${asset.file} -> ${match[2]}`);
    }
  }
  const entry = fs.readFileSync(outputPath(review.script), 'utf8');
  assert.ok(entry.includes(`Fa=function(e){return"${manifest.releasePrefix}/"+e`));
});

test('production adaptations preserve real Sub2API contracts', () => {
  const entry = fs.readFileSync(outputPath(assetBySuffix('/assets/index-CHmaXGoD.js').file), 'utf8');
  const keys = fs.readFileSync(outputPath(assetBySuffix('/assets/KeysView-C1R_JMh0.js').file), 'utf8');
  const layout = fs.readFileSync(outputPath(assetBySuffix('/assets/AppLayout.vue_vue_type_script_setup_true_lang-C4ysyc_b.js').file), 'utf8');
  const provider = fs.readFileSync(outputPath(assetBySuffix('/assets/providerConfig-wYxXJe_k.js').file), 'utf8');
  assert.ok(entry.includes('/logo.png?v=20260715'));
  assert.ok(keys.includes('Da.get("/settings/public")'));
  assert.ok(keys.includes('y.custom_endpoints||[]'));
  assert.ok(layout.includes('subscription_enabled'));
  assert.ok(layout.includes('Wei-Shaw/sub2api'));
  assert.ok(layout.includes('ghcr.io/wei-shaw/sub2api'));
  assert.ok(provider.includes('defaultValue:""'));
  assert.ok(!provider.includes('ep.kedaya.xyz'));
});

test('navigation keeps application transitions client-side and rewrites captured origin links', () => {
  const calls = [];
  const listeners = {};
  const win = {
    location: {
      origin: 'http://127.0.0.1:4175',
      href: 'http://127.0.0.1:4175/dashboard',
      pathname: '/dashboard',
      protocol: 'http:',
      host: '127.0.0.1:4175',
      assign: url => calls.push(['assign', url]),
    },
    history: {
      pushState: (...args) => calls.push(['pushState', ...args]),
      replaceState: (...args) => calls.push(['replaceState', ...args]),
    },
    addEventListener: (name, callback) => { listeners[name] = callback; },
    document: { addEventListener() {} },
  };
  installNavigation(win);
  win.history.pushState({}, '', '/admin/users');
  assert.deepEqual(calls[0], ['pushState', {}, '', '/admin/users']);
  win.history.pushState({}, '', 'https://api.zhouz.online/payment/result');
  assert.deepEqual(calls[1], ['pushState', {}, '', 'http://127.0.0.1:4175/payment/result']);
  assert.equal(typeof listeners.popstate, 'undefined');
});

test('published assets contain no captured Kedaya or review-only runtime markers', () => {
  const files = [
    path.join(output, 'index.html'),
    ...manifest.publishedAssets.map(asset => outputPath(asset.file)),
    outputPath(`${manifest.releasePrefix}/bootstrap.js`),
    outputPath(`${manifest.releasePrefix}/entries.js`),
  ];
  const forbidden = [/kedaya/i, /review\.local/i, /LOCAL_DEMO/i, /\/review\/session/i, /demoUser/i];
  for (const file of files) {
    const source = fs.readFileSync(file, 'utf8');
    for (const pattern of forbidden) assert.doesNotMatch(source, pattern, file);
  }
});

test('review runtime loads its generated descriptors without preview session initialization', () => {
  const bootstrap = fs.readFileSync(outputPath(`${manifest.releasePrefix}/bootstrap.js`), 'utf8');
  const entries = fs.readFileSync(outputPath(`${manifest.releasePrefix}/entries.js`), 'utf8');
  assert.ok(bootstrap.includes("import { entries } from './entries.js'"));
  assert.ok(bootstrap.includes('entries[kind].styles'));
  assert.ok(bootstrap.includes('import(entries[kind].script)'));
  assert.ok(!bootstrap.includes('localStorage.setItem'));
  assert.ok(!bootstrap.includes('/review/session'));
  assert.ok(entries.includes(manifest.releasePrefix));
});
