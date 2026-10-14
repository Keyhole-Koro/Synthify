/**
 * Next.js application configuration for Synthify Monitor UI.
 *
 * @module
 */
import type { NextConfig } from 'next';
import path from 'node:path';

/**
 * Next.js configuration object configuring standalone builds, Turbopack monorepo roots,
 * and externalizing Node-only packages like firebase-admin.
 */
const nextConfig: NextConfig = {
  experimental: {
    externalDir: true,
  },
  // firebase-admin は Node 専用のネイティブ依存を持つため、バンドルせず
  // サーバー実行時に require させる。
  serverExternalPackages: ['firebase-admin'],
  turbopack: {
    root: path.join(__dirname, '..', '..', '..'),
  },
  // monorepo (bun workspaces) なので standalone のトレース基点をリポジトリルート
  // に固定する。これで出力は .next/standalone/apps/monitor/ui/server.js になる。
  outputFileTracingRoot: path.join(__dirname, '..', '..', '..'),
  output: 'standalone',
  images: { unoptimized: true },
};

export default nextConfig;
