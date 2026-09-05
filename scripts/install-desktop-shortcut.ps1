$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$cmd = Join-Path $root "scripts\azula.cmd"
$desktop = [Environment]::GetFolderPath("Desktop")
$lnkPath = Join-Path $desktop "Azula.lnk"

$shell = New-Object -ComObject WScript.Shell
$lnk = $shell.CreateShortcut($lnkPath)
$lnk.TargetPath = $cmd
$lnk.WorkingDirectory = $root
$lnk.WindowStyle = 7
$lnk.Description = "Azula"
$icon = Join-Path $root "electron\node_modules\electron\dist\electron.exe"
if (Test-Path $icon) { $lnk.IconLocation = "$icon,0" }
$lnk.Save()
Write-Host "Desktop shortcut: $lnkPath"
