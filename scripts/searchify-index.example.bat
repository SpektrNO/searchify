@echo off
REM Convenience wrapper: load env then index SEARCHIFY_ROOTS (keyword-only).
REM Place next to searchify.exe (e.g. C:\Apps) together with searchify-env.bat
REM copied from scripts\searchify-env.example.bat

cd /d "%~dp0"
if not exist "searchify-env.bat" (
  echo Missing searchify-env.bat — copy scripts\searchify-env.example.bat and edit paths.
  exit /b 1
)
if not exist "searchify.exe" (
  echo Missing searchify.exe in %CD%
  exit /b 1
)

call "%~dp0searchify-env.bat"
if errorlevel 1 exit /b 1

echo.
echo Indexing %SEARCHIFY_ROOTS% ...
searchify.exe index --skip-embed %*
if errorlevel 1 exit /b 1
