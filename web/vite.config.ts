import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// In development the API runs on :8080; the proxy keeps the browser same-origin so
// the refresh cookie and WebSocket origin check behave exactly as in production.
const api = process.env.SARATHI_API_URL ?? "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { "/api": { target: api, ws: true } },
  },
  build: {
    sourcemap: false,
    chunkSizeWarningLimit: 1200,
  },
});
