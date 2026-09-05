import { spawn } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const entry = path.join(dir, "dist", "index.js");
const child = spawn(process.execPath, [entry], { stdio: "inherit", cwd: dir });
child.on("exit", (code) => process.exit(code ?? 1));
