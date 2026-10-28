/** @file @brief Serve the assembled review base for local frontend development. */
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assemble } from './assemble.mjs';
import { buildPublicPages } from '../build.mjs';

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, '../../backend/internal/web/dist');
const port = Number(process.env.VITE_DEV_PORT || process.env.PORT || 3000);
const host = process.env.VITE_DEV_HOST || '0.0.0.0';
const backendUrl = (process.env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080').replace(/\/$/, '');

buildPublicPages();
assemble({ outputDir: output });

const mime = new Map([
  ['.css', 'text/css; charset=utf-8'],
  ['.html', 'text/html; charset=utf-8'],
  ['.js', 'text/javascript; charset=utf-8'],
  ['.mjs', 'text/javascript; charset=utf-8'],
  ['.json', 'application/json; charset=utf-8'],
  ['.svg', 'image/svg+xml'],
  ['.png', 'image/png'],
  ['.jpg', 'image/jpeg'],
  ['.woff2', 'font/woff2'],
]);

function body(request) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    request.on('data', chunk => chunks.push(chunk));
    request.on('end', () => resolve(Buffer.concat(chunks)));
    request.on('error', reject);
  });
}

async function proxy(request, response) {
  const payload = request.method === 'GET' || request.method === 'HEAD' ? undefined : await body(request);
  const headers = { ...request.headers };
  delete headers.host;
  delete headers.connection;
  const upstream = await fetch(`${backendUrl}${request.url}`, {
    method: request.method,
    headers,
    body: payload,
  });
  response.writeHead(upstream.status, Object.fromEntries([...upstream.headers].filter(([key]) => key !== 'transfer-encoding')));
  response.end(Buffer.from(await upstream.arrayBuffer()));
}

const server = http.createServer(async (request, response) => {
  try {
    if (/^(?:\/api|\/v1|\/setup)(?:\/|$)/.test(request.url || '')) {
      await proxy(request, response);
      return;
    }
    const pathname = decodeURIComponent(new URL(request.url || '/', 'http://localhost').pathname);
    const relative = pathname === '/' ? 'index.html' : pathname.replace(/^\/+/, '');
    const file = path.resolve(output, relative);
    const inside = file === output || file.startsWith(`${output}${path.sep}`);
    const candidate = inside && fs.existsSync(file) && fs.statSync(file).isDirectory() ? path.join(file, 'index.html') : file;
    const target = inside && fs.existsSync(candidate) && fs.statSync(candidate).isFile() ? candidate : path.join(output, 'index.html');
    response.writeHead(200, { 'content-type': mime.get(path.extname(target)) || 'application/octet-stream' });
    fs.createReadStream(target).pipe(response);
  } catch (error) {
    response.writeHead(502, { 'content-type': 'text/plain; charset=utf-8' });
    response.end(String(error?.stack || error));
  }
});

server.listen(port, host, () => {
  console.log(`Kedaya review base dev server: http://${host === '0.0.0.0' ? '127.0.0.1' : host}:${port}`);
  console.log(`API proxy target: ${backendUrl}`);
});
