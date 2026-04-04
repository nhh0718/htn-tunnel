#!/usr/bin/env node
// Shim: finds and executes the htn-tunnel binary downloaded by postinstall.
const path = require("path");
const os = require("os");
const fs = require("fs");

const ext = os.platform() === "win32" ? ".exe" : "";
const binPath = path.join(__dirname, `htn-tunnel${ext}`);

if (!fs.existsSync(binPath)) {
  console.error("htn-tunnel: binary not found. Try reinstalling:");
  console.error("  npm install -g htn-tunnel");
  process.exit(1);
}

const result = require("child_process").spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: false,
});
process.exit(result.status ?? 1);
