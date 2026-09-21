/** @file @brief Verify assembled assets and navigation across the preserved homepage boundary. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { installNavigation, runtimeForPath } from './runtime/navigation.js';

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, '../../backend/internal/web/dist');
const manifest = JSON.parse(fs.readFileSync(path.join(output, 'integration/build-manifest.json'), 'utf8'));
const reference = JSON.parse(fs.readFileSync(path.join(root, 'reference-manifest.json'), 'utf8'));

test('original homepage compiled assets retain their pre-assembly bytes', () => {
  for (const asset of manifest.homeAssets) {
    const digest = createHash('sha256').update(fs.readFileSync(path.join(output, asset.file))).digest('hex');
    assert.equal(digest, asset.sha256, asset.file);
  }
  assert.equal(manifest.homeAssets.length, 5);
});

test('Kedaya assets match the pinned release except for documented API adaptations', () => {
  const patched = new Map(manifest.changes.map(change => [change.file, change]));
  for (const asset of reference.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'))) {
    const file = asset.path.endsWith('/') ? asset.path.slice(1) + 'index.html' : asset.path.slice(1);
    const digest = createHash('sha256').update(fs.readFileSync(path.join(output, file))).digest('hex');
    assert.equal(digest, patched.get(file)?.adaptedHash || asset.sha256, file);
  }
});

test('homepage transitions reload the document and console transitions remain client-side', () => {
  const calls = [], listeners = {};
  const win = { location: { origin: 'https://api.zhouz.online', href: 'https://api.zhouz.online/dashboard', pathname: '/dashboard', assign: url => calls.push(['assign', url]), replace: url => calls.push(['replace', url]), reload: () => calls.push(['reload']) }, history: { pushState: (...args) => calls.push(['pushState', ...args]), replaceState() {} }, addEventListener: (name, callback) => { listeners[name] = callback; }, document: { addEventListener() {} } };
  installNavigation(win);
  win.history.pushState({}, '', '/keys');
  win.history.pushState({}, '', '/home');
  assert.equal(calls[0][0], 'pushState');
  assert.deepEqual(calls[1], ['assign', 'https://api.zhouz.online/home']);
  win.location.pathname = '/home';
  listeners.popstate();
  assert.deepEqual(calls[2], ['reload']);
});

test('production entry has isolated styles and no preview account initialization', () => {
  assert.ok(!manifest.entries.home.styles.some(style => manifest.entries.console.styles.includes(style)));
  const html = fs.readFileSync(path.join(output, 'index.html'), 'utf8');
  assert.ok(html.includes(manifest.releasePrefix + '/bootstrap.js'));
  assert.ok(!html.includes('preview/session'));
  assert.ok(!html.includes('auth_token'));
  assert.ok(!html.includes('__APP_CONFIG__='));
});

test('site ticket and custom pages use their complete native implementation', () => {
  for (const route of ['/tickets', '/tickets/', '/admin/tickets', '/custom/cc-switch-guide', '/custom/usage-leaderboard']) assert.equal(runtimeForPath(route), 'native', route);
  assert.equal(runtimeForPath('/home'), 'home');
  assert.equal(runtimeForPath('/dashboard'), 'console');
  assert.equal(runtimeForPath('/admin/users'), 'console');
  assert.equal(manifest.entries.native.script, manifest.entries.home.script);
  const calls = [];
  const win = { location: { origin: 'https://api.zhouz.online', href: 'https://api.zhouz.online/tickets', pathname: '/tickets', assign: url => calls.push(['assign', url]) }, history: { pushState: (...args) => calls.push(['pushState', ...args]), replaceState() {} }, addEventListener() {}, document: { addEventListener() {} } };
  installNavigation(win);
  win.history.pushState({}, '', '/custom/cc-switch-guide');
  win.history.pushState({}, '', '/dashboard');
  assert.equal(calls[0][0], 'pushState');
  assert.deepEqual(calls[1], ['assign', 'https://api.zhouz.online/dashboard']);
});

test('published console imports and styles use an immutable release directory', () => {
  assert.match(manifest.releasePrefix, /^\/integration\/[a-f0-9]{12}$/);
  assert.ok(manifest.entries.console.script.startsWith(manifest.releasePrefix + '/assets/'));
  for (const asset of manifest.publishedAssets) {
    assert.ok(asset.file.startsWith(manifest.releasePrefix + '/assets/'));
    const digest = createHash('sha256').update(fs.readFileSync(path.join(output, asset.file))).digest('hex');
    assert.equal(digest, asset.sha256, asset.file);
  }
  for (const entry of Object.values(manifest.entries)) for (const style of entry.styles) assert.ok(fs.existsSync(path.join(output, style)), style);
  const consoleCode = fs.readFileSync(path.join(output, manifest.entries.console.script), 'utf8');
  assert.ok(consoleCode.includes(`Va=function(e){return"${manifest.releasePrefix}/"+e}`));
});
