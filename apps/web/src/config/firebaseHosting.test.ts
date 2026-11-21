import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

// The static export writes each route as <route>.html (view.html for /view). Firebase
// Hosting matches /view to view.html only when cleanUrls is on; without it every path
// but / falls through to the "**" rewrite and gets index.html, the landing page. CI
// runs `next dev`, which routes by itself, so only this check catches a regression.
describe('firebase.json hosting', () => {
  const firebaseJson = path.resolve(__dirname, '../../../../firebase.json');
  const hosting = JSON.parse(readFileSync(firebaseJson, 'utf8')).hosting;

  it('serves exported pages at their clean URLs', () => {
    expect(hosting.public).toBe('apps/web/out');
    expect(hosting.cleanUrls).toBe(true);
  });

  it('still sends paths with no file to the app', () => {
    expect(hosting.rewrites.at(-1)).toEqual({ source: '**', destination: '/index.html' });
  });
});
