const { contextBridge } = require("electron");

function arg(prefix) {
  const hit = process.argv.find((a) => a.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : "";
}

contextBridge.exposeInMainWorld("azulaDesktop", {
  shell: true,
  graphqlUrl: arg("--azula-gql=") || "http://localhost:8080/graphql",
  deviceId: arg("--azula-device="),
  deviceName: "Azula Desktop",
});
