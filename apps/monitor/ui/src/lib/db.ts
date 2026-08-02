import { Pool } from 'pg';
import { config, LOCAL_MONITOR_DATABASE_URL } from '../config';

// Next.js の dev mode で HMR が走るたびに新しい Pool ができないよう、
// globalThis にキャッシュする。
declare global {
  // eslint-disable-next-line no-var
  var __logViewerPgPool: Pool | undefined;
}

function createPool(): Pool {
  // MONITOR_DATABASE_URL が未設定のとき、config は local 向けの既定値を返す。それは
  // compose では正しく、デプロイ先では「起動には成功してダッシュボードが何も出さない」
  // という一番気づきにくい壊れ方になる。既定値のまま本番に届いていたら、静かに繋ぎに
  // いくのではなく落とす。
  //
  // 検査をここに置いているのは、Pool を作るのは実際にクエリが要るときだけで、
  // `next build` からは到達しないため。config 側で必須にするとビルドが落ちる。
  const connectionString = config.MONITOR_DATABASE_URL;
  if (
    process.env.NODE_ENV === 'production' &&
    connectionString === LOCAL_MONITOR_DATABASE_URL
  ) {
    throw new Error(
      'MONITOR_DATABASE_URL is not set. Refusing to fall back to the local default ' +
        'in production: the dashboard would start and silently read nothing. ' +
        'See docs/architecture/environment-variables.md.',
    );
  }

  return new Pool({
    connectionString,
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
