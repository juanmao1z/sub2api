/** @file @brief Verify the synchronized Kedaya review base and assembled production output. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { runtimeForPath } from './runtime/navigation.js';
import os from 'node:os';
import { assemble } from './assemble.mjs';

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

test('restored public page layout and styles retain their verified historical source', () => {
  const directory = path.resolve(root, '../public-pages');
  const provenance = JSON.parse(fs.readFileSync(path.join(directory, 'sources.json'), 'utf8'));
  assert.equal(provenance.commit, manifest.publicPages.sourceCommit);
  const adapted = new Set(['src/views/auth/LoginView.vue', 'src/components/auth/TotpLoginModal.vue']);
  for (const source of provenance.files) {
    if (!adapted.has(source.file)) assert.equal(sha256(path.join(directory, source.file)), source.sha256, source.file);
  }
  for (const file of ['src/views/HomeView.vue', 'src/components/layout/AuthLayout.vue', 'src/styles/public-pages.css', 'src/style.css']) {
    assert.ok(provenance.files.some(source => source.file === file), file);
  }
});

test('production output isolates the original public pages from the review console', () => {
  assert.equal(manifest.source, 'kedaya-review/public');
  assert.deepEqual(Object.keys(manifest.entries).sort(), ['native', 'review']);
  assert.equal(manifest.publicPages.sourceCommit, '81025b8e210e3ddb33a8e78a8aa02aba0c3abc80');
  for (const route of ['/', '/home', '/login']) assert.equal(runtimeForPath(route), 'native', route);
  for (const route of ['/dashboard', '/keys', '/payment/result', '/admin', '/admin/settings', '/privacy', '/terms']) {
    assert.equal(runtimeForPath(route), 'review', route);
  }
  assert.match(manifest.releasePrefix, /^\/integration\/[a-f0-9]{12}$/);
  assert.ok(review.script.startsWith(`${manifest.releasePrefix}/assets/`));
  assert.ok(review.styles.every(style => style.startsWith(`${manifest.releasePrefix}/`)));
  assert.ok(manifest.entries.native.script.startsWith(`${manifest.releasePrefix}/native/`));
  assert.ok(manifest.entries.native.styles.every(style => style.startsWith(`${manifest.releasePrefix}/native/`)));
  for (const entry of Object.values(manifest.entries)) {
    for (const file of [entry.script, ...entry.styles]) {
      assert.ok(fs.existsSync(outputPath(file)), file);
      const asset = manifest.publishedAssets.find(asset => asset.file === file);
      assert.ok(asset, file);
      assert.equal(sha256(outputPath(file)), asset.sha256, file);
    }
  }
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

test('channel status card uses selected-window aggregate metrics and health', () => {
  const temporaryOutput = fs.mkdtempSync(path.join(os.tmpdir(), 'sub2api-channel-status-'));
  try {
    const assembled = assemble({ outputDir: temporaryOutput });
    const source = fs.readFileSync(path.join(temporaryOutput, 'assets/ChannelStatusView-DNoLdA9d.js'), 'utf8');
    assert.ok(source.includes(',_=v(()=>a.row.metrics),g=v(()=>a.row.health)'));
    assert.doesNotMatch(source, /\(\(h=f\.value\).*?a\.row\.metrics\)/);
    assert.doesNotMatch(source, /\(\(h=f\.value\).*?a\.row\.health\)/);
    assert.ok(assembled.changes.some(change => change.file === 'assets/ChannelStatusView-DNoLdA9d.js'));
    const statusStyles = fs.readFileSync(path.join(temporaryOutput, 'assets/ChannelStatusView-DV2xhrMN.css'), 'utf8');
    assert.match(statusStyles, /\.health-operational\[data-v-160eba47\].*background:#16a34a/);
    assert.match(statusStyles, /\.health-degraded\[data-v-160eba47\].*background:#eab308/);
    assert.match(statusStyles, /\.health-failed\[data-v-160eba47\].*background:#dc2626/);
    assert.match(statusStyles, /\.health-error\[data-v-160eba47\].*background:#dc2626/);
    assert.match(statusStyles, /\.health-unknown\[data-v-160eba47\]\{background:#9ca3af\}/);
    assert.ok(source.includes('const C={healthy:{colorClass:"is-healthy",heightPct:100},warning:{colorClass:"is-warning",heightPct:65},critical:{colorClass:"is-critical",heightPct:35},unknown:{colorClass:"is-unknown",heightPct:15}};'));
    assert.ok(source.includes('A=F==="unknown"?{...C.unknown}:{...C[F]}'));
    assert.doesNotMatch(source, /colorClass:Pn\(W\)/);
    assert.match(statusStyles, /\.v3-soft-glass-bar\.is-healthy\[data-v-fa54021f\]\{background:#22c55e\}/);
    assert.match(statusStyles, /\.v3-soft-glass-bar\.is-warning\[data-v-fa54021f\]\{background:#eab308\}/);
    assert.match(statusStyles, /\.v3-soft-glass-bar\.is-critical\[data-v-fa54021f\]\{background:#ef4444\}/);
    assert.match(statusStyles, /\.v3-soft-glass-bar\.is-unknown\[data-v-fa54021f\]\{background:#6b7280\}/);
  } finally {
    fs.rmSync(temporaryOutput, { recursive: true, force: true });
  }
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

test('guide aliases and owned canvas application are published with matching digests', () => {
  const entry = fs.readFileSync(outputPath(review.script), 'utf8');
  assert.ok(entry.includes('path:"/usage-guide"'));
  assert.ok(entry.includes('"/custom/cc-switch-guide"'));
  assert.ok(entry.includes('import("./usage-guide.js")'));
  for (const file of ['/canvas/index.html', '/canvas/canvas.js', '/canvas/canvas-core.mjs', '/canvas/canvas.css', `${manifest.releasePrefix}/assets/usage-guide.js`]) {
    const asset = manifest.publishedAssets.find(item => item.file === file);
    assert.ok(asset, file);
    assert.equal(sha256(outputPath(file)), asset.sha256, file);
  }
  const canvas = fs.readFileSync(outputPath('/canvas/index.html'), 'utf8');
  assert.ok(canvas.includes('src="./canvas.js"'));
  assert.ok(!canvas.includes('/bootstrap.js'));
});

test('Stripe payments bind secrets to the local order snapshot', () => {
  const stripe = fs.readFileSync(outputPath(`${manifest.releasePrefix}/assets/StripePaymentView-BaTYfbsS.js`), 'utf8');
  const payment = fs.readFileSync(outputPath(`${manifest.releasePrefix}/assets/PaymentView-Bu7KA8D-.js`), 'utf8');
  assert.ok(stripe.includes('restored?.orderId===a?restored.clientSecret:""'));
  assert.ok(!stripe.includes('m.query.client_secret'));
  assert.doesNotMatch(payment, /client_secret:[M$]\.client_secret,/);
});
