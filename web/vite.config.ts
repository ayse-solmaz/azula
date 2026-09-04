import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: process.env.ELECTRON === "1" ? "./" : "/",
  server: {
    port: 3000,
    proxy: {
      "/graphql": "http://localhost:8080",
    },
  },
});
