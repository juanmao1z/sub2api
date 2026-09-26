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
    '{path:"/ip-allowlist",label:l("nav.ipAllowlist"),icon:ue},',
    '',
    'Apply the requested console menu selection.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    'const{t:l}=Te(),g=We(),{isConsoleSignal:A}=Je()',
    'const{t:l,locale:kedayaLocale}=Te(),g=We(),{isConsoleSignal:A}=Je()',
    'Use the current language for the site ticket menu.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    '{path:"/profile",label:l("nav.profile"),icon:S}',
    '{path:"/tickets",label:kedayaLocale.value.startsWith("zh")?"工单系统":"Tickets",icon:_},{path:"/profile",label:l("nav.profile"),icon:S}',
    'Restore the user ticket workspace entry.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    '{path:"/admin/subscriptions",label:l("nav.subscriptions"),icon:Z,hideInSimpleMode:!0}',
    '{path:"/admin/subscriptions",label:l("nav.subscriptions"),icon:Z,hideInSimpleMode:!0,featureFlag:()=>{var U;return((U=x.cachedPublicSettings)==null?void 0:U.subscription_enabled)!==false}}',
    'Apply the same subscription visibility flag as the native sidebar.');
  patch('assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js',
    '{path:"/admin/plugins",label:l("nav.plugins"),icon:R,featureFlag:Ne},{path:"/admin/announcements",label:l("nav.announcements"),icon:q}',
    '{path:"/admin/plugins",label:l("nav.plugins"),icon:R,featureFlag:Ne},{path:"/admin/tickets",label:kedayaLocale.value.startsWith("zh")?"工单系统":"Tickets",icon:_},{path:"/admin/announcements",label:l("nav.announcements"),icon:q}',
    'Keep the administrator ticket workspace before announcements across feature-flag combinations.');
  patch('assets/index-DlKpmTe8.js',
    'function va(P){const k=(P==null?void 0:P.trim())||"";return/^data:image\\//i.test(k)?"/site-logo":k}',
    'function va(P){return(P==null?void 0:P.trim())||"/logo.png?v=20260715"}',
    'Keep the site logo URL/data URI and original PNG fallback; no /site-logo endpoint is required.');

  const ticketAssetSetup = ({ releasePrefix, releaseDirectory, routeAsset }) => {
    const consoleVendorVue = referenceManifest.assets.find(asset => asset.path.startsWith('/assets/vendor-vue-') && asset.path.endsWith('.js'));
    if (!consoleVendorVue) throw new Error('Expected the pinned console Vue runtime asset');
    const nativeTicketScripts = fs.readdirSync(path.join(outputDir, 'assets')).filter(file => /^SupportTicketsView-.*\.js$/.test(file));
    const userTicketScript = nativeTicketScripts.find(file => !fs.readFileSync(path.join(outputDir, 'assets', file), 'utf8').includes('adminList'));
    const adminTicketScript = nativeTicketScripts.find(file => fs.readFileSync(path.join(outputDir, 'assets', file), 'utf8').includes('adminList'));
    if (!userTicketScript || !adminTicketScript) throw new Error('Expected native user and administrator ticket chunks');
    const ticketBridge = path.join(releaseDirectory, 'ticket-bridge.js');
    fs.writeFileSync(ticketBridge, `/** @brief Adapt native ticket services to the Kedaya console runtime. */
import { d as defineComponent } from '${releasePrefix}${consoleVendorVue.path}';

const messages = {
  'zh': {
    'support.centerTitle': '工单系统', 'support.centerDescription': '提交问题并与支持团队保持沟通', 'support.title': '我的工单', 'support.adminTitle': '工单管理', 'support.adminDescription': '查看和处理用户提交的工单', 'support.adminSearch': '搜索工单、用户或邮箱', 'support.search': '搜索工单', 'support.filterStatus': '筛选状态', 'support.allTickets': '全部工单', 'support.ticketCount': ({ count }) => count + ' 条', 'support.newTicket': '新建工单', 'support.empty': '暂无工单', 'support.noMatches': '没有匹配的工单', 'support.historyHint': '你的工单和对话会显示在这里。', 'support.adminEmptyHint': '当前没有待处理的工单。', 'support.changeFilters': '请调整筛选条件后重试。', 'support.resetFilters': '重置筛选', 'support.selectTicket': '选择一个工单', 'support.selectHint': '从左侧选择工单查看详情。', 'support.emptyHint': '创建工单后可在这里查看对话。', 'support.adminSelectHint': '从左侧选择工单查看详情。', 'support.contactSupport': '联系支持', 'support.createHint': '请描述你遇到的问题，我们会尽快回复。', 'support.type': '问题类型', 'support.refund': '退款记录', 'support.suggestion': '建议', 'support.refundHint': '与订单退款相关的问题', 'support.suggestionHint': '反馈问题或提出建议', 'support.subject': '主题', 'support.subjectPlaceholder': '请输入工单主题', 'support.orderId': '订单号', 'support.orderPlaceholder': '请输入订单号', 'support.contact': '联系方式', 'support.contactPlaceholder': '邮箱、电话或其他联系方式', 'support.description': '问题描述', 'support.descriptionPlaceholder': '请详细描述问题', 'support.privacyHint': '请勿填写密码或其他敏感信息。', 'support.submitTicket': '提交工单', 'support.submitting': '提交中...', 'support.reply': '回复', 'support.replyPlaceholder': '输入回复内容', 'support.sending': '发送中...', 'support.close': '关闭工单', 'support.closedHint': '该工单已关闭。', 'support.adminReplyHint': '回复会发送给提交工单的用户。', 'support.manageStatus': '工单状态', 'support.saveStatus': '保存状态', 'support.savingStatus': '保存中...', 'support.adminClosedHint': '该工单已关闭。', 'support.user': '用户', 'support.you': '你', 'support.team': '支持团队', 'support.originalMessage': '原始问题', 'support.created': '工单已创建', 'support.replySent': '回复已发送', 'support.closed': '工单已关闭', 'support.statusSaved': '状态已保存', 'support.status.OPEN': '未关闭', 'support.status.RESOLVED': '已解决', 'support.status.CLOSED': '已关闭', 'common.loading': '加载中...', 'common.refresh': '刷新', 'common.cancel': '取消', 'common.error': '操作失败'
  },
  'en': {
    'support.centerTitle': 'Ticket System', 'support.centerDescription': 'Submit issues and stay in touch with support', 'support.title': 'My Tickets', 'support.adminTitle': 'Ticket Management', 'support.adminDescription': 'Review and respond to user tickets', 'support.adminSearch': 'Search tickets, users or email', 'support.search': 'Search tickets', 'support.filterStatus': 'Filter status', 'support.allTickets': 'All tickets', 'support.ticketCount': ({ count }) => count + ' tickets', 'support.newTicket': 'New Ticket', 'support.empty': 'No tickets yet', 'support.noMatches': 'No matching tickets', 'support.historyHint': 'Your tickets and conversations will appear here.', 'support.adminEmptyHint': 'There are no tickets to handle.', 'support.changeFilters': 'Adjust the filters and try again.', 'support.resetFilters': 'Reset filters', 'support.selectTicket': 'Select a ticket', 'support.selectHint': 'Select a ticket on the left to view details.', 'support.emptyHint': 'Create a ticket to start a conversation.', 'support.adminSelectHint': 'Select a ticket on the left to view details.', 'support.contactSupport': 'Contact support', 'support.createHint': 'Describe the issue and our team will reply as soon as possible.', 'support.type': 'Issue type', 'support.refund': 'Refund record', 'support.suggestion': 'Suggestion', 'support.refundHint': 'Questions about an order refund', 'support.suggestionHint': 'Report an issue or share a suggestion', 'support.subject': 'Subject', 'support.subjectPlaceholder': 'Enter a subject', 'support.orderId': 'Order ID', 'support.orderPlaceholder': 'Enter the order ID', 'support.contact': 'Contact information', 'support.contactPlaceholder': 'Email, phone, or another contact method', 'support.description': 'Description', 'support.descriptionPlaceholder': 'Describe the issue', 'support.privacyHint': 'Do not include passwords or other sensitive information.', 'support.submitTicket': 'Submit ticket', 'support.submitting': 'Submitting...', 'support.reply': 'Reply', 'support.replyPlaceholder': 'Write a reply', 'support.sending': 'Sending...', 'support.close': 'Close ticket', 'support.closedHint': 'This ticket is closed.', 'support.adminReplyHint': 'The reply will be sent to the ticket owner.', 'support.manageStatus': 'Ticket status', 'support.saveStatus': 'Save status', 'support.savingStatus': 'Saving...', 'support.adminClosedHint': 'This ticket is closed.', 'support.user': 'User', 'support.you': 'You', 'support.team': 'Support team', 'support.originalMessage': 'Original issue', 'support.created': 'Ticket created', 'support.replySent': 'Reply sent', 'support.closed': 'Ticket closed', 'support.statusSaved': 'Status saved', 'support.status.OPEN': 'Open', 'support.status.RESOLVED': 'Resolved', 'support.status.CLOSED': 'Closed', 'common.loading': 'Loading...', 'common.refresh': 'Refresh', 'common.cancel': 'Cancel', 'common.error': 'Operation failed'
  }
};

export function u() {
  const locale = { value: document.documentElement.lang?.startsWith('zh') ? 'zh' : 'en' };
  return { locale, t(key, params) { const value = messages[locale.value][key] ?? messages.en[key] ?? key; return typeof value === 'function' ? value(params || {}) : value; } };
}

export function a() { return { showError: message => console.error(message), showSuccess: message => console.info(message) }; }
export const _ = defineComponent({ name: 'TicketIcon', setup: () => () => null });
export function B(component) { return component; }
export function b(error) { return error?.message || 'Operation failed'; }

async function request(method, url, body) {
  const headers = { 'Content-Type': 'application/json', 'Accept-Language': document.documentElement.lang?.startsWith('zh') ? 'zh' : 'en' };
  const token = localStorage.getItem('auth_token');
  if (token) headers.Authorization = \`Bearer \${token}\`;
  const response = await fetch('/api/v1' + url, { method, headers, credentials: 'include', body: body === undefined ? undefined : JSON.stringify(body) });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok || payload.code && payload.code !== 0) throw Object.assign(new Error(payload.message || 'Request failed'), { status: response.status, response: { data: payload } });
  return { data: payload.data };
}
export const f = { get: url => request('GET', url), post: (url, body) => request('POST', url, body), patch: (url, body) => request('PATCH', url, body) };
export const s = { list: () => f.get('/support/tickets'), create: body => f.post('/support/tickets', body), get: id => f.get(\`/support/tickets/\${id}\`), addMessage: (id, body) => f.post(\`/support/tickets/\${id}/messages\`, { body }), close: id => f.post(\`/support/tickets/\${id}/close\`), adminList: () => f.get('/admin/support/tickets'), adminGet: id => f.get(\`/admin/support/tickets/\${id}\`), adminAddMessage: (id, body) => f.post(\`/admin/support/tickets/\${id}/messages\`, { body }), adminSetStatus: (id, status) => f.patch(\`/admin/support/tickets/\${id}/status\`, { status }) };
`, 'utf8');

  function prepareTicketChunk(sourceName, outputName) {
    let source = fs.readFileSync(path.join(outputDir, 'assets', sourceName), 'utf8');
    source = source.replace(/from"\.\/vendor-vue-[^"]+\.js"/, `from"${releasePrefix}${consoleVendorVue.path}"`)
      .replace(/from"\.\/AppLayout\.vue_vue_type_script_setup_true_lang-[^"]+\.js"/, `from"${releasePrefix}/assets/AppLayout.vue_vue_type_script_setup_true_lang-D-wpKinR.js"`)
      .replace(/from"\.\/index-[^"]+\.js"/, `from"${releasePrefix}/ticket-bridge.js"`)
      .replace(/from"\.\/supportTickets-[^"]+\.js"/, `from"${releasePrefix}/ticket-bridge.js"`)
      .replace(/from"\.\/apiError-[^"]+\.js"/, `from"${releasePrefix}/ticket-bridge.js"`)
      .replace(/from"\.\/vendor-i18n-[^"]+\.js"/, `from"${releasePrefix}/ticket-bridge.js"`)
      .replace(/(from|import)"\.\/([^"]+)"/g, (_, syntax, asset) => `${syntax}"/assets/${asset}"`);
    fs.writeFileSync(path.join(releaseDirectory, outputName), source, 'utf8');
  }
  prepareTicketChunk(userTicketScript, 'tickets-user.js');
  prepareTicketChunk(adminTicketScript, 'tickets-admin.js');
  for (const css of fs.readdirSync(path.join(outputDir, 'assets')).filter(file => /^SupportTicketsView-.*\.css$/.test(file))) {
    fs.copyFileSync(path.join(outputDir, 'assets', css), path.join(releaseDirectory, css));
  }

  let routeSource = fs.readFileSync(routeAsset, 'utf8');
  const routeAnchor = '{path:"/admin",redirect:"/admin/dashboard"}';
  if (routeSource.split(routeAnchor).length !== 2) throw new Error('Expected Kedaya admin route anchor');
  routeSource = routeSource.replace(routeAnchor,
    `{path:"/tickets",name:"SupportTickets",component:()=>g(()=>import("${releasePrefix}/tickets-user.js")),meta:{requiresAuth:!0,title:"Ticket System",titleKey:"nav.supportTickets"}},{path:"/admin/tickets",name:"AdminSupportTickets",component:()=>g(()=>import("${releasePrefix}/tickets-admin.js")),meta:{requiresAuth:!0,requiresAdmin:!0,title:"Ticket System",titleKey:"nav.supportTickets"}},{path:"/admin",redirect:"/admin/dashboard"}`);
    fs.writeFileSync(routeAsset, routeSource, 'utf8');
  };
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
  ticketAssetSetup({ releasePrefix, releaseDirectory, routeAsset: path.join(releaseDirectory, entries.console.script.slice(1)) });
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
  for (const css of fs.readdirSync(releaseDirectory).filter(file => /^SupportTicketsView-.*\.css$/.test(file))) entries.console.styles.push(releasePrefix + '/' + css);
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
      html, body { min-height: 100%; margin: 0; }
      #startup-loading { position: fixed; inset: 0; z-index: 2147483647; display: grid; place-content: center; justify-items: center; gap: 12px; background: #f9fafb; color: #4b5563; font: 14px system-ui, sans-serif; }
      #startup-loading-indicator { width: 24px; height: 24px; border: 2px solid #d1d5db; border-top-color: #15803d; border-radius: 50%; animation: startup-spin .8s linear infinite; }
      @keyframes startup-spin { to { transform: rotate(360deg); } }
      @media (prefers-reduced-motion: reduce) { #startup-loading-indicator { animation: none; } }
    </style>
    <script type="module" src="${releasePrefix}/bootstrap.js"></script>
  </head>
  <body>
    <div id="app"></div>
    <div id="startup-loading" role="status" aria-live="polite">
      <span id="startup-loading-indicator" aria-hidden="true"></span>
      <span>正在加载页面</span>
    </div>
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
