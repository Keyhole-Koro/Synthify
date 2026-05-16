import { Pool } from 'pg';
import { config } from '../config';

// Next.js の dev mode で HMR が走るたびに新しい Pool ができないよう、
// globalThis にキャッシュする。
declare global {
  // eslint-disable-next-line no-var
  var __logViewerPgPool: Pool | undefined;
}

function createPool(): Pool {
  return new Pool({
    connectionString: config.LOG_VIEWER_DATABASE_URL,
    max: 5,
    idleTimeoutMillis: 30_000,
  });
}

export function getPool(): Pool {
  if (!globalThis.__logViewerPgPool) {
    globalThis.__logViewerPgPool = createPool();
  }
  return globalThis.__logViewerPgPool;
}
