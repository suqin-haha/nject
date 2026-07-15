import { mkdir } from "node:fs/promises";
import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const binaryName = process.platform === "win32" ? "nject-lsp.exe" : "nject-lsp";
const output = path.join(root, "bin", binaryName);

await mkdir(path.dirname(output), { recursive: true });

const child = spawn("go", ["build", "-o", output, "."], {
  cwd: path.join(root, "server"),
  stdio: "inherit",
  shell: false,
});

const exitCode = await new Promise((resolve, reject) => {
  child.once("error", reject);
  child.once("exit", resolve);
});
if (exitCode !== 0) {
  process.exit(typeof exitCode === "number" ? exitCode : 1);
}
