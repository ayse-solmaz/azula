$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path (Join-Path $root "web\package.json"))) {
  $root = Resolve-Path (Join-Path $PSScriptRoot "..")
}
Set-Location $root
Write-Host "building web for electron (relative asset base)"
Set-Location (Join-Path $root "web")
if (-not (Test-Path "node_modules")) { npm install }
$env:ELECTRON = "1"
npm run build
Set-Location $root
$dest = Join-Path $root "electron\web"
if (Test-Path $dest) { Remove-Item $dest -Recurse -Force }
Copy-Item (Join-Path $root "web\dist") $dest -Recurse
Write-Host "packing windows installer (needs API on localhost:8080 at runtime)"
Write-Host "macOS DMG is not built here — use scripts/pack-electron.sh on a Mac or GitHub Actions workflow desktop"
Set-Location (Join-Path $root "electron")
if (-not (Test-Path "node_modules")) { npm install }
npm run pack
Write-Host "installer: electron\dist"
