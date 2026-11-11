import { describe, expect, it } from 'vitest';
import { buildCsp, buildSecurityHeaders, parseCspMode } from './security-headers';

describe('security headers', () => {
  it('normalizes CSP mode to report unless enforce is explicit', () => {
    expect(parseCspMode('enforce')).toBe('enforce');
    expect(parseCspMode('report')).toBe('report');
    expect(parseCspMode('unexpected')).toBe('report');
  });

  it('includes third-party origins in development only when configured', () => {
    const production = buildCsp({ isProduction: true, devOrigins: ['http://localhost:5173'] });
    const development = buildCsp({ isProduction: false, devOrigins: ['http://localhost:5173'] });
    expect(production).not.toContain('http://localhost:5173');
    expect(development).toContain('http://localhost:5173');
    expect(development).toContain("default-src 'self'");
    expect(development).toContain("object-src 'none'");
  });

  it('selects the requested CSP response header and baseline headers', () => {
    const enforce = buildSecurityHeaders({ isProduction: true, cspMode: 'enforce' });
    const report = buildSecurityHeaders({ isProduction: true });
    expect(enforce['Content-Security-Policy']).toBeTruthy();
    expect(enforce['Content-Security-Policy-Report-Only']).toBeUndefined();
    expect(report['Content-Security-Policy-Report-Only']).toBeTruthy();
    expect(report['X-Content-Type-Options']).toBe('nosniff');
    expect(report['Referrer-Policy']).toBe('strict-origin-when-cross-origin');
  });
});
