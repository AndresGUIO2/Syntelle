@echo off
setlocal
set "SYNTELLE_DEV_NODE=%~dp0.tools\node-v24.22.0-win-x64\node.exe"
if not exist "%SYNTELLE_DEV_NODE%" set "SYNTELLE_DEV_NODE=node"
"%SYNTELLE_DEV_NODE%" "%~dp0scripts\development\dev.mjs" %*
exit /b %errorlevel%
