import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Serves the renderer preview harness in `preview/`. Not part of the packaged app.
export default defineConfig({
  root: "preview",
  plugins: [react()],
  server: { port: 5199, strictPort: true },
});
