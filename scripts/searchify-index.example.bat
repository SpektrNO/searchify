@echo off
REM Convenience wrapper: load env then index SEARCHIFY_ROOTS (keyword-only).
REM Place next to searchify.exe (e.g. C:\Apps) together with searchify-env.bat
REM copied from scripts\searchify-env.example.bat
REM
REM After a successful keyword index, backfill vectors in the same CMD window:
REM   call searchify-env.bat
REM   set "SEARCHIFY_SKIP_EMBED=0"
REM   searchify.exe embed --force
REM
REM After changing SEARCHIFY_CHUNK_* or SEARCHIFY_EMBED_MODEL, re-index/re-embed:
REM   searchify.exe index --force --skip-embed "%SEARCHIFY_ROOTS%"
REM   searchify.exe embed --force

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
echo Indexing "%SEARCHIFY_ROOTS%" ...
if "%~1"=="" (
  searchify.exe index --skip-embed "%SEARCHIFY_ROOTS%"
) else (
  searchify.exe index --skip-embed %*
)
if errorlevel 1 exit /b 1

echo.
echo Done. For vectors: set SEARCHIFY_SKIP_EMBED=0 then run searchify.exe embed --force
