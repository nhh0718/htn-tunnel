// Postinstall fallback — downloads binary from GitHub Releases
// when optionalDependencies are disabled or unavailable.

const fs = require("fs");
const path = require("path");
const os = require("os");
const https = require("https");

const PLATFORMS = {
  "darwin:arm64": "@htn-tunnel/darwin-arm64",
  "darwin:x64": "@htn-tunnel/darwin-x64",
  "linux:arm64": "@htn-tunnel/linux-arm64",
  "linux:x64": "@htn-tunnel/linux-x64",
  "win32:arm64": "@htn-tunnel/win32-arm64",
  "win32:x64": "@htn-tunnel/win32-x64",
};

const platform = os.platform();
const arch = os.arch();
const key = `${platform}:${arch}`;
const pkg = PLATFORMS[key];

// Check if platform package already installed via optionalDependencies
if (pkg) {
  try {
    require.resolve(`${pkg}/package.json`);
    process.exit(0); // Already installed, nothing to do
  } catch {
    // Not installed — fall through to download
  }
}

const version = require("../package.json").version;
const goArch = arch === "x64" ? "amd64" : arch;
const goOS = platform === "win32" ? "windows" : platform;
const ext = platform === "win32" ? ".exe" : "";
const binDir = path.join(__dirname, "..", "bin");
const binPath = path.join(binDir, `htn-tunnel${ext}`);

// Already downloaded
if (fs.existsSync(binPath) && fs.statSync(binPath).size > 0) {
  process.exit(0);
}

const baseUrl = `https://github.com/nhh0718/htn-tunnel/releases/download/v${version}`;
const archiveName = `htn-tunnel_${version}_${goOS}_${goArch}`;
const archiveExt = platform === "win32" ? ".zip" : ".tar.gz";
const url = `${baseUrl}/${archiveName}${archiveExt}`;

console.log(`htn-tunnel: downloading v${version} for ${platform}/${arch}...`);

function follow(url, cb) {
  https.get(url, (res) => {
    if (res.statusCode === 301 || res.statusCode === 302) {
      return follow(res.headers.location, cb);
    }
    cb(res);
  }).on("error", (err) => {
    console.error("htn-tunnel: download failed:", err.message);
    console.error("Install manually: https://github.com/nhh0718/htn-tunnel/releases");
    process.exit(1);
  });
}

fs.mkdirSync(binDir, { recursive: true });

follow(url, (res) => {
  if (res.statusCode !== 200) {
    console.error(`htn-tunnel: download failed (HTTP ${res.statusCode})`);
    console.error("Install manually: https://github.com/nhh0718/htn-tunnel/releases");
    process.exit(1);
  }

  const chunks = [];
  res.on("data", (chunk) => chunks.push(chunk));
  res.on("end", () => {
    const data = Buffer.concat(chunks);

    if (archiveExt === ".zip") {
      // Simple zip extraction — find the binary entry
      extractZipBinary(data, `htn-tunnel${ext}`, binPath);
    } else {
      // tar.gz extraction
      extractTarGzBinary(data, `htn-tunnel${ext}`, binPath);
    }

    fs.chmodSync(binPath, 0o755);
    console.log("htn-tunnel: installed successfully");
  });
});

function extractTarGzBinary(gzData, filename, dest) {
  const zlib = require("zlib");
  const tarData = zlib.gunzipSync(gzData);

  // Simple tar parser — find file entry matching filename
  let offset = 0;
  while (offset < tarData.length) {
    const header = tarData.subarray(offset, offset + 512);
    if (header[0] === 0) break;

    const name = header.toString("utf8", 0, 100).replace(/\0/g, "");
    const sizeStr = header.toString("utf8", 124, 136).replace(/\0/g, "").trim();
    const size = parseInt(sizeStr, 8) || 0;
    offset += 512;

    if (name.endsWith(filename)) {
      fs.writeFileSync(dest, tarData.subarray(offset, offset + size));
      return;
    }
    offset += Math.ceil(size / 512) * 512;
  }
  throw new Error(`${filename} not found in archive`);
}

function extractZipBinary(zipData, filename, dest) {
  // Simple zip parser — find local file header for the binary
  let offset = 0;
  while (offset < zipData.length - 4) {
    if (zipData.readUInt32LE(offset) !== 0x04034b50) break; // PK\x03\x04

    const nameLen = zipData.readUInt16LE(offset + 26);
    const extraLen = zipData.readUInt16LE(offset + 28);
    const compSize = zipData.readUInt32LE(offset + 18);
    const name = zipData.toString("utf8", offset + 30, offset + 30 + nameLen);

    const dataStart = offset + 30 + nameLen + extraLen;
    if (name.endsWith(filename)) {
      fs.writeFileSync(dest, zipData.subarray(dataStart, dataStart + compSize));
      return;
    }
    offset = dataStart + compSize;
  }
  throw new Error(`${filename} not found in archive`);
}
