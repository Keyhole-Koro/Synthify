import { z } from 'zod';

const envSchema = z.object({
  // BFF (Next.js Route Handlers) は MONITOR_DATABASE_URL で Postgres を直接参照する。
  // monitor ロールに GRANT SELECT が与えられた read-only DSN を指す想定
  // (db/migrations/0014_monitor_role.up.sql, view は 0013_monitor_views.up.sql)。
  MONITOR_DATABASE_URL: z
    .string()
    .default('postgres://monitor@127.0.0.1:5432/synthify?sslmode=disable'),

  // Firebase ID トークン検証に使う project ID (apps/api と共通)。
  // FIREBASE_AUTH_EMULATOR_HOST が設定されていればエミュレータに接続する。
  FIREBASE_PROJECT_ID: z.string().default('demo-synthify'),

  // admin として API を叩けるメールの CSV。apps/api の SYNTHIFY_ADMIN_USER_EMAILS
  // と同じ値を指す想定。未設定なら fail-closed で全アクセスを拒否する。
  SYNTHIFY_ADMIN_USER_EMAILS: z.string().default(''),
});

const processEnv = {
  MONITOR_DATABASE_URL: process.env.MONITOR_DATABASE_URL,
  FIREBASE_PROJECT_ID: process.env.FIREBASE_PROJECT_ID,
  SYNTHIFY_ADMIN_USER_EMAILS: process.env.SYNTHIFY_ADMIN_USER_EMAILS,
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
