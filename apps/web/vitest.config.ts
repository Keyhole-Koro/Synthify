import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  envDir: path.resolve(import.meta.dirname, '../..'),
  envPrefix: ['VITE_', 'NEXT_PUBLIC_'],
  test: {
    environment: 'jsdom',
    globals: true,
    exclude: ['**/node_modules/**', '**/vender/**', 'e2e/**'],
  },
  resolve: {
    // The vendored library may have its own node_modules (when it is developed in place);
    // React must still resolve to this app's single copy.
    dedupe: ['react', 'react-dom'],
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
      // Same as vite.config.ts: the vendored library is consumed from source.
      '@keyhole-koro/paper-in-paper': path.resolve(import.meta.dirname, 'vender/paper-in-paper/src/lib/index.ts'),
    },
  },
});
