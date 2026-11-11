/**
 * Mode for Content Security Policy header delivery ('report' for report-only or 'enforce' for active enforcement).
 */
export type CspMode = 'report' | 'enforce';

/**
 * Configuration options for generating HTTP security headers and CSP directives.
 *
 * @property isProduction Whether development-only origins must be omitted.
 * @property cspMode Whether CSP is enforced or emitted in report-only mode.
 * @property devOrigins Additional origins allowed only for local development tooling.
 */
export type SecurityHeaderOptions = {
  isProduction: boolean;
  cspMode?: CspMode;
  devOrigins?: string[];
};

/**
 * Parses and validates a raw CSP mode value, defaulting to 'report' if not 'enforce'.
 *
 * @param value - Raw input value.
 * @returns The resolved CspMode.
 */
export function parseCspMode(value: unknown): CspMode {
  return value === 'enforce' ? 'enforce' : 'report';
}

const THIRD_PARTY = {
  firebaseAuthFrames: [
    'https://accounts.google.com',
    'https://*.firebaseapp.com',
    'https://synthify-491705.firebaseapp.com',
    'https://synthify-stage-491705.firebaseapp.com',
  ],
  googleConnect: [
    'https://*.googleapis.com',
    'https://identitytoolkit.googleapis.com',
    'https://securetoken.googleapis.com',
    'https://*.firebaseio.com',
    'https://*.firebaseapp.com',
  ],
  newRelicConnect: [
    'https://*.nr-data.net',
  ],
  backendConnect: [
    'https://*.run.app',
    'https://synthify.keyhole.work',
    'https://stage.synthify.keyhole.work',
  ],
  scripts: [
    'https://apis.google.com',
    'https://www.gstatic.com',
  ],
  fonts: [
    'https://fonts.gstatic.com',
  ],
  styles: [
    'https://fonts.googleapis.com',
  ],
} as const;

function directive(name: string, sources: string[]) {
  return `${name} ${sources.join(' ')}`;
}

/**
 * Constructs the Content Security Policy directive string based on environment and options.
 *
 * @param options - Security header generation options. In production, `devOrigins` is
 *   ignored so local emulator endpoints cannot accidentally enter the deployed policy.
 * @returns The semicolon-delimited CSP string containing the complete directive set.
 */
export function buildCsp(options: SecurityHeaderOptions): string {
  const dev = options.isProduction ? [] : (options.devOrigins ?? []);

  const scriptSrc = [
    "'self'",
    "'unsafe-inline'",
    "'unsafe-eval'",
    ...THIRD_PARTY.scripts,
    ...dev,
  ];

  const connectSrc = [
    "'self'",
    ...THIRD_PARTY.googleConnect,
    ...THIRD_PARTY.newRelicConnect,
    ...THIRD_PARTY.backendConnect,
    ...dev,
  ];

  const frameSrc = [
    "'self'",
    ...THIRD_PARTY.firebaseAuthFrames,
    ...dev,
  ];

  const imgSrc = [
    "'self'",
    'data:',
    'blob:',
    'https://*.googleusercontent.com',
    ...dev,
  ];

  const fontSrc = [
    "'self'",
    'data:',
    ...THIRD_PARTY.fonts,
    ...dev,
  ];

  const styleSrc = [
    "'self'",
    "'unsafe-inline'",
    ...THIRD_PARTY.styles,
    ...dev,
  ];

  const directives = [
    directive('default-src', ["'self'"]),
    directive('base-uri', ["'self'"]),
    directive('object-src', ["'none'"]),
    directive('frame-ancestors', ["'none'"]),
    directive('frame-src', frameSrc),
    directive('connect-src', connectSrc),
    directive('img-src', imgSrc),
    directive('font-src', fontSrc),
    directive('style-src', styleSrc),
    directive('script-src', scriptSrc),
  ];

  return directives.join('; ');
}

/**
 * Builds standard security response headers including HSTS, X-Frame-Options, and CSP.
 *
 * @param options - Security header generation options passed to CSP construction.
 * @returns Map of header names to values suitable for a Fetch `Headers`/Response object.
 *   The CSP header name is `Content-Security-Policy` in enforce mode and
 *   `Content-Security-Policy-Report-Only` otherwise.
 */
export function buildSecurityHeaders(options: SecurityHeaderOptions): Record<string, string> {
  const cspHeaderName = options.cspMode === 'enforce'
    ? 'Content-Security-Policy'
    : 'Content-Security-Policy-Report-Only';

  const headers: Record<string, string> = {
    'X-Frame-Options': 'SAMEORIGIN',
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()',
    'Strict-Transport-Security': 'max-age=31536000; includeSubDomains',
    [cspHeaderName]: buildCsp(options),
  };

  return headers;
}
