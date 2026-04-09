import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ['@keyhole-koro/paper-in-paper'],
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
