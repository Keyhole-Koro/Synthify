import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, loadEnv, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { buildSecurityHeaders, parseCspMode } from './src/shared/security-headers';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function devSecurityHeaders(isProduction: boolean, cspMode: 'report' | 'enforce'): Plugin {
  const headers = buildSecurityHeaders({
    isProduction,
    cspMode,
    devOrigins: isProduction
      ? []
      : ['http://127.0.0.1:*', 'http://localhost:*', 'ws://127.0.0.1:*', 'ws://localhost:*'],
  });

  return {
    name: 'synthify-dev-security-headers',
    configureServer(server) {
      server.middlewares.use((_req, res, next) => {
        for (const [name, value] of Object.entries(headers)) {
          res.setHeader(name, value);
        }
        next();
      });
    },
  };
}

export default defineConfig(({ mode }) => {
  const isProduction = mode === 'production';
  const env = loadEnv(mode, path.resolve(__dirname, '../..'), '');
  const localEnv = loadEnv(mode, __dirname, '');
  const merged = { ...env, ...localEnv };

  const cspMode = parseCspMode(merged.CSP_MODE);

  return {
    root: __dirname,
    envDir: path.resolve(__dirname, '../..'),
    publicDir: path.resolve(__dirname, 'public'),
    envPrefix: ['VITE_', 'NEXT_PUBLIC_'],
    plugins: [
      tanstackRouter({
        target: 'react',
        autoCodeSplitting: true,
        routesDirectory: path.resolve(__dirname, 'src/client/routes'),
        generatedRouteTree: path.resolve(__dirname, 'src/client/routeTree.gen.ts'),
      }),
      react(),
      devSecurityHeaders(isProduction, cspMode),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, 'src'),
        '@keyhole-koro/paper-in-paper': path.resolve(__dirname, 'vender/paper-in-paper/src/lib/index.ts'),
        '@keyhole-koro/paper-in-paper/*': path.resolve(__dirname, 'vender/paper-in-paper/src/lib/*'),
      },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      strictPort: true,
      watch: process.env.CHOKIDAR_USEPOLLING === 'true' ? { usePolling: true, interval: 300 } : undefined,
    },
    build: {
      outDir: path.resolve(__dirname, 'out'),
      emptyOutDir: true,
      sourcemap: false,
    },
  };
});
