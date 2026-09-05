const { app, BrowserWindow, ipcMain, safeStorage, session } = require("electron");
const fs = require("fs");
const path = require("path");
const { randomUUID } = require("crypto");

const WEB_URL = process.env.ELECTRON_WEB_URL || process.env.WEB_URL || "http://localhost:3001";
const API_GRAPHQL = process.env.AZULA_GRAPHQL || "http://127.0.0.1:8080/graphql";

app.disableHardwareAcceleration();

let memoryToken = "";

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

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 840,
    show: true,
    backgroundColor: "#03045e",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  win.show();
  win.webContents.on("did-fail-load", (_e, code, desc, url) => {
    win.loadURL(
      "data:text/html;charset=utf-8," +
        encodeURIComponent(
          `<body style="font-family:Segoe UI;background:#03045e;color:#fff;padding:2rem">
           <h1>Azula yüklenemedi</h1>
           <p>${code} ${desc}</p>
           <p>${url}</p>
           </body>`
        )
    );
  });
  const bundled = path.join(__dirname, "web", "index.html");
  if (fs.existsSync(bundled)) {
    win.loadFile(bundled).catch((err) => {
      win.loadURL(
        "data:text/html;charset=utf-8," +
          encodeURIComponent(`<body style="color:#fff;background:#03045e;padding:2rem"><h1>Azula</h1><p>${String(err)}</p></body>`)
      );
    });
  } else {
    win.loadURL(WEB_URL).catch((err) => {
      win.loadURL(
        "data:text/html;charset=utf-8," +
          encodeURIComponent(`<body style="color:#fff;background:#03045e;padding:2rem"><h1>Azula</h1><p>${String(err)}</p><p>${WEB_URL}</p></body>`)
      );
    });
  }
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
