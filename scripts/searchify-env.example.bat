@echo off
REM Searchify Windows environment — copy to searchify-env.bat and edit paths.
REM Usage (same CMD window):
REM   call searchify-env.bat
REM   searchify.exe index --skip-embed "%SEARCHIFY_ROOTS%"
REM   searchify.exe embed --force
REM
REM Important: use set "NAME=value" (quotes around NAME=value). Do NOT put
REM quotes inside the value itself (e.g. set FOO="C:\path" is wrong).

REM --- required ---
set "SEARCHIFY_ROOTS=C:\Users\frank\OneDrive\Personal Vault\bedrift"

REM --- index storage (use a dedicated folder; avoid huge shared DBs while testing) ---
set "SEARCHIFY_INDEX_DIR=C:\temp\searchify-fts"

REM --- extract / memory (recommended for large PDF corpora) ---
set "SEARCHIFY_EXTRACT_TIMEOUT=20s"
set "SEARCHIFY_SKIP_EMBED=1"
REM set "SEARCHIFY_EMBED_BACKEND=none"
REM set "SEARCHIFY_TEXT_ONLY=1"
REM set "SEARCHIFY_EXTRACT_INPROCESS=1"
REM set "SEARCHIFY_MAX_FILE_BYTES=2097152"
REM set "SEARCHIFY_MAX_EXTRACT_BYTES=524288"
REM set "SEARCHIFY_MAX_CHUNKS_PER_FILE=64"

REM --- chunking (structure-aware; after change: index --force then embed --force) ---
REM set "SEARCHIFY_CHUNK_BYTES=3072"
REM set "SEARCHIFY_CHUNK_OVERLAP=256"

REM --- embeddings (unset SKIP_EMBED / use embed CLI when ready for vectors) ---
REM set "SEARCHIFY_SKIP_EMBED=0"
REM set "SEARCHIFY_EMBED_BACKEND=process"
REM set "SEARCHIFY_EMBED_MODEL=minilm-l6-v2"
REM set "SEARCHIFY_EMBED_MODEL=mpnet-base-v2"
REM set "SEARCHIFY_EMBED_BATCH=1"
REM set "SEARCHIFY_EMBED_RELOAD=1"

REM --- optional: Poppler (pdftotext) if not already on system PATH ---
REM set "PATH=C:\Apps\poppler\Library\bin;%PATH%"

REM --- optional: OCR (needs tesseract; PDF OCR also needs pdftoppm) ---
REM set "SEARCHIFY_OCR=1"
REM set "SEARCHIFY_OCR_LANG=eng"

REM --- optional: HTTP serve ---
REM set "SEARCHIFY_HTTP_TOKEN=dev-secret"
REM set "SEARCHIFY_HTTP_ADDR=127.0.0.1:8080"

REM --- optional: LangSearch (search_web + local search rerank=true) ---
REM set "LANGSEARCH_API_KEY=sk-your-key-here"

REM --- optional: relative path base / watch (OneDrive: consider WATCH_RESCAN) ---
REM set "SEARCHIFY_PATH_BASE=%SEARCHIFY_ROOTS%"
REM set "SEARCHIFY_WATCH_PATHS=%SEARCHIFY_ROOTS%"
REM set "SEARCHIFY_WATCH_DEBOUNCE=1s"
REM set "SEARCHIFY_WATCH_RESCAN=5m"

echo SEARCHIFY_ROOTS=%SEARCHIFY_ROOTS%
echo SEARCHIFY_INDEX_DIR=%SEARCHIFY_INDEX_DIR%
echo SEARCHIFY_SKIP_EMBED=%SEARCHIFY_SKIP_EMBED%
echo SEARCHIFY_EXTRACT_TIMEOUT=%SEARCHIFY_EXTRACT_TIMEOUT%
if defined SEARCHIFY_EMBED_MODEL (echo SEARCHIFY_EMBED_MODEL=%SEARCHIFY_EMBED_MODEL%) else (echo SEARCHIFY_EMBED_MODEL: default minilm-l6-v2)
if defined SEARCHIFY_EMBED_BACKEND (echo SEARCHIFY_EMBED_BACKEND=%SEARCHIFY_EMBED_BACKEND%) else (echo SEARCHIFY_EMBED_BACKEND: default process)
if defined SEARCHIFY_CHUNK_BYTES (echo SEARCHIFY_CHUNK_BYTES=%SEARCHIFY_CHUNK_BYTES%) else (echo SEARCHIFY_CHUNK_BYTES: default 3072)
if defined SEARCHIFY_CHUNK_OVERLAP (echo SEARCHIFY_CHUNK_OVERLAP=%SEARCHIFY_CHUNK_OVERLAP%) else (echo SEARCHIFY_CHUNK_OVERLAP: default 256)
if defined LANGSEARCH_API_KEY (echo LANGSEARCH_API_KEY: set) else (echo LANGSEARCH_API_KEY: not set)
where pdftotext >nul 2>&1 && (echo pdftotext: OK) || (echo pdftotext: NOT on PATH — install Poppler for safer PDF extract)
