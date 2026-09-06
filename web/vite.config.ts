import { defineConfig, type ProxyOptions } from "vite";
import react from "@vitejs/plugin-react";

function apiProxy(): ProxyOptions {
  return {
    target: "http://127.0.0.1:8080",
    changeOrigin: true,
    timeout: 120_000,
    proxyTimeout: 120_000,
  };
}

const proxy = {
  "/graphql": apiProxy(),
  "/auth": apiProxy(),
  "/billing": apiProxy(),
};

const allowedHosts = [".loca.lt", ".trycloudflare.com", ".ngrok-free.app", ".ngrok.io"];

export default defineConfig({
  plugins: [react()],
  base: process.env.ELECTRON === "1" ? "./" : "/",
  server: {
    port: 3001,
    strictPort: true,
    host: true,
    // Dev tunnels fail Vite 6 host check unless these suffixes are allowed.
    allowedHosts,
    proxy,
  },
  preview: {
    port: 3001,
    strictPort: true,
    host: true,
    allowedHosts,
    proxy,
  },
});
