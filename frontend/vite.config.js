import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

/**
 * Vite configuration for TimeSync frontend.
 *
 * In development:  npm run dev
 *   The Vite dev server proxies /api/* and /health to the Go backend at
 *   http://localhost:8080.  This avoids CORS issues during development.
 *
 * In production (Docker):
 *   `npm run build` produces a static bundle in /dist which is served by
 *   the nginx container.  VITE_API_BASE_URL should be set to '' (empty) so
 *   the frontend uses relative paths and nginx handles proxying.
 */
export default defineConfig({
  plugins: [react()],

  server: {
    port: 5173,
    proxy: {
      // Proxy all API calls and SSE requests to the Go backend.
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        // WebSocket / SSE support.
        ws: false,
      },
      "/health": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },

  build: {
    // Emit source maps for production debugging (strip in truly sensitive deployments).
    sourcemap: false,
    // Output to /dist for Docker COPY.
    outDir: "dist",
    // Chunk splitting: keep vendor separate from app code.
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (
            id.includes("node_modules/react/") ||
            id.includes("node_modules/react-dom/")
          ) {
            return "react";
          }
        },
      },
    },
  },
});
