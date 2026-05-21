import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  // Static export for Firebase Hosting (no SSR/middleware/API routes; SPA
  // rewrite in firebase.json handles client-side routing). Outputs apps/web/out.
  output: 'export',
  transpilePackages: ['@keyhole-koro/paper-in-paper'],
  turbopack: {
    root: path.join(process.cwd(), "../../"),
  },
  experimental: {
    externalDir: true,
  },
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
