const fs = require("fs");
const http = require("http");
const https = require("https");

const DEFAULT_VITE_URL = "http://127.0.0.1:3001";

function explicitWebURL(env) {
  return String((env && (env.ELECTRON_WEB_URL || env.WEB_URL)) || "").trim();
}

function probeHTTP(targetURL, timeoutMs) {
  const ms = typeof timeoutMs === "number" ? timeoutMs : 600;
  return new Promise((resolve) => {
    let settled = false;
    const done = (ok) => {
      if (settled) return;
      settled = true;
      resolve(Boolean(ok));
    };
    try {
      const u = new URL(targetURL);
      const lib = u.protocol === "https:" ? https : http;
      const req = lib.request(
        {
          hostname: u.hostname,
          port: u.port || (u.protocol === "https:" ? 443 : 80),
          path: u.pathname && u.pathname !== "" ? u.pathname : "/",
          method: "GET",
          timeout: ms,
        },
        (res) => {
          res.resume();
          done(typeof res.statusCode === "number" && res.statusCode > 0 && res.statusCode < 500);
        }
      );
      req.on("error", () => done(false));
      req.on("timeout", () => {
        req.destroy();
        done(false);
      });
      req.end();
    } catch {
      done(false);
    }
  });
}

async function detectLiveVite(probe, timeoutMs) {
  const run = probe || probeHTTP;
  if (await run(DEFAULT_VITE_URL, timeoutMs)) return DEFAULT_VITE_URL;
  if (await run("http://localhost:3001", timeoutMs)) return "http://localhost:3001";
  return "";
}

function resolveWebTarget({
  env = {},
  bundledPath,
  viteLive = false,
  viteURL = DEFAULT_VITE_URL,
  existsSync = fs.existsSync,
} = {}) {
  const explicit = explicitWebURL(env);
  if (explicit) {
    return { kind: "url", value: explicit, source: "env" };
  }
  if (viteLive) {
    return { kind: "url", value: viteURL || DEFAULT_VITE_URL, source: "vite" };
  }
  if (bundledPath && existsSync(bundledPath)) {
    return { kind: "file", value: bundledPath, source: "bundle" };
  }
  return { kind: "url", value: viteURL || DEFAULT_VITE_URL, source: "fallback" };
}

async function pickWebTarget({
  env = process.env,
  bundledPath,
  existsSync = fs.existsSync,
  probe,
  timeoutMs,
} = {}) {
  const explicit = explicitWebURL(env);
  if (explicit) {
    return resolveWebTarget({ env, bundledPath, existsSync });
  }
  const live = await detectLiveVite(probe, timeoutMs);
  return resolveWebTarget({
    env,
    bundledPath,
    existsSync,
    viteLive: Boolean(live),
    viteURL: live || DEFAULT_VITE_URL,
  });
}

module.exports = {
  DEFAULT_VITE_URL,
  detectLiveVite,
  explicitWebURL,
  pickWebTarget,
  probeHTTP,
  resolveWebTarget,
};
