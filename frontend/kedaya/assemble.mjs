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
    const previous = changes.find(change => change.file === file);
    if (previous) {
      previous.reason += ' ' + reason;
      previous.adaptedHash = hash(target);
    } else changes.push({ file, reason, originalHash, adaptedHash: hash(target) });
  }

  copyTree(homeDir, outputDir);
  copyTree(reference, outputDir);
  patch('assets/GroupDistributionChart.vue_vue_type_script_setup_true_lang-CFxB0FuD.js',
    'class:[c.class,I.has(c.key)?"usage-text-column":"",c.key==="created_at"?"usage-date-column":""]',
    'class:[c.class,"zhouz-usage-"+c.key,I.has(c.key)?"usage-text-column":"",c.key==="created_at"?"usage-date-column":""]',
    'Identify usage columns so administrator cells can truncate independently of column visibility and order.');
  patch('assets/KeysView-DKiqlBTi.js',
    'async function Qa(){const{data:y}=await Oa.get("/user/custom-endpoints");return y}',
    'async function Qa(){const{data:y}=await Oa.get("/settings/public");return y.custom_endpoints||[]}',
    'Custom endpoints are provided by the existing public-settings API.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    'H=k(()=>{var L;return!P.isSimpleMode&&((L=$.value)==null?void 0:L.role)==="admin"})',
    'H=k(()=>!1)',
    'Remove the onboarding replay action from the account menu.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    'Ut({storageKey:P.value?"admin_guide":"user_guide"})',
    'Ut({storageKey:P.value?"admin_guide":"user_guide",autoStart:!1})',
    'Disable automatic onboarding tour startup in the pinned console.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    'l.isSimpleMode||!(((d=l.user)==null?void 0:d.role)==="admin")||y()||($=setTimeout(()=>{W()},f.AUTO_START_DELAY_MS))',
    'i.autoStart===false||l.isSimpleMode||!(((d=l.user)==null?void 0:d.role)==="admin")||y()||($=setTimeout(()=>{W()},f.AUTO_START_DELAY_MS))',
    'Make the autoStart option effective so the console does not block navigation with an onboarding overlay.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    '{path:"/ip-allowlist",label:l("nav.ipAllowlist"),icon:ue},',
    '',
    'Apply the requested console menu selection.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    '{path:"/admin/subscriptions",label:l("nav.subscriptions"),icon:Z,hideInSimpleMode:!0}',
    '{path:"/admin/subscriptions",label:l("nav.subscriptions"),icon:Z,hideInSimpleMode:!0,featureFlag:()=>{var U;return((U=x.cachedPublicSettings)==null?void 0:U.subscription_enabled)!==false}}',
    'Apply the same subscription visibility flag as the native sidebar.');
  patch('assets/index-DlKpmTe8.js',
    'function va(P){const k=(P==null?void 0:P.trim())||"";return/^data:image\\//i.test(k)?"/site-logo":k}',
    'function va(P){return(P==null?void 0:P.trim())||"/logo.png?v=20260715"}',
    'Keep the site logo URL/data URI and original PNG fallback; no /site-logo endpoint is required.');

  const integration = path.join(outputDir, 'integration');
  fs.mkdirSync(integration, { recursive: true });
  copyTree(path.join(root, 'runtime'), integration);
  // The release directory prevents existing immutable browser/CDN caches from mixing builds.
  const revision = createHash('sha256').update(hash(fileURLToPath(import.meta.url))).update(JSON.stringify({ entries, changes })).update(
    fs.readdirSync(path.join(root, 'runtime')).sort().map(file => hash(path.join(root, 'runtime', file))).join('')
  ).digest('hex').slice(0, 12);
  const releasePrefix = `/integration/${revision}`;
  const releaseDirectory = path.join(outputDir, releasePrefix);
  fs.mkdirSync(releaseDirectory, { recursive: true });
  copyTree(path.join(root, 'runtime'), releaseDirectory);
  const publishedAssets = [];
  for (const asset of referenceManifest.assets.filter(item => item.path.startsWith('/assets/'))) {
    const target = path.join(releaseDirectory, asset.path);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(path.join(outputDir, asset.path), target);
    if (asset.path === entries.console.script) {
      const code = fs.readFileSync(target, 'utf8');
      const anchor = 'Va=function(e){return"/"+e}';
      if (code.split(anchor).length !== 2) throw new Error('Expected the pinned Vite asset resolver');
      fs.writeFileSync(target, code.replace(anchor, `Va=function(e){return"${releasePrefix}/"+e}`), 'utf8');
    }
    publishedAssets.push({ file: releasePrefix + asset.path, sha256: hash(target) });
  }
  const patchedConsoleAsset = publishedAssets.find(asset => asset.file === releasePrefix + entries.console.script);
  if (patchedConsoleAsset) patchedConsoleAsset.sha256 = hash(path.join(outputDir, patchedConsoleAsset.file));
  entries.console.script = releasePrefix + entries.console.script;
  entries.console.styles = entries.console.styles.map(file => releasePrefix + file);
  const nativeLayoutStyles = fs.readdirSync(path.join(homeDir, 'assets')).filter(file => /^AppHeader-.*\.css$/.test(file)).map(file => '/assets/' + file);
  if (nativeLayoutStyles.length !== 1) throw new Error('Expected the original console layout stylesheet');
  entries.native = {
    script: entries.home.script,
    styles: [...entries.home.styles, ...nativeLayoutStyles, ...entries.console.styles, releasePrefix + '/assets/ConsoleAtmosphere-M5YVlMLF.css', releasePrefix + '/console.css'],
  };
  entries.console.styles.push(releasePrefix + '/console.css');
  fs.writeFileSync(path.join(integration, 'entries.js'), `/** @brief Generated application entry descriptors. */\nexport const entries = ${JSON.stringify(entries, null, 2)};\n`, 'utf8');
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
        if (brand && /^(?:\/(?!\/)|data:image\/|https?:\/\/)/i.test(logo)) {
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
  const manifest = { builtAt: new Date().toISOString(), releasePrefix, entries, homeAssets, changes, publishedAssets };
  fs.writeFileSync(path.join(integration, 'build-manifest.json'), JSON.stringify(manifest, null, 2), 'utf8');
  return manifest;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const output = path.resolve(root, '../../backend/internal/web/dist');
  const result = assemble({ homeDir: output });
  console.log(`Kedaya adaptation: ${result.homeAssets.length} homepage assets preserved; ${result.changes.length} compatibility patches; ${output}`);
}
