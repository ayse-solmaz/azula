const { app, BrowserWindow } = require("electron");
const fs = require("fs");
const path = require("path");
const { randomUUID } = require("crypto");

const WEB_URL = process.env.ELECTRON_WEB_URL || process.env.WEB_URL || "http://localhost:3000";
const API_GRAPHQL = process.env.AZULA_GRAPHQL || "http://localhost:8080/graphql";

function deviceId() {
  const file = path.join(app.getPath("userData"), "device.json");
  try {
    const raw = JSON.parse(fs.readFileSync(file, "utf8"));
    if (raw && typeof raw.id === "string" && raw.id) return raw.id;
  } catch (_) {
    /* first launch */
  }
  const id = randomUUID();
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify({ id }, null, 2));
  return id;
}

function createWindow() {
  const id = deviceId();
  const win = new BrowserWindow({
    width: 1280,
    height: 840,
    backgroundColor: "#000080",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      additionalArguments: [`--azula-device=${id}`, `--azula-gql=${API_GRAPHQL}`],
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  const bundled = path.join(__dirname, "web", "index.html");
  if (app.isPackaged && fs.existsSync(bundled)) {
    win.loadFile(bundled);
  } else {
    win.loadURL(WEB_URL);
  }
}

app.whenReady().then(() => {
  createWindow();
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
