/** @file @brief Assemble the original homepage and the pinned Kedaya console for production. */
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.dirname(fileURLToPath(import.meta.url));

/** @brief Calculate the exact file digest. @param file Path to a file. @return SHA-256 hex. */
function hash(file) { return createHash('sha256').update(fs.readFileSync(file)).digest('hex'); }

/** @brief Read Vite's generated module and stylesheet entry points.
 * @param directory Build directory. @return Module and stylesheet paths.
 * @throws Error if the build has no valid module entry or global stylesheet.
 */
function readEntry(directory) {
  const html = fs.readFileSync(path.join(directory, 'index.html'), 'utf8');
  const script = html.match(/<script[^>]+type="module"[^>]+src="([^"]+)"/)?.[1];
  const styles = [...html.matchAll(/<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"/g)].map(match => match[1]);
  if (!script || !styles.length) throw new Error(`Expected a fresh Vite build in ${directory}`);
  return { script, styles };
}

/** @brief Produce a deployable frontend while preserving the source homepage's compiled bytes.
 * @param options.homeDir Fresh output of the site's existing Vite build.
 * @param options.outputDir Optional separate output; defaults to updating homeDir in place.
 * @return Entry descriptors, homepage hashes, and compatibility changes.
 * @throws Error for asset collisions, changed vendor files, or unexpected patch anchors.
 */
export function assemble({ homeDir, outputDir = homeDir }) {
  homeDir = path.resolve(homeDir);
  outputDir = path.resolve(outputDir);
  const reference = path.join(root, 'vendor');
  const entries = { home: readEntry(homeDir), console: readEntry(reference) };
  const referenceManifest = JSON.parse(fs.readFileSync(path.join(root, 'reference-manifest.json'), 'utf8'));
  for (const asset of referenceManifest.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'))) {
    const relative = asset.path.endsWith('/') ? asset.path.slice(1) + 'index.html' : asset.path.slice(1);
    if (hash(path.join(reference, relative)) !== asset.sha256) throw new Error(`Vendor checksum mismatch: ${asset.path}`);
  }
  const homeAssets = [entries.home.script, ...entries.home.styles, ...fs.readdirSync(path.join(homeDir, 'assets')).filter(name => /^HomeView-.*\.(js|css)$/.test(name)).map(name => '/assets/' + name)].map(file => ({ file, sha256: hash(path.join(homeDir, file)) }));
  const changes = [], copied = new Map();
  fs.mkdirSync(outputDir, { recursive: true });

  /** @brief Merge static assets, requiring colliding hashed files to be identical.
   * @param source Input directory. @param destination Output directory.
   */
  function copyTree(source, destination) {
    for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
      const from = path.join(source, entry.name), to = path.join(destination, entry.name);
      if (entry.isDirectory()) {
        fs.mkdirSync(to, { recursive: true });
        copyTree(from, to);
      } else {
        const digest = hash(from);
        if (copied.has(to) && from.includes(path.sep + 'assets' + path.sep) && copied.get(to) !== digest) throw new Error(`Asset collision: ${to}`);
        if (from !== to) fs.copyFileSync(from, to);
        copied.set(to, digest);
      }
    }
  }

  /** @brief Modify one verified fragment in the assembled copy, preserving the pinned vendor.
   * @param file Relative asset path. @param before Exact original text.
   * @param after Compatible replacement. @param reason Backend contract difference.
   */
  function patch(file, before, after, reason) {
    const target = path.join(outputDir, file);
    const source = fs.readFileSync(target, 'utf8');
    if (source.split(before).length !== 2) throw new Error(`Expected one patch anchor in ${file}`);
    const originalHash = hash(target);
    fs.writeFileSync(target, source.replace(before, after), 'utf8');
    changes.push({ file, reason, originalHash, adaptedHash: hash(target) });
  }

  copyTree(homeDir, outputDir);
  copyTree(reference, outputDir);
  patch('assets/KeysView-DKiqlBTi.js',
    'async function Qa(){const{data:y}=await Oa.get("/user/custom-endpoints");return y}',
    'async function Qa(){const{data:y}=await Oa.get("/settings/public");return y.custom_endpoints||[]}',
    'Custom endpoints are provided by the existing public-settings API.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    '{path:"/ip-allowlist",label:l("nav.ipAllowlist"),icon:ue},',
    '',
    'Apply the requested console menu selection.');
  patch('assets/index-DlKpmTe8.js',
    'function va(P){const k=(P==null?void 0:P.trim())||"";return/^data:image\\//i.test(k)?"/site-logo":k}',
    'function va(P){return(P==null?void 0:P.trim())||"/logo.png?v=20260715"}',
    'Keep the site logo URL/data URI and original PNG fallback; no /site-logo endpoint is required.');
  const integration = path.join(outputDir, 'integration');
  fs.mkdirSync(integration, { recursive: true });
  copyTree(path.join(root, 'runtime'), integration);
  entries.console.styles.push('/integration/console.css');
  fs.writeFileSync(path.join(integration, 'entries.js'), `/** @brief Generated application entry descriptors. */\nexport const entries = ${JSON.stringify(entries, null, 2)};\n`, 'utf8');
  fs.writeFileSync(path.join(outputDir, 'index.html'), `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <link rel="icon" type="image/png" href="/logo.png?v=20260715" />
    <title>Sub2API - AI API Gateway</title>
    <script type="module" src="/integration/bootstrap.js"></script>
  </head>
  <body><div id="app"></div></body>
</html>
`, 'utf8');
  const manifest = { builtAt: new Date().toISOString(), entries, homeAssets, changes };
  fs.writeFileSync(path.join(integration, 'build-manifest.json'), JSON.stringify(manifest, null, 2), 'utf8');
  return manifest;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const output = path.resolve(root, '../../backend/internal/web/dist');
  const result = assemble({ homeDir: output });
  console.log(`Kedaya adaptation: ${result.homeAssets.length} homepage assets preserved; ${result.changes.length} compatibility patches; ${output}`);
}
