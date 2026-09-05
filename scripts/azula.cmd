@echo off
setlocal
cd /d "%~dp0\.."
set ELECTRON_EXE=%CD%\electron\node_modules\electron\dist\electron.exe
set APP_DIR=%CD%\electron

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "try { $c = New-Object System.Net.Sockets.TcpClient('127.0.0.1',8080); $c.Close() } catch { if (Get-Command go -ErrorAction SilentlyContinue) { Start-Process go -ArgumentList @('run','./cmd/api') -WorkingDirectory '%CD%' -WindowStyle Minimized } }"

if not exist "%ELECTRON_EXE%" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0\start-desktop.ps1"
  exit /b %ERRORLEVEL%
)
if not exist "%APP_DIR%\web\index.html" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0\start-desktop.ps1"
  exit /b %ERRORLEVEL%
)
start "" "%ELECTRON_EXE%" "%APP_DIR%"
exit /b 0
