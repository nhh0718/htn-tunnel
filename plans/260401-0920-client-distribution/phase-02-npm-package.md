# Phase 2: npm Package Distribution

## Priority: HIGH | Status: Not started | Effort: 6-8h

## Overview
Wrap Go binary in npm package using esbuild pattern (optionalDependencies + JS shim). Users install with `npm i -g htn-tunnel`.

## Architecture

```
npm registry:
├── htn-tunnel (main package)
│   ├── bin/htn-tunnel.js (shim → detects platform → exec binary)
│   ├── lib/install.js (postinstall fallback → download from GitHub)
│   └── package.json (optionalDependencies → platform packages)
│
├── @htn-tunnel/darwin-arm64
│   └── bin/htn-tunnel (macOS ARM64 binary)
├── @htn-tunnel/darwin-x64
│   └── bin/htn-tunnel (macOS Intel binary)
├── @htn-tunnel/linux-arm64
│   └── bin/htn-tunnel (Linux ARM64 binary)
├── @htn-tunnel/linux-x64
│   └── bin/htn-tunnel (Linux x64 binary)
├── @htn-tunnel/win32-arm64
│   └── bin/htn-tunnel.exe (Windows ARM64 binary)
└── @htn-tunnel/win32-x64
    └── bin/htn-tunnel.exe (Windows x64 binary)
```

## Directory Structure (in repo)

```
npm/
├── htn-tunnel/                    # Main package
│   ├── package.json
│   ├── bin/
│   │   └── htn-tunnel.js         # Platform detection shim
│   └── lib/
│       └── install.js            # postinstall fallback
│
└── platform-template/             # Template for platform packages
    └── package.json.tmpl
```

## Implementation Steps

### 1. Create npm org scope

Register `@htn-tunnel` org on npmjs.com (free for public packages).

### 2. Create main package

**npm/htn-tunnel/package.json:**
```json
{
  "name": "htn-tunnel",
  "version": "0.1.0",
  "description": "Self-hosted tunnel client - expose localhost to the internet",
  "bin": {
    "htn-tunnel": "./bin/htn-tunnel.js"
  },
  "optionalDependencies": {
    "@htn-tunnel/darwin-arm64": "0.1.0",
    "@htn-tunnel/darwin-x64": "0.1.0",
    "@htn-tunnel/linux-arm64": "0.1.0",
    "@htn-tunnel/linux-x64": "0.1.0",
    "@htn-tunnel/win32-arm64": "0.1.0",
    "@htn-tunnel/win32-x64": "0.1.0"
  },
  "scripts": {
    "postinstall": "node lib/install.js"
  },
  "keywords": ["tunnel", "ngrok", "localhost", "expose", "dev"],
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/htn-sys/htn-tunnel"
  }
}
```

**npm/htn-tunnel/bin/htn-tunnel.js:**
```javascript
#!/usr/bin/env node
const { execFileSync } = require("child_process");
const path = require("path");
const os = require("os");

const PLATFORMS = {
  "darwin:arm64": "@htn-tunnel/darwin-arm64",
  "darwin:x64": "@htn-tunnel/darwin-x64",
  "linux:arm64": "@htn-tunnel/linux-arm64",
  "linux:x64": "@htn-tunnel/linux-x64",
  "win32:arm64": "@htn-tunnel/win32-arm64",
  "win32:x64": "@htn-tunnel/win32-x64",
};

const key = `${os.platform()}:${os.arch()}`;
const pkg = PLATFORMS[key];
if (!pkg) {
  console.error(`Unsupported platform: ${key}`);
  process.exit(1);
}

const ext = os.platform() === "win32" ? ".exe" : "";
const binPath = path.join(
  path.dirname(require.resolve(`${pkg}/package.json`)),
  "bin",
  `htn-tunnel${ext}`
);

try {
  execFileSync(binPath, process.argv.slice(2), { stdio: "inherit" });
} catch (e) {
  process.exit(e.status || 1);
}
```

**npm/htn-tunnel/lib/install.js:**
```javascript
// Fallback: download binary from GitHub Releases if optionalDeps failed
const fs = require("fs");
const path = require("path");
const os = require("os");
const https = require("https");

const pkg = require("../package.json");
const version = pkg.version;
const platform = os.platform();
const arch = os.arch();
const ext = platform === "win32" ? ".exe" : "";
const binDir = path.join(__dirname, "../bin");
const binPath = path.join(binDir, `htn-tunnel${ext}`);

// Check if binary already exists (from optionalDeps)
const key = `${platform}:${arch}`;
const pkgName = {
  "darwin:arm64": "@htn-tunnel/darwin-arm64",
  "darwin:x64": "@htn-tunnel/darwin-x64",
  "linux:arm64": "@htn-tunnel/linux-arm64",
  "linux:x64": "@htn-tunnel/linux-x64",
  "win32:arm64": "@htn-tunnel/win32-arm64",
  "win32:x64": "@htn-tunnel/win32-x64",
}[key];

try {
  require.resolve(`${pkgName}/package.json`);
  // Platform package installed via optionalDeps — nothing to do
  process.exit(0);
} catch {
  // Not installed — download from GitHub
}

const goArch = arch === "x64" ? "amd64" : arch;
const goOS = platform === "win32" ? "windows" : platform;
const url = `https://github.com/htn-sys/htn-tunnel/releases/download/v${version}/htn-tunnel_${version}_${goOS}_${goArch}${ext}`;

console.log(`Downloading htn-tunnel v${version} for ${platform}/${arch}...`);

function download(url, dest) {
  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        return download(res.headers.location, dest).then(resolve, reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`Download failed: ${res.statusCode}`));
      }
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      file.on("finish", () => { file.close(); resolve(); });
    }).on("error", reject);
  });
}

fs.mkdirSync(binDir, { recursive: true });
download(url, binPath)
  .then(() => {
    fs.chmodSync(binPath, 0o755);
    console.log("Done.");
  })
  .catch((err) => {
    console.error("Failed to download htn-tunnel:", err.message);
    console.error("Install manually: https://github.com/htn-sys/htn-tunnel/releases");
    process.exit(1);
  });
```

### 3. Platform package template

**npm/platform-template/package.json.tmpl:**
```json
{
  "name": "@htn-tunnel/{{PLATFORM}}",
  "version": "{{VERSION}}",
  "description": "htn-tunnel binary for {{PLATFORM}}",
  "os": ["{{OS}}"],
  "cpu": ["{{CPU}}"],
  "bin": {
    "htn-tunnel": "./bin/htn-tunnel{{EXT}}"
  },
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/htn-sys/htn-tunnel"
  }
}
```

Platform mapping:

| npm package | os | cpu | Go build |
|---|---|---|---|
| @htn-tunnel/darwin-arm64 | darwin | arm64 | GOOS=darwin GOARCH=arm64 |
| @htn-tunnel/darwin-x64 | darwin | x64 | GOOS=darwin GOARCH=amd64 |
| @htn-tunnel/linux-arm64 | linux | arm64 | GOOS=linux GOARCH=arm64 |
| @htn-tunnel/linux-x64 | linux | x64 | GOOS=linux GOARCH=amd64 |
| @htn-tunnel/win32-arm64 | win32 | arm64 | GOOS=windows GOARCH=arm64 |
| @htn-tunnel/win32-x64 | win32 | x64 | GOOS=windows GOARCH=amd64 |

### 4. Build + publish script

**scripts/publish-npm.sh:**
```bash
#!/bin/bash
set -e
VERSION=$1
if [ -z "$VERSION" ]; then echo "Usage: ./scripts/publish-npm.sh 0.1.0"; exit 1; fi

PLATFORMS=("darwin:arm64:amd64" "darwin:x64:amd64" "darwin:arm64:arm64"
           "linux:x64:amd64" "linux:arm64:arm64"
           "win32:x64:amd64" "win32:arm64:arm64")

# Correct platform mapping
declare -A PLATFORM_MAP=(
  ["darwin-arm64"]="darwin:arm64"
  ["darwin-x64"]="darwin:amd64"
  ["linux-arm64"]="linux:arm64"
  ["linux-x64"]="linux:amd64"
  ["win32-arm64"]="windows:arm64"
  ["win32-x64"]="windows:amd64"
)

for PLAT in darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-arm64 win32-x64; do
  IFS=':' read -r GOOS GOARCH <<< "${PLATFORM_MAP[$PLAT]}"
  EXT=""; [ "$GOOS" = "windows" ] && EXT=".exe"

  PKG_DIR="dist/npm/@htn-tunnel/$PLAT"
  mkdir -p "$PKG_DIR/bin"

  # Build binary
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
    go build -ldflags "-s -w -X main.version=$VERSION" \
    -o "$PKG_DIR/bin/htn-tunnel$EXT" ./cmd/client

  # Create package.json
  IFS='-' read -r OS CPU <<< "$PLAT"
  cat > "$PKG_DIR/package.json" << EOF
{
  "name": "@htn-tunnel/$PLAT",
  "version": "$VERSION",
  "description": "htn-tunnel binary for $PLAT",
  "os": ["$OS"],
  "cpu": ["$CPU"],
  "license": "MIT"
}
EOF

  # Publish
  cd "$PKG_DIR" && npm publish --access public && cd -
done

# Publish main package
cd npm/htn-tunnel
sed -i "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" package.json
sed -i "s/\"@htn-tunnel\/\([^\"]*\)\": \"[^\"]*\"/\"@htn-tunnel\/\1\": \"$VERSION\"/g" package.json
npm publish --access public
```

### 5. GitHub Actions workflow

Add to `.github/workflows/release.yml`:
```yaml
  publish-npm:
    needs: release
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          registry-url: https://registry.npmjs.org
      - run: chmod +x scripts/publish-npm.sh && ./scripts/publish-npm.sh ${GITHUB_REF_NAME#v}
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

## Success Criteria
- [ ] `npm i -g htn-tunnel` works on macOS, Linux, Windows
- [ ] `npx htn-tunnel --version` works without prior install
- [ ] postinstall fallback works when optionalDeps disabled
- [ ] `htn-tunnel http 3000` connects to server correctly

## Files to Create
- `npm/htn-tunnel/package.json`
- `npm/htn-tunnel/bin/htn-tunnel.js`
- `npm/htn-tunnel/lib/install.js`
- `scripts/publish-npm.sh`

## Prerequisites
- Phase 1 complete (GitHub Releases for postinstall fallback)
- npm org `@htn-tunnel` created
- `NPM_TOKEN` secret in GitHub repo
