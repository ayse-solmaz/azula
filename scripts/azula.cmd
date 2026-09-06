@echo off
setlocal
cd /d "%~dp0\.."
rem Desktop: API :8080 + live Vite :3001 (preferred) or a fresh electron/web bundle.
rem   scripts\azula.cmd          — start Vite if packing would be slow, else bundle
rem   scripts\azula.cmd dev      — azula-dev path: ELECTRON_WEB_URL=http://127.0.0.1:3001
rem   scripts\azula-dev.cmd      — same as azula.cmd dev
if /i "%~1"=="dev" goto :dev
if /i "%~1"=="-dev" goto :dev
if /i "%~1"=="--dev" goto :dev
if /i "%~1"=="azula-dev" goto :dev
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0\start-desktop.ps1"
exit /b %ERRORLEVEL%

:dev
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0\start-desktop.ps1" -Dev
exit /b %ERRORLEVEL%
