# Handoff: opt-richer-file-types

**Status:** done  
**Created:** 2026-08-07  
**Specifier:** spec complete  
**Developer:** complete

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-richer-file-types` |
| Parent issue | #24 — https://github.com/SpektrNO/searchify/issues/24 |
| Open tasks | _(none)_ |
| Closed | `spec` (#25), `engine` (#26), `verify` (#27), `docs` (#28) |

Task order: `audit` → `spec` → `engine` → `verify` → `docs`

## Intent

Expand local indexing beyond the text/code extension allowlist so OneDrive-style personal/business corpora are searchable: extract plain text from PDF, Word, Excel/CSV, and images (OCR), then chunk/embed/search exactly as today — fail soft, Windows+Linux, Groundline-local friendly.

## Scope for THIS implementation

### In scope (must ship)

**P0 formats**

| Kind | Extensions | Behavior |
|------|------------|----------|
| PDF | `.pdf` | Native text extract; optional OCR fallback for image-only / scanned pages when OCR enabled |
| Word | `.docx` | Plain text (headings/paragraphs/tables where the library exposes them cheaply) |
| Excel | `.xlsx`, `.csv` | Sheet/cell text joined into searchable plain text; CSV treated as UTF-8 text (newline-preserving) |
| Images | `.png` `.jpg` `.jpeg` `.webp` `.tif` `.tiff` `.gif` | OCR → searchable text **only when OCR is enabled**; when OCR off, skip with a clear message (do not count as hard error) |

**Cheap P1 allowlist expands (same change set)**

| Kind | Extensions | Behavior |
|------|------------|----------|
| Extra text | `.xml` `.toml` `.ini` `.log` `.rst` `.adoc` `.markdown` | Passthrough UTF-8 text (same path as `.md`/`.txt`) |
| HTML | `.html` `.htm` | Strip tags → visible text (scripts/styles discarded) |

**Existing passthrough (unchanged semantics)**

`.md` `.txt` `.go` `.ts` `.tsx` `.js` `.json` `.yaml` `.yml` `.sql` `.sh` `.py` `.rs`

### Stretch in this PR (include if pure-Go / no CGO and budgets hold)

| Kind | Extensions | Notes |
|------|------------|--------|
| PowerPoint | `.pptx` | Slide text |
| OpenDocument | `.odt` `.ods` `.odp` | LibreOffice common in EU |
| RTF | `.rtf` | Legacy docs |
| Email | `.eml` | Subject + body text |

If a stretch format needs heavy native deps or unreliable extractors, **defer to a follow-up** and document in Deviations — do not block P0.

### Explicitly out of scope (P2 / non-goals)

- EPUB, zip (recursive), HEIC/BMP, legacy `.doc`/`.xls`/`.ppt`, `.msg`
- Layout-faithful conversion, thumbnails, gallery UI
- Encrypted/DRM files without user-provided unlock
- Binary blobs with no extractable text and no OCR path
- Changing MCP/REST tool **shapes** (names, required fields, result schemas beyond optional status fields)
- Requiring network/cloud OCR services

---

## Technical contract

| Area | Requirement |
|------|-------------|
| MCP tools | **No** new tools. `index_paths`, `remove_paths`, `index_prune`, `search_local`, `search_web` unchanged. Optional: `index_status` may add `ocr_enabled` (bool) and/or `index_extensions` (string list) — additive only |
| CLI | `searchify index` / `remove` / `prune` unchanged flags; richer files appear in existing `indexed=` / `messages` |
| REST | `POST /v1/index` / `POST /v1/search` unchanged request/response shapes |
| Search mode | keyword / vector / hybrid unchanged; they consume extracted plain text chunks |
| `search_file` | For passthrough text: keep line scan of file bytes. For extractor formats: extract → keyword-scan **extracted** lines (line numbers refer to extracted text, not PDF page coords). Fail soft with error if extract fails |
| Performance | See budgets below; indexing must not hang forever on one bad PDF/image |
| Acceptance | See Acceptance criteria |

### Extractor contract (implementable)

Introduce a small pluggable extraction layer (suggested package `internal/extract` or under `internal/local/extract`):

```go
// Conceptual — developer may rename but must preserve behavior.
type Extractor interface {
    // Extensions returns lowercase extensions including the leading dot, e.g. ".pdf".
    Extensions() []string
    // Extract returns UTF-8 plain text suitable for existing chunkFile().
    // warn is non-fatal (partial OCR, truncated pages). err means skip this file.
    Extract(ctx context.Context, path string, r io.Reader, size int64) (text string, warn []string, err error)
}
```

Rules:

1. **Registry by extension** (MIME optional later). First registered wins; tests pin the map.
2. **Passthrough extractor** for existing text/code + cheap P1 text extensions: read UTF-8 (invalid sequences → replace or skip with message; prefer replace so partial files still index).
3. **Format extractors** for PDF/Office/HTML/images — pure Go preferred; **default Searchify build must not require CGO**.
4. **OCR** is a separate capability behind config (see env). Image extractors and PDF scanned-page fallback call OCR only when enabled. When OCR is disabled or unavailable (binary missing), skip OCR paths with a warning — do not fail process start.
5. **Fail soft:** extract error → increment `errors` (or treat as skip with message — prefer `errors++` + message like today for read failures), continue remaining files. Walk/size skip messages stay non-fatal.
6. **Indexing pipeline change:** `IndexPaths` must not always `os.ReadFile` + treat bytes as text. Flow: walk allowlist → open file → `Extract` → `chunkFile([]byte(text))` → embed/store as today. Empty extract (0 runes after trim) → skip with message, do not insert empty chunks.
7. **Incremental keys unchanged:** still `mtime` + `size` of the **source file** (not extracted length). `force` re-extracts.
8. **No schema bump** required (still FTS text + vectors). Schema stays at v2 unless implementation discovers a real need — prefer no migration.

### Size / time budgets

| Knob | Default | Behavior |
|------|---------|----------|
| Max source file size | **32 MiB** for extractor formats; **2 MiB** retained for passthrough text/code (or single unified `SEARCHIFY_MAX_FILE_BYTES` default **32 MiB** applied to all — pick one approach and document; prefer **unified 32 MiB** with message on skip) | Over size → skip + message (same pattern as today’s “larger than 2MB”) |
| Per-file extract timeout | **30s** (`SEARCHIFY_EXTRACT_TIMEOUT`) | Context cancel → error message, continue |
| OCR | Off by default | When on, same per-file timeout applies; OCR may use a nested budget ≤ extract timeout |

Raising the limit is required: PDFs/Office frequently exceed 2 MiB; leaving 2 MiB would make P0 useless.

### Environment / config

| Variable | Required | Default | Meaning |
|----------|----------|---------|---------|
| `SEARCHIFY_OCR` | no | `0` / unset = off | `1`/`true`/`on` enables OCR for images and scanned-PDF fallback |
| `SEARCHIFY_OCR_LANG` | no | `eng` | Passed to OCR engine when OCR on (e.g. Tesseract `-l`) |
| `SEARCHIFY_MAX_FILE_BYTES` | no | `33554432` (32 MiB) | Skip files larger than this during walk |
| `SEARCHIFY_EXTRACT_TIMEOUT` | no | `30s` | Go duration; per-file extract deadline |

OCR runtime dependency (when `SEARCHIFY_OCR=1`): document in README that a local engine (e.g. Tesseract on `PATH`) is required for OCR paths; absence → warnings, not hard crash. Prefer optional build tags only if needed; default binary remains pure Go for non-OCR extract.

### Library / dependency guidance (non-binding, for implementer)

- Prefer **pure Go** extractors that work on **Windows and Linux** (Groundline local deploy).
- Candidates to evaluate: mature ZIP/XML Office readers, PDF text libs, HTML strippers; optional Tesseract binding/CLI for OCR.
- Do **not** pull cloud document APIs.
- If one library covers PDF+docx+xlsx+pptx+html with pure Go, using it behind the `Extractor` interface is fine — keep the interface so formats can be swapped later.

### Version

Bump MCP `serverVersion` from `0.6.2` → **`0.7.0`** (user-visible indexing capability expansion). No DB schema version change expected.

---

## Acceptance criteria

Observable pass/fail:

1. **Allowlist:** Walk indexes P0 + cheap P1 + existing extensions; other extensions still ignored (no error spam per file).
2. **PDF / DOCX / XLSX / CSV:** Fixture corpus under a root → `index_paths` (or CLI `index`) reports `indexed`/`updated` > 0; `search_local` hybrid/keyword returns hits on distinctive strings from those files.
3. **Images + OCR off:** `.png`/`.jpg` under root → skip/warn, process continues; no panic; other files still index.
4. **Images + OCR on** (CI or manual where Tesseract available): image with known text → indexed and searchable via `search_local`.
5. **Fail soft:** Corrupt/truncated PDF in a mixed folder → message + `errors` (or skip), sibling good files still indexed.
6. **Budgets:** File over `SEARCHIFY_MAX_FILE_BYTES` skipped with message; hung extract cancelled by `SEARCHIFY_EXTRACT_TIMEOUT`.
7. **Tool contracts:** Existing MCP/REST/CLI request fields unchanged; clients need no code changes except optional status fields.
8. **Platforms:** `go test ./...` and `go build` succeed on Linux; Windows build must compile without CGO for the default (non-OCR) path.
9. **Watch/prune/remove:** Auto-index watch, `remove_paths`, and `index_prune` continue to work for new extensions (same path-based identity).
10. **Docs (task `docs`):** README lists new env vars + supported extensions; `architecture.md` file-types bullet updated from “today only…” to the shipped allowlist.

---

## Touchpoints

Likely packages (developer confirms):

- `internal/local/walk.go` — extension map + size limit
- `internal/local/indexer.go` — extract-then-chunk instead of raw `ReadFile` as text
- New: `internal/extract` (or similar) — registry + passthrough + format extractors
- `internal/config` — new env vars
- `internal/file` — `search_file` extract path for non-text
- `internal/mcp` — optional `index_status` fields; `serverVersion` → `0.7.0`
- `internal/local/watch.go` — no API change; benefits from wider allowlist
- Tests: fixtures for pdf/docx/xlsx/csv/html + corrupt file; OCR gated or stubbed
- Docs: `README.md`, `docs/architecture.md` (task `docs`)

Must not contradict [architecture.md](../architecture.md): SQLite FTS5 + chunk vectors, path allowlist, MCP tool set, fail-soft indexing remain.

---

## Out of scope

- P2 formats (EPUB, zip bombs, HEIC/BMP, legacy Office) unless a stretch format lands “for free”
- Changing hybrid/RRF/embed model behavior
- Postgres / store adapter
- REST new endpoints
- Guaranteeing OCR quality or multi-language accuracy beyond best-effort `SEARCHIFY_OCR_LANG`

---

## Open questions / non-blocking defaults

| Question | Spec default (implementer follows unless blocked) |
|----------|---------------------------------------------------|
| Unified vs dual size limits | **Unified** `SEARCHIFY_MAX_FILE_BYTES` default 32 MiB for all indexable types |
| OCR engine | **Tesseract on PATH** when OCR on; pure-Go OCR not required for v0.7.0 |
| `.doc` best-effort | **Out** of this ship |
| Scanned PDF without OCR | Index whatever native text exists; if empty → skip + message suggesting `SEARCHIFY_OCR=1` |

---

## Implementation result

### Changes

- `internal/extract` — pluggable registry: passthrough, HTML, PDF, DOCX, XLSX/CSV, images (Tesseract), stretch PPTX/ODF/RTF/EML
- `IndexPaths` / walk / watch use registry + `SEARCHIFY_MAX_FILE_BYTES` / `SEARCHIFY_EXTRACT_TIMEOUT` / OCR flags
- `search_file` extracts non-passthrough formats before keyword scan
- `index_status`: `ocr_enabled`, `index_extensions`; MCP version **0.7.0**

### Verification

- [x] `go test ./...`
- [x] `GOOS=windows GOARCH=amd64 go build` (no CGO)
- [ ] Manual OCR with Tesseract on a real image/scanned PDF

### Deviations from spec

- None material; stretch formats (pptx/odf/rtf/eml) shipped as pure-Go ZIP/XML/text parsers

### Follow-ups

- P2 formats (EPUB, zip, HEIC, legacy Office)
- Stronger in-process OCR without external Tesseract
- Optional REST discovery of supported extensions

---

## Handoff summary (developer scan)

- **Ship P0** (pdf, docx, xlsx, csv, image OCR) + **cheap P1** (extra text + html); stretch pptx/odf/rtf/eml only if pure-Go and cheap; **P2 deferred**.
- Add pluggable **`Extractor` registry**; index pipeline = extract → existing `chunkFile` / FTS / vectors; **fail soft**; **no MCP/REST shape breaks**.
- Config: `SEARCHIFY_OCR` (default off), `SEARCHIFY_OCR_LANG`, `SEARCHIFY_MAX_FILE_BYTES` (default 32MiB), `SEARCHIFY_EXTRACT_TIMEOUT` (default 30s).
- Bump MCP version **0.6.2 → 0.7.0**; no schema migration expected.
- Next tasks: **engine #26** → **verify #27** → **docs #28**; PR later with `Closes #24`.
