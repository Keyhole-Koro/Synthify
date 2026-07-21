// next dev をプレビューモードで起動 → 準備できたら capture.mjs を実行 → 後始末。
// 実 DB / Firebase は不要 (認証はプレビューフラグでバイパス、データは fixtures)。

import { spawn } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const uiRoot = join(__dirname, '..', '..');

const PORT = process.env.PORT ?? '5174';
const BASE_URL = `http://127.0.0.1:${PORT}`;

const devEnv = {
  ...process.env,
  NODE_ENV: 'development',
  NEXT_PUBLIC_MONITOR_PREVIEW: '1',
  // クライアント Firebase SDK 初期化用のダミー値 (プレビューでは実際には使わない)。
  NEXT_PUBLIC_FIREBASE_API_KEY: process.env.NEXT_PUBLIC_FIREBASE_API_KEY ?? 'demo-api-key',
  NEXT_PUBLIC_FIREBASE_PROJECT_ID: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID ?? 'demo-synthify',
  NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN ?? 'demo-synthify.firebaseapp.com',
  NEXT_PUBLIC_FIREBASE_APP_ID: process.env.NEXT_PUBLIC_FIREBASE_APP_ID ?? '1:1234567890:web:demo-synthify',
};

function waitForServer(url, timeoutMs = 90000) {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const tick = async () => {
      try {
        const res = await fetch(url);
        if (res.ok || res.status === 200) return resolve();
      } catch {
        // not up yet
      }
      if (Date.now() - start > timeoutMs) return reject(new Error(`server not ready: ${url}`));
      setTimeout(tick, 1000);
    };
    tick();
  });
}

function run(cmd, args, opts) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { stdio: 'inherit', ...opts });
    child.on('exit', (code) => (code === 0 ? resolve() : reject(new Error(`${cmd} exited ${code}`))));
    child.on('error', reject);
  });
}

async function main() {
  console.log('→ starting next dev (preview mode)…');
  const dev = spawn('bun', ['run', 'next', 'dev', '--hostname', '127.0.0.1', '--port', PORT], {
    cwd: uiRoot,
    env: devEnv,
    stdio: 'inherit',
  });

  const cleanup = () => {
    if (!dev.killed) dev.kill('SIGTERM');
  };
  process.on('exit', cleanup);
  process.on('SIGINT', () => { cleanup(); process.exit(1); });

  try {
    await waitForServer(BASE_URL);
    console.log('→ server ready, capturing…');
    await run('node', [join(__dirname, 'capture.mjs')], { cwd: uiRoot, env: { ...devEnv, BASE_URL } });
  } finally {
    cleanup();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
