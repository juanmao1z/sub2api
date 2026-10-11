/** @file @brief Assemble the pinned Sub2API review UI with production runtime adapters. */
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.dirname(fileURLToPath(import.meta.url));

/** @brief Calculate a file digest. */
function hash(file) {
  return createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

/** @brief Read the pinned Vite entry and global styles. */
function readEntry(directory) {
  const html = fs.readFileSync(path.join(directory, 'index.html'), 'utf8');
  const script = html.match(/<script[^>]+type="module"[^>]+src="([^"]+)"/)?.[1];
  const styles = [...html.matchAll(/<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"/g)].map(match => match[1]);
  if (!script || !styles.length) throw new Error(`Expected a pinned review build in ${directory}`);
  return { script, styles };
}

/** @brief Assemble the review asset graph and patch only production contract differences. */
export function assemble({ homeDir, outputDir = homeDir }) {
  homeDir = path.resolve(homeDir);
  outputDir = path.resolve(outputDir);
  const reference = path.join(root, 'vendor');
  const referenceManifest = JSON.parse(fs.readFileSync(path.join(root, 'reference-manifest.json'), 'utf8'));
  const referenceEntry = readEntry(reference);
  const entries = { review: referenceEntry };
  const changes = [];
  const copied = new Map();

  for (const asset of referenceManifest.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'))) {
    const file = path.join(reference, asset.path.slice(1));
    if (!fs.existsSync(file) || hash(file) !== asset.sha256) throw new Error(`Vendor checksum mismatch: ${asset.path}`);
  }

  /** @brief Copy a tree while detecting conflicting generated assets. */
  function copyTree(source, destination) {
    for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
      const from = path.join(source, entry.name);
      const to = path.join(destination, entry.name);
      if (entry.isDirectory()) {
        fs.mkdirSync(to, { recursive: true });
        copyTree(from, to);
        continue;
      }
      const digest = hash(from);
      if (copied.has(to) && copied.get(to) !== digest) throw new Error(`Asset collision: ${to}`);
      fs.mkdirSync(path.dirname(to), { recursive: true });
      if (from !== to) fs.copyFileSync(from, to);
      copied.set(to, digest);
    }
  }

  /** @brief Replace one exact fragment and record the production adaptation. */
  function patch(file, before, after, reason) {
    const target = path.join(outputDir, file);
    const source = fs.readFileSync(target, 'utf8');
    if (source.split(before).length !== 2) throw new Error(`Expected one patch anchor in ${file}`);
    const originalHash = hash(target);
    fs.writeFileSync(target, source.replace(before, after), 'utf8');
    changes.push({ file, reason, originalHash, adaptedHash: hash(target) });
  }

  /** @brief Replace all occurrences of a known captured-site string. */
  function replaceAll(file, before, after, reason) {
    const target = path.join(outputDir, file);
    const source = fs.readFileSync(target, 'utf8');
    const count = source.split(before).length - 1;
    if (!count) throw new Error(`Expected captured-site text in ${file}: ${before}`);
    const originalHash = hash(target);
    fs.writeFileSync(target, source.split(before).join(after), 'utf8');
    changes.push({ file, reason: `${reason} (${count} replacements)`, originalHash, adaptedHash: hash(target) });
  }

  // Keep the formal Vite output for public branding files, then replace its compiled app graph.
  copyTree(homeDir, outputDir);
  fs.rmSync(path.join(outputDir, 'assets'), { recursive: true, force: true });
  fs.rmSync(path.join(outputDir, 'canvas'), { recursive: true, force: true });
  copyTree(path.join(reference, 'assets'), path.join(outputDir, 'assets'));
  if (fs.existsSync(path.join(reference, 'canvas'))) copyTree(path.join(reference, 'canvas'), path.join(outputDir, 'canvas'));

  patch('assets/index-CHmaXGoD.js',
    'function Aa(E){const S=(E==null?void 0:E.trim())||"";return/^data:image\\//i.test(S)?"/site-logo":S}',
    'function Aa(E){return(E==null?void 0:E.trim())||"/logo.png?v=20260715"}',
    'Use the formal Sub2API logo fallback; the backend does not expose a /site-logo endpoint.');
  patch('assets/index-CHmaXGoD.js',
    'Fa=function(e){return"/"+e}',
    'Fa=function(e){return"/integration/__RELEASE_PREFIX__/"+e}',
    'Resolve lazy review chunks inside the immutable production release directory.');
  patch('assets/KeysView-C1R_JMh0.js',
    'async function Za(){const{data:y}=await Da.get("/user/custom-endpoints");return y}',
    'async function Za(){const{data:y}=await Da.get("/settings/public");return y.custom_endpoints||[]}',
    'Use the existing public-settings contract for custom endpoints.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-C4ysyc_b.js',
    '{path:"/admin/subscriptions",label:s("nav.subscriptions"),icon:J,hideInSimpleMode:!0}',
    '{path:"/admin/subscriptions",label:s("nav.subscriptions"),icon:J,hideInSimpleMode:!0,featureFlag:()=>{var U;return((U=x.cachedPublicSettings)==null?void 0:U.subscription_enabled)!==false}}',
    'Keep administrator subscription navigation aligned with public feature settings.');
  patch('assets/providerConfig-wYxXJe_k.js',
    'defaultValue:"https://ep.kedaya.xyz"',
    'defaultValue:""',
    'Remove the captured-site payment gateway default; administrators must configure the real provider endpoint.');
  replaceAll('assets/catalog-D1qxTQrs.js', 'kedaya.ai', 'Sub2API', 'Replace captured-site domain in legal fallback content.');
  replaceAll('assets/AppLayout.vue_vue_type_script_setup_true_lang-C4ysyc_b.js', 'ghcr.io/kiss-kedaya/sub2api', 'ghcr.io/wei-shaw/sub2api', 'Use the formal Sub2API container repository in the version panel.');
  replaceAll('assets/AppLayout.vue_vue_type_script_setup_true_lang-C4ysyc_b.js', 'kiss-kedaya/sub2api', 'Wei-Shaw/sub2api', 'Use the formal Sub2API source repository in the version panel.');

  const integration = path.join(outputDir, 'integration');
  fs.rmSync(integration, { recursive: true, force: true });
  fs.mkdirSync(integration, { recursive: true });
  copyTree(path.join(root, 'runtime'), integration);

  const revision = createHash('sha256')
    .update(hash(fileURLToPath(import.meta.url)))
    .update(JSON.stringify({ entries, changes }))
    .update(fs.readdirSync(path.join(root, 'runtime')).sort().map(file => hash(path.join(root, 'runtime', file))).join(''))
    .digest('hex').slice(0, 12);
  const releasePrefix = `/integration/${revision}`;
  const releaseDirectory = path.join(outputDir, releasePrefix);
  fs.mkdirSync(releaseDirectory, { recursive: true });
  copyTree(path.join(root, 'runtime'), releaseDirectory);

  const entryFile = path.join(outputDir, entries.review.script);
  const entryCode = fs.readFileSync(entryFile, 'utf8').replace('/integration/__RELEASE_PREFIX__/', `${releasePrefix}/`);
  fs.writeFileSync(entryFile, entryCode, 'utf8');

  const publishedAssets = [];
  for (const asset of referenceManifest.assets.filter(item => item.path.startsWith('/assets/') || item.path.startsWith('/canvas/'))) {
    const target = path.join(releaseDirectory, asset.path.slice(1));
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(path.join(outputDir, asset.path.slice(1)), target);
    publishedAssets.push({ file: releasePrefix + asset.path, sha256: hash(target) });
  }

  entries.review.script = releasePrefix + entries.review.script;
  entries.review.styles = entries.review.styles.map(file => releasePrefix + file);
  entries.review.styles.push(releasePrefix + '/console.css');
  fs.writeFileSync(path.join(integration, 'entries.js'), `/** @brief Generated production application entry descriptors. */\nexport const entries = ${JSON.stringify(entries, null, 2)};\n`, 'utf8');
  fs.copyFileSync(path.join(integration, 'entries.js'), path.join(releaseDirectory, 'entries.js'));

  fs.writeFileSync(path.join(outputDir, 'index.html'), `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <link rel="icon" type="image/png" href="/logo.png?v=20260715" />
    <title>Sub2API - AI API Gateway</title>
    <style>
      html, body { min-height: 100%; margin: 0; background: var(--startup-bg); }
      :root { color-scheme: light; --startup-bg: #fafafa; --startup-text: #737379; --startup-line: #e3e3e6; --startup-accent: #52525b; }
      :root.dark { color-scheme: dark; --startup-bg: #0b0f14; --startup-text: #98a5b4; --startup-line: #27313d; --startup-accent: #82a7ff; }
      #startup-loading { position: fixed; inset: 0; z-index: 2147483647; display: grid; place-content: center; justify-items: center; gap: 12px; background: var(--startup-bg); color: var(--startup-text); font: 14px system-ui, sans-serif; }
      #startup-loading-brand { display: flex; align-items: center; gap: 8px; min-height: 32px; font-weight: 600; }
      #startup-loading-brand:empty { display: none; }
      #startup-loading-logo { width: 32px; height: 32px; object-fit: contain; }
      #startup-loading-indicator { width: 24px; height: 24px; border: 2px solid var(--startup-line); border-top-color: var(--startup-accent); border-radius: 50%; animation: startup-spin .8s linear infinite; }
      @keyframes startup-spin { to { transform: rotate(360deg); } }
      @media (prefers-reduced-motion: reduce) { #startup-loading-indicator { animation: none; } }
    </style>
    <script nonce="__CSP_NONCE_VALUE__">
      (() => {
        let theme = 'light'
        try { theme = localStorage.getItem('theme') === 'dark' ? 'dark' : 'light' } catch {}
        const root = document.documentElement
        root.classList.toggle('dark', theme === 'dark')
        root.dataset.theme = theme
        root.style.colorScheme = theme
      })()
    </script>
    <script type="module" src="${releasePrefix}/bootstrap.js"></script>
  </head>
  <body>
    <div id="app"></div>
    <div id="startup-loading" role="status" aria-live="polite">
      <div id="startup-loading-brand" aria-hidden="true"></div>
      <span id="startup-loading-indicator" aria-hidden="true"></span>
      <span id="startup-loading-label">正在加载页面</span>
    </div>
    <script nonce="__CSP_NONCE_VALUE__">
      (() => {
        const config = window.__APP_CONFIG__
        const name = typeof config?.site_name === 'string' ? config.site_name.trim() : ''
        const logo = typeof config?.site_logo === 'string' ? config.site_logo.trim() : ''
        const brand = document.getElementById('startup-loading-brand')
        const label = document.getElementById('startup-loading-label')
        if (name && brand) brand.textContent = name
        if (name && label) label.textContent = '正在加载 ' + name
        if (brand && /^(?:\\/(?!\\/)|data:image\\/|https?:\\/\\/)/i.test(logo)) {
          const image = document.createElement('img')
          image.id = 'startup-loading-logo'
          image.alt = ''
          image.src = logo
          brand.prepend(image)
        }
      })()
    </script>
  </body>
</html>
`, 'utf8');

  const manifest = {
    builtAt: new Date().toISOString(),
    releasePrefix,
    entries,
    homeAssets: ['logo.png', 'logo.svg'].filter(file => fs.existsSync(path.join(outputDir, file))).map(file => ({ file: '/' + file, sha256: hash(path.join(outputDir, file)) })),
    changes,
    publishedAssets,
  };
  fs.writeFileSync(path.join(integration, 'build-manifest.json'), JSON.stringify(manifest, null, 2), 'utf8');
  return manifest;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const output = path.resolve(root, '../../backend/internal/web/dist');
  const result = assemble({ homeDir: output });
  console.log(`Sub2API review UI adaptation: ${result.publishedAssets.length} compiled assets; ${result.changes.length} production patches; ${output}`);
}
