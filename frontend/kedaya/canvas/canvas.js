import { MAX_NODES, MAX_DOCUMENT_BYTES, parseDocument, sameOriginBase, safeImageSource, validateDocument } from './canvas-core.mjs';

const $ = id => document.getElementById(id);
const params = new URLSearchParams(location.search);
const apiKey = params.get('apiKey') || '';
const baseParam = params.get('baseUrl') || '';
const lang = params.get('lang')?.toLowerCase().startsWith('zh') ? 'zh' : 'en';
const theme = params.get('theme');
let cleanUrl = true;
try { history.replaceState(null, '', location.pathname); } catch { cleanUrl = false; }
if (theme === 'dark' || theme === 'light') document.documentElement.dataset.theme = theme;
document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
const dictionary = {
  en: { title: 'Infinite Canvas', addNote: '+ Note', addText: '+ Text', select: 'Select', pan: 'Pan', delete: 'Delete', export: 'Export', import: 'Import', editor: 'Selection', empty: 'Select a node to edit it.', content: 'Content', generate: 'Generate', model: 'Model', kind: 'Output', text: 'Text', image: 'Image', prompt: 'Prompt', generateButton: 'Generate', cancel: 'Cancel', retry: 'Retry', loading: 'Loading models...', saved: 'Saved locally', saving: 'Saving...', noStorage: 'Local storage unavailable', modelsFailed: 'Models unavailable', ready: 'Ready', working: 'Generating...', cancelled: 'Request cancelled', imported: 'Canvas imported', exported: 'Canvas exported', noKey: 'Missing API key', invalidBase: 'API address must be this site origin', unsafeUrl: 'Could not remove API key from URL' },
  zh: { title: '无限画布', addNote: '+ 便笺', addText: '+ 文字', select: '选择', pan: '平移', delete: '删除', export: '导出', import: '导入', editor: '选中内容', empty: '选择节点后可编辑。', content: '内容', generate: '生成', model: '模型', kind: '输出', text: '文字', image: '图片', prompt: '提示词', generateButton: '生成', cancel: '取消', retry: '重试', loading: '加载模型中...', saved: '已保存至本地', saving: '正在保存...', noStorage: '本地存储不可用', modelsFailed: '无法加载模型', ready: '就绪', working: '生成中...', cancelled: '已取消请求', imported: '已导入画布', exported: '已导出画布', noKey: '缺少 API 密钥', invalidBase: 'API 地址必须为本站域名', unsafeUrl: '无法从地址栏移除密钥' }
};
const t = dictionary[lang];
for (const element of document.querySelectorAll('[data-i18n]')) element.textContent = t[element.dataset.i18n];
document.title = t.title;

const canvas = $('canvas');
const world = $('world');
let doc = { version: 1, nodes: [], view: { x: 0, y: 0, scale: 1 } };
let selected = null;
let mode = 'select';
let drag = null;
let request = null;
let modelsRequest = null;
let storageKey = null;
let saveTimer = null;
let userId = null;
let apiOrigin = null;

function message(value, error = false) {
  $('request-status').textContent = String(value).split(apiKey || '\u0000').join('[redacted]').slice(0, 300);
  $('request-status').classList.toggle('error', error);
}
function saveMessage(value) { $('save-state').textContent = value; }
function authUser() {
  try {
    const raw = localStorage.getItem('auth_user');
    const user = JSON.parse(raw || 'null');
    return user?.id === undefined || user?.id === null ? null : String(user.id);
  } catch { return null; }
}
function sameUser() { return authUser() === userId; }
function current() { return doc.nodes.find(node => node.id === selected); }
function snapshot() { return validateDocument(doc, apiKey, location.origin); }
function queueSave() {
  if (!storageKey) return;
  saveMessage(t.saving);
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    if (!sameUser()) { storageKey = null; saveMessage(t.noStorage); return; }
    try { localStorage.setItem(storageKey, JSON.stringify(snapshot())); saveMessage(t.saved); }
    catch (error) { saveMessage(`${t.noStorage}: ${error.message}`); }
  }, 400);
}
function render() {
  world.style.transform = `translate(${doc.view.x}px, ${doc.view.y}px) scale(${doc.view.scale})`;
  $('zoom-label').textContent = `${Math.round(doc.view.scale * 100)}%`;
  const existing = new Map([...world.children].map(el => [el.dataset.id, el]));
  for (const node of doc.nodes) {
    let el = existing.get(node.id);
    if (el && el.dataset.type !== node.type) { el.remove(); el = null; }
    if (!el) {
      el = document.createElement('div');
      el.className = `node ${node.type}`;
      el.dataset.id = node.id;
      el.dataset.type = node.type;
      el.tabIndex = 0;
      el.setAttribute('role', 'button');
      if (node.type === 'image') {
        const image = document.createElement('img');
        image.referrerPolicy = 'no-referrer';
        image.onerror = () => message('Image could not be loaded', true);
        el.append(image);
      }
      el.append(document.createElement('span'));
      world.append(el);
    }
    existing.delete(node.id);
    el.style.left = `${node.x}px`;
    el.style.top = `${node.y}px`;
    el.setAttribute('aria-label', `${node.type}: ${node.text.slice(0, 70)}`);
    el.setAttribute('aria-selected', String(node.id === selected));
    if (node.type === 'image') {
      const image = el.firstElementChild;
      image.alt = node.text;
      if (image.getAttribute('src') !== node.src) image.src = node.src;
    }
    el.lastElementChild.textContent = node.text;
  }
  for (const el of existing.values()) el.remove();
  const node = current();
  $('editor').hidden = !node;
  $('selection-empty').hidden = !!node;
  $('delete').disabled = !node;
  if (node && document.activeElement !== $('node-text')) $('node-text').value = node.text;
}
function changed() { render(); queueSave(); }
function select(id) { selected = id; render(); $('node-text').value = current()?.text || ''; }
function add(type, text, src) {
  if (doc.nodes.length >= MAX_NODES) throw new Error(`Maximum ${MAX_NODES} nodes`);
  const rect = canvas.getBoundingClientRect();
  const node = { id: crypto.randomUUID(), type,
    x: Math.round((rect.width / 2 - doc.view.x) / doc.view.scale - 115),
    y: Math.round((rect.height / 2 - doc.view.y) / doc.view.scale - 60), text };
  if (type === 'image') node.src = src;
  const next = validateDocument({ ...doc, nodes: [...doc.nodes, node] }, apiKey, location.origin);
  doc = next;
  selected = node.id;
  changed();
  $('node-text').value = text;
}
function zoom(factor, x = canvas.clientWidth / 2, y = canvas.clientHeight / 2) {
  const old = doc.view.scale;
  const scale = Math.max(.25, Math.min(3, Math.round(old * factor * 1000) / 1000));
  doc.view.x = Math.max(-100000, Math.min(100000, x - (x - doc.view.x) * scale / old));
  doc.view.y = Math.max(-100000, Math.min(100000, y - (y - doc.view.y) * scale / old));
  doc.view.scale = scale;
  changed();
}
function nudge(dx, dy) {
  const node = current();
  if (!node) return;
  node.x = Math.max(-100000, Math.min(100000, node.x + dx));
  node.y = Math.max(-100000, Math.min(100000, node.y + dy));
  changed();
}
function pan(dx, dy) {
  doc.view.x = Math.max(-100000, Math.min(100000, doc.view.x + dx));
  doc.view.y = Math.max(-100000, Math.min(100000, doc.view.y + dy));
  changed();
}
function changeMode(next) {
  mode = next;
  $('mode-select').setAttribute('aria-pressed', String(next === 'select'));
  $('mode-pan').setAttribute('aria-pressed', String(next === 'pan'));
}
function removeSelection() {
  if (!current()) return;
  doc.nodes = doc.nodes.filter(node => node.id !== selected);
  selected = null;
  changed();
}
function setBusy(busy) {
  $('generate').disabled = busy || !apiOrigin || !apiKey;
  $('cancel').hidden = !busy;
  $('retry').hidden = busy || !$('retry').dataset.available;
}
async function callApi(path, options = {}) {
  if (!cleanUrl || !apiOrigin || !apiKey || !sameUser()) throw new Error('API session unavailable');
  const response = await fetch(`${apiOrigin}${path}`, { ...options,
    redirect: 'error', credentials: 'omit', referrerPolicy: 'no-referrer',
    headers: { Authorization: `Bearer ${apiKey}`, ...(options.body ? { 'Content-Type': 'application/json' } : {}) } });
  const body = await response.text();
  if (body.length > MAX_DOCUMENT_BYTES * 2) throw new Error('Response too large');
  let data;
  try { data = JSON.parse(body); } catch { throw new Error(`Invalid API response (${response.status})`); }
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${data?.error?.message || data?.message || 'Request failed'}`);
  return data;
}
async function loadModels() {
  modelsRequest?.abort();
  const controller = new AbortController();
  modelsRequest = controller;
  const select = $('model');
  select.replaceChildren(new Option(t.loading, ''));
  select.disabled = true;
  try {
    const result = await callApi('/v1/models', { signal: controller.signal });
    if (modelsRequest !== controller) return;
    const list = result?.data;
    if (!Array.isArray(list)) throw new Error('Invalid model list');
    const ids = [...new Set(list.map(item => item?.id).filter(id => typeof id === 'string' && id.length > 0 && id.length <= 200))];
    if (!ids.length) throw new Error('No available models');
    select.replaceChildren(...ids.map(id => new Option(id, id)));
    select.disabled = false;
    message(t.ready);
  } catch (error) {
    if (modelsRequest !== controller || error.name === 'AbortError') return;
    select.replaceChildren(new Option(t.modelsFailed, ''));
    message(`${t.modelsFailed}: ${error.message}`, true);
  } finally { if (modelsRequest === controller) modelsRequest = null; }
}
function verifyImage(src, signal) {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.referrerPolicy = 'no-referrer';
    const finish = error => {
      clearTimeout(timer);
      signal.removeEventListener('abort', cancel);
      image.onload = image.onerror = null;
      if (error) { image.src = ''; reject(error); } else resolve();
    };
    const cancel = () => finish(new DOMException('Cancelled', 'AbortError'));
    const timer = setTimeout(() => finish(new Error('Image load timed out')), 15000);
    signal.addEventListener('abort', cancel, { once: true });
    image.onload = () => finish();
    image.onerror = () => finish(new Error('Image could not be loaded'));
    if (signal.aborted) cancel(); else image.src = src;
  });
}
async function generate() {
  if (request) return;
  const prompt = $('prompt').value.trim();
  const model = $('model').value;
  const kind = $('kind').value;
  if (!prompt || !model || !$('model').querySelector(`option:checked`)?.value) { message('Choose a model and enter a prompt', true); return; }
  const controller = new AbortController();
  request = controller;
  $('retry').dataset.available = '';
  setBusy(true);
  message(t.working);
  try {
    const result = kind === 'image'
      ? await callApi('/v1/images/generations', { method: 'POST', signal: controller.signal, body: JSON.stringify({ model, prompt, n: 1, response_format: 'b64_json' }) })
      : await callApi('/v1/chat/completions', { method: 'POST', signal: controller.signal, body: JSON.stringify({ model, messages: [{ role: 'user', content: prompt }], stream: false }) });
    if (request !== controller || controller.signal.aborted) return;
    if (!sameUser()) throw new Error('User changed during request');
    if (kind === 'image') {
      const image = result?.data?.[0];
      const src = image?.b64_json ? `data:image/png;base64,${image.b64_json}` : image?.url;
      if (!safeImageSource(src, apiKey, location.origin)) throw new Error('No safe image returned');
      await verifyImage(src, controller.signal);
      if (controller.signal.aborted) return;
      if (!sameUser()) throw new Error('User changed during request');
      add('image', prompt.slice(0, 300), src);
    } else {
      const content = result?.choices?.[0]?.message?.content;
      if (typeof content !== 'string' || !content.trim()) throw new Error('No text returned');
      if (content.length > 4000) throw new Error('Generated text exceeds 4000 characters');
      add('text', content.trim());
    }
    message(t.ready);
  } catch (error) {
    if (controller.signal.aborted) message(t.cancelled);
    else { message(error.message, true); $('retry').dataset.available = 'true'; }
  } finally {
    if (request === controller) { request = null; setBusy(false); }
  }
}

canvas.addEventListener('pointerdown', event => {
  if (event.button !== 0) return;
  const element = event.target.closest('.node');
  if (element && mode === 'select') select(element.dataset.id);
  else if (!element) select(null);
  drag = { id: element && mode === 'select' ? element.dataset.id : null, x: event.clientX, y: event.clientY };
  canvas.setPointerCapture(event.pointerId);
  canvas.classList.add('dragging');
});
canvas.addEventListener('pointermove', event => {
  if (!drag) return;
  const dx = event.clientX - drag.x, dy = event.clientY - drag.y;
  drag.x = event.clientX; drag.y = event.clientY;
  if (drag.id) nudge(dx / doc.view.scale, dy / doc.view.scale);
  else pan(dx, dy);
});
function endDrag() { drag = null; canvas.classList.remove('dragging'); }
canvas.addEventListener('pointerup', endDrag);
canvas.addEventListener('pointercancel', endDrag);
canvas.addEventListener('lostpointercapture', endDrag);
canvas.addEventListener('wheel', event => {
  event.preventDefault();
  const rect = canvas.getBoundingClientRect();
  zoom(event.deltaY < 0 ? 1.1 : 1 / 1.1, event.clientX - rect.left, event.clientY - rect.top);
}, { passive: false });
canvas.addEventListener('keydown', event => {
  if (event.target !== canvas && !event.target.classList.contains('node')) return;
  if (event.target.classList.contains('node') && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); select(event.target.dataset.id); $('node-text').focus(); return; }
});
document.addEventListener('keydown', event => {
  if (event.target.closest('input, textarea, select') || event.ctrlKey || event.metaKey || event.altKey) return;
  if (event.key === 'Delete' || event.key === 'Backspace') { event.preventDefault(); removeSelection(); }
  else if (event.key === 'Escape') select(null);
  else if ('wasd'.includes(event.key.toLowerCase()) && event.key.length === 1) {
    const moves = { w: [0, 40], a: [40, 0], s: [0, -40], d: [-40, 0] };
    event.preventDefault(); pan(...moves[event.key.toLowerCase()]);
  } else if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)) {
    event.preventDefault();
    const step = event.shiftKey ? 50 : 10;
    const moves = { ArrowUp: [0, -step], ArrowDown: [0, step], ArrowLeft: [-step, 0], ArrowRight: [step, 0] };
    if (current()) nudge(...moves[event.key]); else pan(...moves[event.key]);
  }
});
$('add-note').onclick = () => { try { add('note', lang === 'zh' ? '新便笺' : 'New note'); } catch (error) { message(error.message, true); } };
$('add-text').onclick = () => { try { add('text', lang === 'zh' ? '新文字' : 'New text'); } catch (error) { message(error.message, true); } };
$('delete').onclick = removeSelection;
$('mode-select').onclick = () => changeMode('select');
$('mode-pan').onclick = () => changeMode('pan');
$('node-text').oninput = () => {
  const node = current();
  if (!node) return;
  if (apiKey && $('node-text').value.includes(apiKey)) { $('node-text').value = node.text; message('Sensitive content cannot be saved', true); return; }
  node.text = $('node-text').value; changed();
};
for (const button of document.querySelectorAll('[data-nudge]')) button.onclick = () => nudge(...button.dataset.nudge.split(',').map(Number));
for (const button of document.querySelectorAll('[data-shift]')) button.onclick = () => pan(...button.dataset.shift.split(',').map(Number));
$('home').onclick = () => { doc.view = { x: canvas.clientWidth / 2, y: canvas.clientHeight / 2, scale: 1 }; changed(); };
$('zoom-in').onclick = () => zoom(1.2);
$('zoom-out').onclick = () => zoom(1 / 1.2);
$('generate').onclick = generate;
$('retry').onclick = generate;
$('cancel').onclick = () => { request?.abort(); };
$('reload-models').onclick = loadModels;
$('export').onclick = () => {
  try {
    const blob = new Blob([JSON.stringify(snapshot())], { type: 'application/json' });
    if (blob.size > MAX_DOCUMENT_BYTES) throw new Error('File exceeds 3 MB');
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = 'canvas.json';
    link.click();
    setTimeout(() => URL.revokeObjectURL(link.href), 60000);
    message(t.exported);
  } catch (error) { message(error.message, true); }
};
$('import').onclick = () => $('file').click();
$('file').onchange = async event => {
  const file = event.target.files?.[0];
  event.target.value = '';
  if (!file) return;
  try {
    if (file.size > MAX_DOCUMENT_BYTES) throw new Error('File exceeds 3 MB');
    const next = parseDocument(await file.text(), apiKey, location.origin);
    if (!window.confirm('Replace the current canvas?')) return;
    doc = next; selected = null; changed(); message(t.imported);
  } catch (error) { message(error.message, true); }
};

async function initialize() {
  render();
  userId = authUser();
  if (!cleanUrl) { message(t.unsafeUrl, true); setBusy(false); return; }
  try { apiOrigin = sameOriginBase(baseParam, location.origin); }
  catch { message(t.invalidBase, true); setBusy(false); return; }
  if (!apiKey) { message(t.noKey, true); setBusy(false); return; }
  try {
    if (userId !== null && crypto.subtle) {
      const bytes = new TextEncoder().encode(`canvas-v1\u0000${userId}\u0000${apiKey}`);
      const hash = await crypto.subtle.digest('SHA-256', bytes);
      if (!sameUser()) throw new Error('User changed');
      storageKey = `canvas-v1:${Array.from(new Uint8Array(hash), byte => byte.toString(16).padStart(2, '0')).join('')}`;
      const saved = localStorage.getItem(storageKey);
      if (saved) doc = parseDocument(saved, apiKey, location.origin);
      else doc.view = { x: canvas.clientWidth / 2, y: canvas.clientHeight / 2, scale: 1 };
      saveMessage(saved ? t.saved : '');
    } else { saveMessage(t.noStorage); doc.view = { x: canvas.clientWidth / 2, y: canvas.clientHeight / 2, scale: 1 }; }
  } catch (error) { storageKey = null; saveMessage(`${t.noStorage}: ${error.message}`); }
  render();
  setBusy(false);
  loadModels();
}
function checkSession() {
  if (sameUser()) return;
  request?.abort();
  modelsRequest?.abort();
  clearTimeout(saveTimer);
  storageKey = null;
  doc = { version: 1, nodes: [], view: { x: canvas.clientWidth / 2, y: canvas.clientHeight / 2, scale: 1 } };
  selected = null;
  render();
  $('node-text').value = '';
  $('model').replaceChildren(new Option(t.modelsFailed, ''));
  $('model').disabled = true;
  setBusy(false);
  saveMessage(t.noStorage);
  message('User changed; reload the canvas', true);
}
window.addEventListener('storage', event => { if (event.key === 'auth_user' || event.key === null) checkSession(); });
window.addEventListener('focus', checkSession);
window.addEventListener('pagehide', () => { request?.abort(); modelsRequest?.abort(); clearTimeout(saveTimer); if (storageKey && sameUser()) { try { localStorage.setItem(storageKey, JSON.stringify(snapshot())); } catch { /* storage may be full */ } } });
initialize();
