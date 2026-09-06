# One-command desktop: start the API if needed, then open the Electron shell.
# Prefer a live Vite UI (http://127.0.0.1:3001) over a stale electron/web bundle.
# Packing web/dist is the fallback when Vite is not running.
# Dev path (skip pack, start Vite):  scripts\azula.cmd dev  or  scripts\azula-dev.cmd
#   powershell -File scripts/start-desktop.ps1 -Dev
param(
  [switch]$Dev
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Show-Fail([string]$Msg) {
  Add-Type -AssemblyName System.Windows.Forms
  [System.Windows.Forms.MessageBox]::Show($Msg, "Azula", "OK", "Error") | Out-Null
  exit 1
}

function Get-Npm {
  $npm = Join-Path $env:ProgramFiles "nodejs\npm.cmd"
  if (-not (Test-Path $npm)) { $npm = "npm.cmd" }
  return $npm
}

function Invoke-Npm([string]$WorkDir, [string]$NpmArgs) {
  $npm = Get-Npm
  $p = Start-Process -FilePath "cmd.exe" -ArgumentList @("/c", "`"$npm`" $NpmArgs") -WorkingDirectory $WorkDir -Wait -PassThru -WindowStyle Normal
  if ($p.ExitCode -ne 0) {
    Show-Fail "npm $NpmArgs başarısız (kod $($p.ExitCode))"
  }
}

function Test-Port([int]$Port) {
  try {
    $c = New-Object System.Net.Sockets.TcpClient("127.0.0.1", $Port)
    $c.Close()
    return $true
  } catch {
    return $false
  }
}

function Start-ViteIfNeeded([int]$Seconds = 40) {
  if (Test-Port 3001) { return $true }
  if (-not (Test-Path (Join-Path $webDir "package.json"))) { return $false }
  if (-not (Test-Path (Join-Path $webDir "node_modules"))) {
    Invoke-Npm $webDir "install"
  }
  $npm = Get-Npm
  Start-Process -FilePath "cmd.exe" -ArgumentList @("/c", "`"$npm`" run dev") -WorkingDirectory $webDir -WindowStyle Minimized
  for ($i = 0; $i -lt $Seconds; $i++) {
    if (Test-Port 3001) { return $true }
    Start-Sleep -Seconds 1
  }
  return $false
}

function Ensure-Bundle {
  if (Test-Path $bundled) { return $true }
  if (-not (Test-Path (Join-Path $webDir "node_modules"))) {
    Invoke-Npm $webDir "install"
  }
  $env:ELECTRON = "1"
  Invoke-Npm $webDir "run build"
  $dest = Join-Path $electronDir "web"
  if (Test-Path $dest) { Remove-Item $dest -Recurse -Force }
  Copy-Item (Join-Path $webDir "dist") $dest -Recurse
  return (Test-Path $bundled)
}

$electronDir = Join-Path $root "electron"
$webDir = Join-Path $root "web"
$electronExe = Join-Path $electronDir "node_modules\electron\dist\electron.exe"
$bundled = Join-Path $electronDir "web\index.html"

if (-not (Test-Path $electronExe)) {
  if (-not (Test-Path (Join-Path $electronDir "package.json"))) {
    Show-Fail "electron klasörü yok."
  }
  Invoke-Npm $electronDir "install"
}
if (-not (Test-Path $electronExe)) {
  Show-Fail "Electron yüklü değil. electron klasöründe npm install çalıştır."
}

# Browser = :3001. Desktop needs API :8080 + either live Vite or a fresh bundle.
# Prefer Vite when it is already up. If there is no bundle, start Vite instead of a slow pack.
if ($Dev) {
  if (-not (Start-ViteIfNeeded)) {
    Show-Fail "Vite :3001 açılmadı. web klasöründe npm run dev çalıştırın."
  }
  $env:ELECTRON_WEB_URL = "http://127.0.0.1:3001"
} elseif (Test-Port 3001) {
  $env:ELECTRON_WEB_URL = "http://127.0.0.1:3001"
} elseif (-not (Test-Path $bundled)) {
  if (Start-ViteIfNeeded) {
    $env:ELECTRON_WEB_URL = "http://127.0.0.1:3001"
  } elseif (-not (Ensure-Bundle)) {
    Show-Fail "Arayüz yok: Vite (:3001) veya electron\web\index.html"
  }
}

function Wait-Api([int]$Seconds = 25) {
  for ($i = 0; $i -lt $Seconds; $i++) {
    try {
      $r = Invoke-WebRequest -Uri "http://127.0.0.1:8080/health" -UseBasicParsing -TimeoutSec 2
      if ($r.StatusCode -eq 200) { return $true }
    } catch {
      Start-Sleep -Seconds 1
    }
  }
  return $false
}

if (-not (Test-Port 8080)) {
  $go = (Get-Command go -ErrorAction SilentlyContinue).Source
  if ($go) {
    Start-Process -FilePath $go -ArgumentList @("run", "./cmd/api") -WorkingDirectory $root -WindowStyle Minimized
  }
}

if (-not (Wait-Api)) {
  Add-Type -AssemblyName System.Windows.Forms
  [System.Windows.Forms.MessageBox]::Show(
    "API henüz ayakta değil (http://127.0.0.1:8080/health). MongoDB açık mı? Masaüstü yine açılacak; giriş çalışmazsa go run ./cmd/api çalıştırın.`n`nThe API is not up yet. The desktop window will still open; sign-in needs go run ./cmd/api and MongoDB.",
    "Azula",
    "OK",
    "Warning"
  ) | Out-Null
}

Start-Process -FilePath $electronExe -ArgumentList $electronDir -WorkingDirectory $electronDir
