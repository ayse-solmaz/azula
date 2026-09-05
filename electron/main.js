const { app, BrowserWindow, ipcMain, safeStorage, session } = require("electron");
const fs = require("fs");
const path = require("path");
const { randomUUID } = require("crypto");

const WEB_URL = process.env.ELECTRON_WEB_URL || process.env.WEB_URL || "http://localhost:3001";
const API_GRAPHQL = process.env.AZULA_GRAPHQL || "http://127.0.0.1:8080/graphql";

app.disableHardwareAcceleration();

let memoryToken = "";
let lastTarget = { kind: "url", value: WEB_URL };

function sessionFile() {
  return path.join(app.getPath("userData"), "session.bin");
}

function loadToken() {
  if (memoryToken) return memoryToken;
  try {
    const buf = fs.readFileSync(sessionFile());
    memoryToken = safeStorage.isEncryptionAvailable()
      ? safeStorage.decryptString(buf)
      : buf.toString("utf8");
    return memoryToken;
  } catch {
    return "";
  }
}

function saveToken(token) {
  memoryToken = typeof token === "string" ? token : "";
  if (!memoryToken) {
    try {
      fs.unlinkSync(sessionFile());
    } catch {
      /* missing */
    }
    return;
  }
  const payload = safeStorage.isEncryptionAvailable()
    ? safeStorage.encryptString(memoryToken)
    : Buffer.from(memoryToken, "utf8");
  fs.mkdirSync(path.dirname(sessionFile()), { recursive: true });
  fs.writeFileSync(sessionFile(), payload);
}

function deviceId() {
  const file = path.join(app.getPath("userData"), "device.json");
  try {
    const raw = JSON.parse(fs.readFileSync(file, "utf8"));
    if (raw && typeof raw.id === "string" && raw.id) return raw.id;
  } catch {
    /* first launch */
  }
  const id = randomUUID();
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify({ id }, null, 2));
  return id;
}

function installAuthHeader() {
  session.defaultSession.webRequest.onBeforeSendHeaders(
    { urls: ["http://127.0.0.1:8080/*", "http://localhost:8080/*"] },
    (details, callback) => {
      const token = loadToken();
      if (token && details.method !== "OPTIONS") {
        details.requestHeaders.Authorization = `Bearer ${token}`;
      }
      callback({ requestHeaders: details.requestHeaders });
    }
  );
}

function esc(s) {
  return String(s || "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[c]));
}

function recoveryHTML(tried, reason) {
  return `<!DOCTYPE html>
<html lang="tr">
<head>
  <meta charset="utf-8" />
  <title>Azula</title>
  <style>
    body { font-family: "Segoe UI", sans-serif; background:#03045e; color:#fff; margin:0; padding:2.5rem; max-width:40rem; line-height:1.5; }
    h1 { font-size:1.55rem; margin:0 0 1rem; }
    p { margin:0 0 1rem; }
    .box { background:#04156e; border:1px solid rgba(144,224,239,.28); border-radius:12px; padding:1rem 1.1rem; margin:1rem 0; }
    code { background:#05207c; padding:2px 6px; border-radius:4px; }
    button { background:#00b4d8; color:#03045e; border:0; padding:10px 16px; border-radius:8px; font-weight:700; cursor:pointer; }
    button:hover { filter:brightness(1.08); }
    .muted { opacity:.85; font-size:.92rem; }
  </style>
</head>
<body>
  <h1>Azula açılamadı / Could not open Azula</h1>
  <p><strong>Türkçe.</strong> Masaüstü penceresi arayüzü yükleyemedi. İki yol var:</p>
  <p>(a) API ve web’i yerelde çalıştırın: <code>go run ./cmd/api</code> ve <code>cd web && npm run dev</code> — adres <code>http://localhost:3001</code>.</p>
  <p>(b) Paketi derleyin ki <code>electron/web</code> oluşsun: <code>scripts\\azula.cmd</code> veya <code>scripts\\start-desktop.ps1</code>.</p>
  <p class="muted"><strong>English.</strong> The desktop window could not load the UI. Either (a) run the API and web locally (<code>go run ./cmd/api</code> and <code>cd web && npm run dev</code> at <code>http://localhost:3001</code>), or (b) build/pack so <code>electron/web</code> exists (<code>scripts\\azula.cmd</code> or <code>scripts\\start-desktop.ps1</code>).</p>
  <div class="box">
    <p>Denenen adres / URL tried: <code>${esc(tried)}</code></p>
    <p class="muted">${esc(reason)}</p>
  </div>
  <button type="button" id="retry">Tekrar dene / Retry</button>
  <script>
    document.getElementById("retry").onclick = function () {
      if (window.azulaDesktop && window.azulaDesktop.retryLoad) window.azulaDesktop.retryLoad();
    };
  </script>
</body>
</html>`;
}

function showRecovery(win, tried, reason) {
  win.loadURL("data:text/html;charset=utf-8," + encodeURIComponent(recoveryHTML(tried, reason))).catch(() => {
    if (!win.isVisible()) win.show();
  });
}

function loadTarget(win) {
  if (lastTarget.kind === "file") {
    return win.loadFile(lastTarget.value);
  }
  return win.loadURL(lastTarget.value);
}

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 840,
    show: false,
    backgroundColor: "#03045e",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  win.once("ready-to-show", () => {
    if (!win.isDestroyed()) win.show();
  });
  setTimeout(() => {
    if (!win.isDestroyed() && !win.isVisible()) win.show();
  }, 2500);

  win.webContents.on("did-fail-load", (_e, code, desc, url, isMainFrame) => {
    if (!isMainFrame) return;
    if (String(url || "").startsWith("data:")) return;
    showRecovery(win, url || lastTarget.value, `${code} ${desc}`.trim());
  });

  const bundled = path.join(__dirname, "web", "index.html");
  if (fs.existsSync(bundled)) {
    lastTarget = { kind: "file", value: bundled };
  } else {
    lastTarget = { kind: "url", value: WEB_URL };
  }

  loadTarget(win).catch((err) => {
    showRecovery(win, lastTarget.value, String(err));
  });

  return win;
}

app.whenReady().then(() => {
  installAuthHeader();
  ipcMain.on("azula:gql", (event) => {
    event.returnValue = API_GRAPHQL;
  });
  ipcMain.on("azula:device", (event) => {
    event.returnValue = deviceId();
  });
  ipcMain.on("azula:session:get", (event) => {
    event.returnValue = loadToken();
  });
  ipcMain.on("azula:session:has", (event) => {
    event.returnValue = Boolean(loadToken());
  });
  ipcMain.on("azula:session:set", (event, token) => {
    saveToken(typeof token === "string" ? token : "");
    event.returnValue = true;
  });
  ipcMain.on("azula:retry", (event) => {
    const win = BrowserWindow.fromWebContents(event.sender);
    if (!win) {
      event.returnValue = false;
      return;
    }
    loadTarget(win).catch((err) => {
      showRecovery(win, lastTarget.value, String(err));
    });
    event.returnValue = true;
  });
  createWindow();
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
}).catch((err) => {
  console.error(err);
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
