// Postinstall: download htn-tunnel binary from GitHub Releases.
const fs = require("fs");
const path = require("path");
const os = require("os");
const https = require("https");
const { execSync } = require("child_process");

// Binary version from GitHub Releases.
const VERSION = "0.2.0";
const REPO = "nhh0718/htn-tunnel";

const platform = os.platform();
const arch = os.arch();
const goOS = platform === "win32" ? "windows" : platform;
const goArch = arch === "x64" ? "amd64" : arch;
const ext = platform === "win32" ? ".exe" : "";
const archiveExt = platform === "win32" ? ".zip" : ".tar.gz";

const binDir = path.join(__dirname, "..", "bin");
const binPath = path.join(binDir, `htn-tunnel${ext}`);

// Skip if binary exists and works.
if (fs.existsSync(binPath) && fs.statSync(binPath).size > 1000) {
  console.log("htn-tunnel: binary already installed.");
  process.exit(0);
}

const archiveName = `htn-tunnel_${VERSION}_${goOS}_${goArch}${archiveExt}`;
const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`;

console.log(`htn-tunnel: downloading v${VERSION} for ${platform}/${arch}...`);

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "htn-"));
const tmpFile = path.join(tmpDir, archiveName);

function follow(url, dest, redirects) {
  return new Promise((resolve, reject) => {
    if (redirects > 5) return reject(new Error("too many redirects"));
    https.get(url, { headers: { "User-Agent": "htn-tunnel-npm" } }, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        return follow(res.headers.location, dest, redirects + 1).then(resolve, reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`HTTP ${res.statusCode}`));
      }
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      file.on("finish", () => { file.close(); resolve(); });
      file.on("error", reject);
    }).on("error", reject);
  });
}

async function main() {
  try {
    await follow(url, tmpFile, 0);
    fs.mkdirSync(binDir, { recursive: true });

    if (archiveExt === ".zip") {
      // Use PowerShell on Windows for reliable zip extraction.
      execSync(
        `powershell -NoProfile -Command "Expand-Archive -Path '${tmpFile}' -DestinationPath '${tmpDir}' -Force"`,
        { stdio: "pipe" }
      );
    } else {
      execSync(`tar -xzf "${tmpFile}" -C "${tmpDir}"`, { stdio: "pipe" });
    }

    // Find the binary in extracted files.
    const binaryName = `htn-tunnel${ext}`;
    const extracted = findFile(tmpDir, binaryName);
    if (!extracted) throw new Error(`${binaryName} not found in archive`);

    fs.copyFileSync(extracted, binPath);
    fs.chmodSync(binPath, 0o755);
    console.log("htn-tunnel: installed successfully!");
  } catch (err) {
    console.error("htn-tunnel: install failed:", err.message);
    console.error(`\nInstall manually: https://github.com/${REPO}/releases/tag/v${VERSION}`);
    console.error(`Or: go install github.com/${REPO}/cmd/htn-tunnel@latest\n`);
    process.exit(1);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

function findFile(dir, name) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      const found = findFile(full, name);
      if (found) return found;
    } else if (entry.name === name) {
      return full;
    }
  }
  return null;
}

main();
