const { test } = require("node:test");
const assert = require("node:assert/strict");
const http = require("http");
const {
  DEFAULT_VITE_URL,
  detectLiveVite,
  pickWebTarget,
  probeHTTP,
  resolveWebTarget,
} = require("./resolve-web");

test("ELECTRON_WEB_URL wins over live Vite and a bundled file", () => {
  const target = resolveWebTarget({
    env: { ELECTRON_WEB_URL: "http://127.0.0.1:3999", WEB_URL: "http://localhost:3001" },
    bundledPath: "/tmp/electron/web/index.html",
    viteLive: true,
    existsSync: () => true,
  });
  assert.equal(target.kind, "url");
  assert.equal(target.value, "http://127.0.0.1:3999");
  assert.equal(target.source, "env");
});

test("WEB_URL is used when ELECTRON_WEB_URL is unset", () => {
  const target = resolveWebTarget({
    env: { WEB_URL: "http://localhost:3001" },
    bundledPath: "/tmp/electron/web/index.html",
    viteLive: false,
    existsSync: () => true,
  });
  assert.equal(target.kind, "url");
  assert.equal(target.value, "http://localhost:3001");
  assert.equal(target.source, "env");
});

test("blank env values do not count as set", () => {
  const target = resolveWebTarget({
    env: { ELECTRON_WEB_URL: "  ", WEB_URL: "" },
    bundledPath: "/abs/electron/web/index.html",
    viteLive: false,
    existsSync: (p) => p === "/abs/electron/web/index.html",
  });
  assert.equal(target.kind, "file");
  assert.equal(target.source, "bundle");
});

test("live Vite is preferred over a stale bundle when env is unset", () => {
  const target = resolveWebTarget({
    env: {},
    bundledPath: "/abs/electron/web/index.html",
    viteLive: true,
    existsSync: () => true,
  });
  assert.equal(target.kind, "url");
  assert.equal(target.value, DEFAULT_VITE_URL);
  assert.equal(target.source, "vite");
});

test("bundle is used when Vite is down and env is unset", () => {
  const bundled = "/abs/electron/web/index.html";
  const target = resolveWebTarget({
    env: {},
    bundledPath: bundled,
    viteLive: false,
    existsSync: (p) => p === bundled,
  });
  assert.equal(target.kind, "file");
  assert.equal(target.value, bundled);
  assert.equal(target.source, "bundle");
});

test("falls back to Vite URL when nothing else is available", () => {
  const target = resolveWebTarget({
    env: {},
    bundledPath: "/missing/index.html",
    viteLive: false,
    existsSync: () => false,
  });
  assert.equal(target.kind, "url");
  assert.equal(target.value, DEFAULT_VITE_URL);
  assert.equal(target.source, "fallback");
});

test("pickWebTarget probes Vite before using the bundle", async () => {
  const target = await pickWebTarget({
    env: {},
    bundledPath: "/abs/electron/web/index.html",
    existsSync: () => true,
    probe: async (url) => url === DEFAULT_VITE_URL,
  });
  assert.equal(target.source, "vite");
  assert.equal(target.value, DEFAULT_VITE_URL);
});

test("pickWebTarget skips the probe when env is set", async () => {
  let probed = false;
  const target = await pickWebTarget({
    env: { ELECTRON_WEB_URL: "http://127.0.0.1:3001" },
    bundledPath: "/abs/electron/web/index.html",
    existsSync: () => true,
    probe: async () => {
      probed = true;
      return true;
    },
  });
  assert.equal(target.source, "env");
  assert.equal(probed, false);
});

test("detectLiveVite tries 127.0.0.1 then localhost", async () => {
  const tried = [];
  const live = await detectLiveVite(async (url) => {
    tried.push(url);
    return url === "http://localhost:3001";
  });
  assert.deepEqual(tried, [DEFAULT_VITE_URL, "http://localhost:3001"]);
  assert.equal(live, "http://localhost:3001");
});

test("probeHTTP is true when a server answers", async () => {
  const server = http.createServer((_req, res) => {
    res.writeHead(200);
    res.end("ok");
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address();
  try {
    assert.equal(await probeHTTP(`http://127.0.0.1:${port}`), true);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
});

test("probeHTTP is false when nothing listens", async () => {
  assert.equal(await probeHTTP("http://127.0.0.1:1", 200), false);
});
