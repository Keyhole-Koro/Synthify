import { z } from 'zod';

const envSchema = z.object({
  // BFF (Next.js Route Handlers) は LOG_VIEWER_DATABASE_URL で Postgres を直接参照する。
  // log_viewer ロールに GRANT SELECT が与えられた read-only DSN を指す想定
  // (db/init/004_log_viewer_role.sql)。
  LOG_VIEWER_DATABASE_URL: z
    .string()
    .default('postgres://log_viewer@127.0.0.1:5432/synthify?sslmode=disable'),
});

const processEnv = {
  LOG_VIEWER_DATABASE_URL: process.env.LOG_VIEWER_DATABASE_URL,
};

const parsed = envSchema.safeParse(processEnv);

if (!parsed.success) {
  console.error(
    '❌ Invalid environment variables:',
    parsed.error.flatten().fieldErrors
  );
  throw new Error('Invalid environment variables');
}

export const config = parsed.data;
