import { fileURLToPath } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The build is the viewer the skill copies next to each tree.json: fixed file names, relative
// URLs (the page is served under a path we do not know), everything in one script.
export default defineConfig({
  base: './',
  plugins: [react()],
  build: {
    outDir: fileURLToPath(new URL('../../skills/synthify-tree/assets/viewer', import.meta.url)),
    emptyOutDir: true,
    assetsDir: '',
    target: 'es2022',
    sourcemap: false,
    rollupOptions: {
      output: { entryFileNames: 'viewer.js', chunkFileNames: '[name].js', assetFileNames: '[name][extname]' },
    },
  },
});
