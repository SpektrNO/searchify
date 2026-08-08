@echo off
REM Searchify Windows environment — copy to searchify-env.bat and edit paths.
REM Usage (same CMD window):
REM   call searchify-env.bat
REM   searchify.exe index --skip-embed "%SEARCHIFY_ROOTS%"
REM
REM Important: use set "NAME=value" (quotes around NAME=value). Do NOT put
REM quotes inside the value itself (e.g. set FOO="C:\path" is wrong).

REM --- required ---
set "SEARCHIFY_ROOTS=C:\Users\frank\OneDrive\Personal Vault\bedrift"

REM --- index storage (use a dedicated folder; avoid huge shared DBs while testing) ---
set "SEARCHIFY_INDEX_DIR=C:\temp\searchify-fts"

REM --- recommended for large PDF corpora ---
set "SEARCHIFY_EXTRACT_TIMEOUT=20s"
set "SEARCHIFY_SKIP_EMBED=1"
REM set "SEARCHIFY_EMBED_BACKEND=none"
REM set "SEARCHIFY_TEXT_ONLY=1"

REM --- optional: Poppler (pdftotext) if not already on system PATH ---
REM set "PATH=C:\Apps\poppler\Library\bin;%PATH%"

REM --- optional: HTTP serve ---
REM set "SEARCHIFY_HTTP_TOKEN=dev-secret"
REM set "SEARCHIFY_HTTP_ADDR=127.0.0.1:8080"

REM --- optional: LangSearch (search_web + local search rerank=true) ---
REM set "LANGSEARCH_API_KEY=sk-your-key-here"

REM --- optional: relative path base / watch ---
REM set "SEARCHIFY_PATH_BASE=%SEARCHIFY_ROOTS%"
REM set "SEARCHIFY_WATCH_PATHS=%SEARCHIFY_ROOTS%"

echo SEARCHIFY_ROOTS=%SEARCHIFY_ROOTS%
echo SEARCHIFY_INDEX_DIR=%SEARCHIFY_INDEX_DIR%
echo SEARCHIFY_SKIP_EMBED=%SEARCHIFY_SKIP_EMBED%
echo SEARCHIFY_EXTRACT_TIMEOUT=%SEARCHIFY_EXTRACT_TIMEOUT%
if defined LANGSEARCH_API_KEY (echo LANGSEARCH_API_KEY: set) else (echo LANGSEARCH_API_KEY: not set)
where pdftotext >nul 2>&1 && (echo pdftotext: OK) || (echo pdftotext: NOT on PATH — install Poppler for safer PDF extract)
