#!/usr/bin/env bash
# Build an unsigned Azula .dmg. Must run on macOS (electron-builder cannot emit Darwin from Windows).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/web"
echo "building web for electron (relative asset base)"
if [[ ! -d node_modules ]]; then
  npm install
fi
ELECTRON=1 npm run build
rm -rf "$root/electron/web"
cp -R "$root/web/dist" "$root/electron/web"
cd "$root/electron"
if [[ ! -d node_modules ]]; then
  npm install
fi
export CSC_IDENTITY_AUTO_DISCOVERY=false
npm run pack:mac
echo "artifact: electron/dist (unsigned dmg/zip). API still expected at localhost:8080."
