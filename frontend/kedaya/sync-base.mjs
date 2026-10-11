/** @file @brief Synchronize the compiled Kedaya review base into this repository. */
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(scriptDir, '../..');
const workspaceRoot = path.resolve(projectRoot, '..');
const sourceRoot = path.resolve(process.env.KEDAYA_REVIEW_ROOT || path.join(workspaceRoot, 'kedaya-review'));
const sourcePublic = path.join(sourceRoot, 'public');
const sourceManifestPath = path.join(sourceRoot, 'evidence', 'asset-manifest.json');
const targetBase = path.join(projectRoot, 'frontend', 'kedaya', 'base');
const targetManifest = path.join(projectRoot, 'frontend', 'kedaya', 'base-manifest.json');

function sha256(file) {
  return createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

function copy(source, target) {
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.copyFileSync(source, target);
}

if (!fs.existsSync(sourcePublic) || !fs.existsSync(sourceManifestPath)) {
  throw new Error(`Kedaya review source not found. Set KEDAYA_REVIEW_ROOT to the review project: ${sourceRoot}`);
}

const manifest = JSON.parse(fs.readFileSync(sourceManifestPath, 'utf8'));
const compiledAssets = manifest.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'));

fs.rmSync(targetBase, { recursive: true, force: true });
fs.mkdirSync(targetBase, { recursive: true });
for (const asset of compiledAssets) {
  const source = path.join(sourcePublic, asset.path.slice(1));
  if (!fs.existsSync(source)) throw new Error(`Missing review asset: ${asset.path}`);
  if (sha256(source) !== asset.sha256) throw new Error(`Review asset checksum mismatch: ${asset.path}`);
  copy(source, path.join(targetBase, asset.path.slice(1)));
}
copy(path.join(sourcePublic, 'index.original.html'), path.join(targetBase, 'index.html'));
if (fs.existsSync(path.join(sourcePublic, 'logo.svg'))) copy(path.join(sourcePublic, 'logo.svg'), path.join(targetBase, 'logo.svg'));
copy(sourceManifestPath, targetManifest);

const missing = compiledAssets.filter(asset => !fs.existsSync(path.join(targetBase, asset.path.slice(1))));
if (missing.length) throw new Error(`Base synchronization incomplete: ${missing.map(asset => asset.path).join(', ')}`);
console.log(`Synchronized ${compiledAssets.length} compiled assets from ${sourceRoot}`);
console.log(`Target: ${path.relative(workspaceRoot, targetBase)}`);
