/**
 * PostgreSQL connection pooling singleton for server-side route handlers.
 *
 * @module
 */
import { Pool } from 'pg';
import { config } from '../config';

// vite dev で HMR が走るたびに新しい Pool ができないよう、
// globalThis にキャッシュする。
declare global {
  // eslint-disable-next-line no-var
  var __logViewerPgPool: Pool | undefined;
}

/** Creates a new PostgreSQL client pool using monitor database configuration. */
function createPool(): Pool {
  return new Pool({
    connectionString: config.MONITOR_DATABASE_URL,
    max: 5,
    idleTimeoutMillis: 30_000,
  });
}

/**
 * Returns the global PostgreSQL pool singleton, creating it if not already initialized.
 *
 * @returns Cached or newly initialized pg.Pool instance. The pool is stored on globalThis
 *   so vite dev reloads do not create one pool per module evaluation.
 * @throws Pool construction errors for invalid database configuration; connection failures
 *   are reported by individual queries.
 */
export function getPool(): Pool {
  if (!globalThis.__logViewerPgPool) {
    globalThis.__logViewerPgPool = createPool();
  }
  return globalThis.__logViewerPgPool;
}
