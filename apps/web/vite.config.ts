import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, process.cwd(), "");

  const apiTarget = environment.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8080";
  return {
    appType: "spa",

    plugins: [react()],

    server: {
      host: "0.0.0.0",
      port: 5173,
      strictPort: true,

      proxy: {
        "/v1": {
          target: apiTarget,
          changeOrigin: false,
        },

        "/api": {
          target: apiTarget,
          changeOrigin: false,
        },
      },
    },

    build: {
      target: "es2023",
      outDir: "dist",
      emptyOutDir: true,
      sourcemap: true,
      reportCompressedSize: true,
    },

    preview: {
      host: "0.0.0.0",
      port: 4173,
      strictPort: true,
    },
  };
});
