#!/bin/bash
# Builds Go binaries for all platforms and publishes npm packages.
# Usage: ./scripts/publish-npm.sh 0.1.0
set -euo pipefail

VERSION="${1:?Usage: $0 <version>}"
REPO="nhh0718/htn-tunnel"

declare -A PLATFORMS=(
  ["darwin-arm64"]="darwin:arm64"
  ["darwin-x64"]="darwin:amd64"
  ["linux-arm64"]="linux:arm64"
  ["linux-x64"]="linux:amd64"
  ["win32-arm64"]="windows:arm64"
  ["win32-x64"]="windows:amd64"
)

echo "=== Building and publishing htn-tunnel v${VERSION} ==="

# 1. Build + publish platform packages
for PLAT in "${!PLATFORMS[@]}"; do
  IFS=':' read -r GOOS GOARCH <<< "${PLATFORMS[$PLAT]}"
  EXT=""; [[ "$GOOS" == "windows" ]] && EXT=".exe"
  IFS='-' read -r OS CPU <<< "$PLAT"

  echo "--- Building @htn-tunnel/${PLAT} (${GOOS}/${GOARCH}) ---"

  PKG_DIR="dist/npm/@htn-tunnel/${PLAT}"
  rm -rf "$PKG_DIR"
  mkdir -p "$PKG_DIR/bin"

  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -ldflags "-s -w -X main.version=${VERSION}" \
    -o "$PKG_DIR/bin/htn-tunnel${EXT}" ./cmd/htn-tunnel

  cat > "$PKG_DIR/package.json" << EOF
{
  "name": "@htn-tunnel/${PLAT}",
  "version": "${VERSION}",
  "description": "htn-tunnel binary for ${PLAT}",
  "os": ["${OS}"],
  "cpu": ["${CPU}"],
  "bin": { "htn-tunnel": "./bin/htn-tunnel${EXT}" },
  "license": "MIT",
  "repository": { "type": "git", "url": "https://github.com/${REPO}" }
}
EOF

  (cd "$PKG_DIR" && npm publish --access public)
  echo "--- Published @htn-tunnel/${PLAT}@${VERSION} ---"
done

# 2. Update + publish main package
echo "--- Publishing htn-tunnel@${VERSION} ---"
MAIN_DIR="npm/htn-tunnel"

# Update version in package.json
node -e "
const fs = require('fs');
const pkg = JSON.parse(fs.readFileSync('${MAIN_DIR}/package.json', 'utf8'));
pkg.version = '${VERSION}';
for (const dep of Object.keys(pkg.optionalDependencies || {})) {
  pkg.optionalDependencies[dep] = '${VERSION}';
}
fs.writeFileSync('${MAIN_DIR}/package.json', JSON.stringify(pkg, null, 2) + '\n');
"

(cd "$MAIN_DIR" && npm publish --access public)
echo "=== Done! htn-tunnel@${VERSION} published ==="
