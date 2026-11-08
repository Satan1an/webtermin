import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "./src") },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
    // The Monaco chunk (~3.6 MB) is lazy-loaded by Files/Stacks only; every
    // other chunk stays well under Vite's 500 kB default.
    chunkSizeWarningLimit: 4000,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "https://localhost:8443",
        changeOrigin: true,
        secure: false,
        ws: true,
      },
    },
  },
});
