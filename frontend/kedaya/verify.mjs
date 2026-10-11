/** @file @brief Verify the synchronized Kedaya review base and assembled production output. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { runtimeForPath } from './runtime/navigation.js';

const root = path.dirname(fileURLToPath(import.meta.url));
const base = path.join(root, 'base');
const sourceManifest = JSON.parse(fs.readFileSync(path.join(root, 'base-manifest.json'), 'utf8'));
const output = path.resolve(root, '../../backend/internal/web/dist');
const manifestPath = path.join(output, 'integration/build-manifest.json');
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
const review = manifest.entries.review;

function outputPath(file) {
  return path.join(output, file.replace(/^\/+/, ''));
}

function sha256(file) {
  return createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

test('base is sourced from the compiled review asset graph', () => {
  const pinned = sourceManifest.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'));
  assert.equal(pinned.length, 196);
  for (const asset of pinned) {
    const file = path.join(base, asset.path.slice(1));
    assert.ok(fs.existsSync(file), asset.path);
    assert.equal(sha256(file), asset.sha256, asset.path);
  }
});

test('production output contains only the review application entry', () => {
  assert.equal(manifest.source, 'kedaya-review/public');
  assert.deepEqual(Object.keys(manifest.entries), ['review']);
  assert.equal(runtimeForPath('/'), 'review');
  for (const route of ['/home', '/login', '/dashboard', '/keys', '/payment/result', '/admin', '/admin/settings', '/privacy', '/terms']) {
    assert.equal(runtimeForPath(route), 'review', route);
  }
  assert.match(manifest.releasePrefix, /^\/integration\/[a-f0-9]{12}$/);
  assert.ok(review.script.startsWith(`${manifest.releasePrefix}/assets/`));
  assert.ok(review.styles.every(style => style.startsWith(`${manifest.releasePrefix}/`)));
});

test('generated shell is valid and has no review-only session bootstrap', () => {
  const html = fs.readFileSync(path.join(output, 'index.html'), 'utf8');
  assert.ok(html.includes(`${manifest.releasePrefix}/bootstrap.js`));
  assert.ok(html.includes("localStorage.getItem('theme')"));
  assert.ok(html.includes('window.__APP_CONFIG__'));
  assert.ok(!html.includes('/review/session'));
  assert.ok(!html.includes('LOCAL_DEMO'));
  assert.ok(!html.includes('demoUser'));
});

test('compiled lazy imports resolve inside the release directory', () => {
  for (const asset of manifest.publishedAssets.filter(item => item.file.endsWith('.js'))) {
    const sourcePath = outputPath(asset.file);
    const source = fs.readFileSync(sourcePath, 'utf8');
    for (const match of source.matchAll(/(?:from\s*|import\s*\()(["'])(\.[^"']+)\1/g)) {
      assert.ok(fs.existsSync(path.resolve(path.dirname(sourcePath), match[2])), `${asset.file} -> ${match[2]}`);
    }
  }
  assert.ok(fs.existsSync(outputPath(`${manifest.releasePrefix}/bootstrap.js`)));
  assert.ok(fs.existsSync(outputPath(`${manifest.releasePrefix}/entries.js`)));
});

test('production adaptations preserve real Sub2API contracts', () => {
  const entry = fs.readFileSync(outputPath('/assets/index-CHmaXGoD.js'), 'utf8');
  const keys = fs.readFileSync(outputPath('/assets/KeysView-C1R_JMh0.js'), 'utf8');
  const layout = fs.readFileSync(outputPath('/assets/AppLayout.vue_vue_type_script_setup_true_lang-C4ysyc_b.js'), 'utf8');
  const provider = fs.readFileSync(outputPath('/assets/providerConfig-wYxXJe_k.js'), 'utf8');
  assert.ok(entry.includes('/logo.png?v=20260715'));
  assert.ok(keys.includes('Da.get("/settings/public")'));
  assert.ok(keys.includes('y.custom_endpoints||[]'));
  assert.ok(layout.includes('subscription_enabled'));
  assert.ok(layout.includes('Wei-Shaw/sub2api'));
  assert.ok(layout.includes('ghcr.io/wei-shaw/sub2api'));
  assert.ok(provider.includes('defaultValue:""'));
  assert.ok(!provider.includes('ep.kedaya.xyz'));
});

test('published assets contain no captured-site or review-only markers', () => {
  const files = [
    path.join(output, 'index.html'),
    ...manifest.publishedAssets.map(asset => outputPath(asset.file)),
    outputPath(`${manifest.releasePrefix}/bootstrap.js`),
    outputPath(`${manifest.releasePrefix}/entries.js`),
  ];
  for (const file of files) {
    const source = fs.readFileSync(file, 'utf8');
    for (const pattern of [/kedaya\.ai/i, /review\.local/i, /LOCAL_DEMO/i, /\/review\/session/i, /demoUser/i]) {
      assert.doesNotMatch(source, pattern, file);
    }
  }
});
