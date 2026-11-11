import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildSecurityHeaders } from '../apps/web/src/shared/security-headers';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, '..');

export function buildHostingConfig(existing: Record<string, unknown>) {
  const securityHeaders = buildSecurityHeaders({ isProduction: true, cspMode: 'report' });
  const toHeaderList = (headers: Record<string, string>) =>
    Object.entries(headers).map(([key, value]) => ({ key, value }));

  const existingHosting = (existing.hosting as Record<string, unknown>) || {};

  return {
    ...existing,
    hosting: {
      ...existingHosting,
      public: 'apps/web/out',
      ignore: ['firebase.json', '**/.*', '**/node_modules/**'],
      headers: [
        {
          source: '**',
          headers: [...toHeaderList(securityHeaders), { key: 'Cache-Control', value: 'no-cache' }],
        },
        {
          source: '/assets/**',
          headers: [{ key: 'Cache-Control', value: 'public, max-age=31536000, immutable' }],
        },
      ],
      rewrites: [
        {
          source: '**',
          destination: '/index.html',
        },
      ],
    },
  };
}

export function syncHostingConfig(check = false): boolean {
  const firebaseJsonPath = path.join(rootDir, 'firebase.json');
  const existing = JSON.parse(fs.readFileSync(firebaseJsonPath, 'utf8')) as Record<string, unknown>;
  const next = `${JSON.stringify(buildHostingConfig(existing), null, 2)}\n`;
  const current = fs.readFileSync(firebaseJsonPath, 'utf8');

  if (current === next) {
    console.log('firebase.json is up to date.');
    return false;
  }

  if (check) {
    console.error('firebase.json is out of date with security-headers.ts. Run: bun run sync:hosting');
    return true;
  }

  fs.writeFileSync(firebaseJsonPath, next);
  console.log('Updated firebase.json with latest security headers.');
  return false;
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(__filename)) {
  const check = process.argv.includes('--check');
  const hasDrift = syncHostingConfig(check);
  if (check && hasDrift) {
    process.exit(1);
  }
}
