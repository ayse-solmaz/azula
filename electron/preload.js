const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("azulaDesktop", {
  shell: true,
  graphqlUrl: () => ipcRenderer.sendSync("azula:gql") || "http://127.0.0.1:8080/graphql",
  deviceId: ipcRenderer.sendSync("azula:device") || "",
  deviceName: "Azula Desktop",
  getToken: () => ipcRenderer.sendSync("azula:session:get") || "",
  hasSession: () => Boolean(ipcRenderer.sendSync("azula:session:has")),
  setSession: (token) => ipcRenderer.sendSync("azula:session:set", token || ""),
  retryLoad: () => ipcRenderer.sendSync("azula:retry"),
});
