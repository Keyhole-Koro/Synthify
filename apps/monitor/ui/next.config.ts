import type { NextConfig } from 'next';
import path from 'node:path';

const nextConfig: NextConfig = {
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
