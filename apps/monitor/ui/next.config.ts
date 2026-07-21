import type { NextConfig } from 'next';
import path from 'node:path';

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
  output: 'standalone',
  images: { unoptimized: true },
};

export default nextConfig;
