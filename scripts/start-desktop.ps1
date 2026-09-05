# One-command desktop: ensure electron/web exists, start the API if needed, open the shell.
# Prefer scripts\azula.cmd from the repo root. Do not start Vite — this packs web/dist into electron/web.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Show-Fail([string]$Msg) {
  Add-Type -AssemblyName System.Windows.Forms
  [System.Windows.Forms.MessageBox]::Show($Msg, "Azula", "OK", "Error") | Out-Null
  exit 1
}

function Invoke-Npm([string]$WorkDir, [string]$NpmArgs) {
  $npm = Join-Path $env:ProgramFiles "nodejs\npm.cmd"
  if (-not (Test-Path $npm)) { $npm = "npm.cmd" }
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

if (-not (Test-Path $bundled)) {
  if (-not (Test-Path (Join-Path $webDir "node_modules"))) {
    Invoke-Npm $webDir "install"
  }
  $env:ELECTRON = "1"
  Invoke-Npm $webDir "run build"
  $dest = Join-Path $electronDir "web"
  if (Test-Path $dest) { Remove-Item $dest -Recurse -Force }
  Copy-Item (Join-Path $webDir "dist") $dest -Recurse
}
if (-not (Test-Path $bundled)) {
  Show-Fail "Arayüz yok: electron\web\index.html"
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
