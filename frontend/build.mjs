/** @file @brief Compile the restored public pages before assembling the fixed console. */
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assemble } from './kedaya/assemble.mjs';

const root = path.dirname(fileURLToPath(import.meta.url));

export function buildPublicPages() {
  execFileSync(process.execPath, [path.join(root, 'node_modules/vite/bin/vite.js'), 'build', '--config', 'public-pages/vite.config.ts'], {
    cwd: root,
    stdio: 'inherit',
  });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  buildPublicPages();
  const result = assemble();
  console.log(`Production frontend: ${result.publishedAssets.length} assets; ${result.changes.length} console patches; entries=${Object.keys(result.entries).join(',')}`);
}
