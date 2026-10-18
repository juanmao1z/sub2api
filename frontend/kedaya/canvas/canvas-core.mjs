export const MAX_DOCUMENT_BYTES = 3 * 1024 * 1024;
export const MAX_NODES = 100;
const MAX_IMAGE_LENGTH = 2 * 1024 * 1024;
const position = value => Number.isFinite(value) && Math.abs(value) <= 100000;
const ownKeys = (value, expected) => Object.keys(value).every(key => expected.includes(key));

export function sameOriginBase(value, origin) {
  if (!value) return origin;
  const url = new URL(value, origin);
  if (url.origin !== origin || !['http:', 'https:'].includes(url.protocol) ||
      url.username || url.password || url.search || url.hash || url.pathname !== '/') {
    throw new Error('API address must be this site origin');
  }
  return origin;
}

export function safeImageSource(value, secret = '', origin = '') {
  if (typeof value !== 'string' || value.length > MAX_IMAGE_LENGTH) return false;
  if (secret && (value.includes(secret) || value.includes(encodeURIComponent(secret)))) return false;
  if (/^data:image\/(png|jpeg|webp);base64,[a-z\d+/]+={0,2}$/i.test(value)) return true;
  try {
    const url = new URL(value, origin || undefined);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return false;
    if (secret && decodeURIComponent(url.href).includes(secret)) return false;
    return true;
  } catch { return false; }
}

export function validateDocument(input, secret = '', origin = '') {
  if (!input || typeof input !== 'object' || Array.isArray(input) ||
      !ownKeys(input, ['version', 'nodes', 'view']) || input.version !== 1 ||
      !Array.isArray(input.nodes) || input.nodes.length > MAX_NODES) {
    throw new Error('Invalid canvas document');
  }
  const view = input.view;
  if (!view || typeof view !== 'object' || Array.isArray(view) ||
      !ownKeys(view, ['x', 'y', 'scale']) || !position(view.x) || !position(view.y) ||
      !Number.isFinite(view.scale) || view.scale < 0.25 || view.scale > 3) {
    throw new Error('Invalid canvas view');
  }
  const ids = new Set();
  const nodes = input.nodes.map(node => {
    if (!node || typeof node !== 'object' || Array.isArray(node) ||
        !ownKeys(node, ['id', 'type', 'x', 'y', 'text', 'src']) ||
        typeof node.id !== 'string' || !/^[a-zA-Z0-9-]{1,64}$/.test(node.id) ||
        ids.has(node.id) || !['text', 'note', 'image'].includes(node.type) ||
        !position(node.x) || !position(node.y) ||
        typeof node.text !== 'string' || node.text.length > 4000 ||
        (secret && node.text.includes(secret))) {
      throw new Error('Invalid or sensitive canvas node');
    }
    ids.add(node.id);
    if (node.type === 'image') {
      if (!safeImageSource(node.src, secret, origin)) throw new Error('Unsafe image source');
      return { id: node.id, type: node.type, x: node.x, y: node.y, text: node.text, src: node.src };
    }
    if (node.src !== undefined) throw new Error('Unexpected image source');
    return { id: node.id, type: node.type, x: node.x, y: node.y, text: node.text };
  });
  const clean = { version: 1, view: { x: view.x, y: view.y, scale: view.scale }, nodes };
  if (new Blob([JSON.stringify(clean)]).size > MAX_DOCUMENT_BYTES) throw new Error('Canvas is too large');
  return clean;
}

export function parseDocument(text, secret = '', origin = '') {
  if (typeof text !== 'string' || new Blob([text]).size > MAX_DOCUMENT_BYTES) throw new Error('File exceeds 3 MB');
  return validateDocument(JSON.parse(text), secret, origin);
}
