import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: process.env.ELECTRON === "1" ? "./" : "/",
  server: {
    port: 3001,
    strictPort: true,
    proxy: {
      "/graphql": "http://127.0.0.1:8080",
      "/auth": "http://127.0.0.1:8080",
      "/billing": "http://127.0.0.1:8080",
    },
  },
});
