/**
 * Production server for the Monitor UI: serves the built client assets and
 * hands every other request (pages and /api routes) to the TanStack Start
 * server bundle. Run after `vite build` with `bun server.ts`.
 *
 * @module
 */
import path from 'node:path';
import handler from './dist/server/server.js';

const clientDir = path.resolve(import.meta.dir, 'dist/client');
const port = Number(process.env.PORT ?? 8080);

Bun.serve({
  port,
  hostname: '0.0.0.0',
  async fetch(request) {
    const { pathname } = new URL(request.url);
    if (pathname.startsWith('/assets/')) {
      const filePath = path.resolve(clientDir, `.${decodeURIComponent(pathname)}`);
      // Stay inside dist/client even if the path contains "..".
      if (filePath.startsWith(clientDir + path.sep)) {
        const file = Bun.file(filePath);
        if (await file.exists()) {
          return new Response(file, {
            headers: { 'Cache-Control': 'public, max-age=31536000, immutable' },
          });
        }
      }
    }
    return handler.fetch(request);
  },
});

console.log(`monitor listening on :${port}`);
