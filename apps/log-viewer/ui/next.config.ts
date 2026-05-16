import type { NextConfig } from 'next';
import path from 'node:path';

const nextConfig: NextConfig = {
  transpilePackages: ['@synthify/proto-ts'],
  experimental: {
    externalDir: true,
  },
  turbopack: {
    root: path.join(__dirname, '..', '..', '..'),
  },
  output: 'standalone',
  images: { unoptimized: true },
};

export default nextConfig;
