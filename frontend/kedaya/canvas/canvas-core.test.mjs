import test from 'node:test';
import assert from 'node:assert/strict';
import { MAX_DOCUMENT_BYTES, parseDocument, sameOriginBase, safeImageSource, validateDocument } from './canvas-core.mjs';

const origin = 'https://example.test';
const empty = { version: 1, view: { x: 0, y: 0, scale: 1 }, nodes: [] };
const node = { id: 'n-1', type: 'note', x: 0, y: 10, text: '<img src=x onerror=alert(1)>' };

test('API origin is restricted, including paths, credentials and protocol-relative hosts', () => {
  assert.equal(sameOriginBase(origin, origin), origin);
  assert.equal(sameOriginBase('', origin), origin);
  for (const url of ['https://other.test', '//other.test', 'https://example.test/v1', 'https://u:p@example.test', 'https://example.test/?token=x', 'javascript:alert(1)']) {
    assert.throws(() => sameOriginBase(url, origin));
  }
});

test('reject unsafe image schemes and credential-bearing or secret-bearing URLs', () => {
  assert.equal(safeImageSource('https://other.test/image.png', 'secret', origin), true);
  for (const url of ['javascript:alert(1)', 'data:image/svg+xml;base64,PHN2Zz4=', 'https://u:p@other.test/a', 'https://other.test/?token=secret', 'https://other.test/%73ecret']) {
    assert.equal(safeImageSource(url, 'secret', origin), false, url);
  }
  assert.equal(safeImageSource('data:image/png;base64,iVBORw0KGgo=', '', origin), true);
});

test('document round trip strips nothing unexpected and keeps text inert', () => {
  const doc = { ...empty, nodes: [node] };
  assert.deepEqual(parseDocument(JSON.stringify(doc), 'secret', origin), doc);
  assert.equal(validateDocument(doc, 'secret', origin).nodes[0].text, node.text);
});

test('malformed, oversized, duplicate, and sensitive documents fail closed', () => {
  const sample = { ...empty, nodes: [node] };
  for (const document of [
    { ...sample, nodes: [node, node] },
    { ...sample, nodes: [{ ...node, src: 'https://example.test/image' }] },
    { ...sample, nodes: [{ ...node, text: 'secret' }] },
    { ...sample, nodes: [{ ...node, type: 'image', src: 'javascript:alert(1)' }] },
    { ...sample, view: { x: Infinity, y: 0, scale: 1 } },
    { ...sample, unexpected: true }
  ]) assert.throws(() => validateDocument(document, 'secret', origin));
  assert.throws(() => parseDocument('x'.repeat(MAX_DOCUMENT_BYTES + 1), '', origin));
});
