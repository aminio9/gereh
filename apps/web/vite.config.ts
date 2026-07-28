import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, process.cwd(), "");

  return {
    appType: "spa",
    plugins: [react()],
    server: {
      host: "0.0.0.0",
      port: 5173,
      strictPort: true,
      proxy: {
        "/v1": {
          target: environment.VITE_DEV_API_PROXY ?? "http://localhost:8080",
          changeOrigin: true,
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
