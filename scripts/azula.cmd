@echo off
setlocal
cd /d "%~dp0\.."
rem One command: build electron/web if needed, start the API, then open the desktop shell.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0\start-desktop.ps1"
exit /b %ERRORLEVEL%
