#!/usr/bin/env bun
//
// Serve apps/web's Next static export the way Firebase Hosting serves it in
// production.
//
// apps/web sets `output: 'export'` (see next.config.ts), so production is a
// pile of static files behind a catch-all rewrite to /index.html
// (firebase.json). `next start` cannot serve that — Next refuses to run it
// against an exported build — so this stands in for Hosting when the e2e stack
// runs against a production build instead of `next dev`.
//
// Deliberately dependency-free: a `bunx serve` here would resolve a version at
// container start, which is exactly how the Playwright browser mismatch got in.
//
// Usage: bun scripts/serve-web-export.mjs [root]   (default apps/web/out)

import { statSync } from 'node:fs';
import { join, resolve, sep } from 'node:path';

const root = resolve(process.argv[2] ?? 'apps/web/out');
const port = Number(process.env.PORT ?? 5173);

function isFile(path) {
  try {
    return statSync(path).isFile();
  } catch {
    return false;
  }
}

if (!isFile(join(root, 'index.html'))) {
  console.error(`[serve-web-export] no static export at ${root} (index.html missing).`);
  console.error('[serve-web-export] build it first: cd apps/web && bun run build');
  process.exit(1);
}

const indexHTML = join(root, 'index.html');

// Hosting resolves a request against the exported tree, then rewrites anything
// left over to /index.html so client-side routing takes it. The export writes
// routes as directories (out/view/index.html), hence the second candidate.
function resolveFile(pathname) {
  let decoded;
  try {
    decoded = decodeURIComponent(pathname);
  } catch {
    return indexHTML;
  }

  const target = resolve(root, `.${decoded}`);
  // resolve() collapses any ../ before this check, so a traversal attempt lands
  // outside root and falls through to the SPA entrypoint rather than escaping.
  if (target !== root && !target.startsWith(root + sep)) return indexHTML;

  for (const candidate of [target, join(target, 'index.html'), `${target}.html`]) {
    if (isFile(candidate)) return candidate;
  }
  return indexHTML;
}

Bun.serve({
  port,
  hostname: '0.0.0.0',
  fetch(request) {
    const { pathname } = new URL(request.url);
    // 200 on the fallback, not 404: that is what the Hosting rewrite does, and
    // the app renders its own not-found states client-side.
    return new Response(Bun.file(resolveFile(pathname)));
  },
});

console.log(`[serve-web-export] serving ${root} on http://0.0.0.0:${port}`);
