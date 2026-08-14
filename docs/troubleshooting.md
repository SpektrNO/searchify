# Troubleshooting

## LangSearch rerank returns 500 / `rerank failed`

Local search with `rerank: true` (MCP `search_local` or REST `POST /v1/search`) calls LangSearch `POST /v1/rerank`. If that upstream call fails, Searchify surfaces `rerank failed: …` and the whole search errors (hybrid without rerank still works).

**Isolate the API first** (same key Searchify uses; do not paste the key into tickets):

```bash
# Load key from env or: set -a && source .env && set +a
curl -sS https://api.langsearch.com/v1/rerank \
  -H "Authorization: Bearer ${LANGSEARCH_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "langsearch-reranker-v1",
    "query": "Tell me the key points of Alibaba 2024 ESG report",
    "top_n": 2,
    "return_documents": true,
    "documents": [
      "Alibaba Group released the 2024 ESG report with carbon reduction progress.",
      "Chocolate cake recipe with frosting and sprinkles."
    ]
  }'
```

| Response | Meaning |
|----------|---------|
| `code` 200 + `results` | Key and rerank engine OK — debug Searchify env / process next |
| `401` / `Invalid API KEY` | Key missing, wrong, or not exported in this shell |
| `404` / `Invalid rerank model` | Typo in `model` (expect `langsearch-reranker-v1`) |
| `429` | Rate limit (free tier ~1 QPS / 60/min / 1000/day) — wait and retry |
| `500` / `rerank engine error` | LangSearch-side failure; note `log_id` and check [dashboard](https://langsearch.com/dashboard) / support. Confirm web search still works with the same key via `POST /v1/web-search`. |

Docs: [Semantic Rerank API](https://docs.langsearch.com/api/semantic-rerank-api), [API limits](https://docs.langsearch.com/limits/api-limits).

## Vector search: embed_model mismatch

Error like `index embed_engine="kjarni" embed_model="minilm-l6-v2" but config engine="ollama" model="nomic-embed-text"` (or a model-only mismatch) means the SQLite index was built with a different embedding stack than the running process. Dimensions / geometry must not be mixed.

```bash
# Match the engine+model you want going forward, then rebuild vectors:
export SEARCHIFY_EMBED_ENGINE=ollama
export SEARCHIFY_EMBED_MODEL=nomic-embed-text
./bin/searchify embed --force
```

Keyword `mode=keyword` still works without re-embed. After a successful force embed, `index_status.embed_engine` / `embed_model` should match the env.

Ollama must be running for both `embed` and vector/hybrid search (`curl -sS http://127.0.0.1:11434/api/tags`). Use an **embedding** model, not a chat-only tag.

## Chunk settings changed / odd retrieval boundaries

Chunk packing uses `SEARCHIFY_CHUNK_BYTES` / `SEARCHIFY_CHUNK_OVERLAP` plus structure splits (Markdown headings, `\f` page breaks). The index does **not** auto-detect stale chunking — after changing those env vars (or upgrading chunk logic), rebuild:

```bash
./bin/searchify index --force <paths…>
# if vectors needed:
./bin/searchify embed --force
```
