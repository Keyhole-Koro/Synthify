import { z } from 'zod';

const envSchema = z.object({
  // BFF (Next.js Route Handlers) は MONITOR_DATABASE_URL で Postgres を直接参照する。
  // monitor ロールに GRANT SELECT が与えられた read-only DSN を指す想定
  // (db/init/004_monitor_role.sql)。
  MONITOR_DATABASE_URL: z
    .string()
    .default('postgres://monitor@127.0.0.1:5432/synthify?sslmode=disable'),
});

const processEnv = {
  MONITOR_DATABASE_URL: process.env.MONITOR_DATABASE_URL,
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
