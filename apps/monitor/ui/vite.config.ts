/**
 * Vite + TanStack Start configuration for the Synthify Monitor UI.
 *
 * @module
 */
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { tanstackStart } from '@tanstack/react-start/plugin/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

const root = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  root,
  // NEXT_PUBLIC_* stays readable in the browser so the deploy variables keep their names.
  envPrefix: ['VITE_', 'NEXT_PUBLIC_'],
  resolve: {
    alias: { '@': path.resolve(root, 'src') },
  },
  server: {
    host: '0.0.0.0',
    port: 5174,
    strictPort: true,
  },
  // firebase-admin and pg are Node-only; load them at runtime instead of bundling.
  ssr: {
    external: ['firebase-admin', 'pg'],
  },
  plugins: [tailwindcss(), tanstackStart(), react()],
});
