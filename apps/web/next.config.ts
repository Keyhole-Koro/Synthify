import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  transpilePackages: ['@keyhole-koro/paper-in-paper', '@synthify/proto-ts'],
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
