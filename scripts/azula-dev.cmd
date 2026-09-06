@echo off
rem azula-dev: skip packing, start Vite if needed, load http://127.0.0.1:3001 in Electron.
call "%~dp0azula.cmd" dev
exit /b %ERRORLEVEL%
