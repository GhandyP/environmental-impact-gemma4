# EIA Service (Go demo)

A small, self-contained HTTP service that turns a site photo into a structured
environmental impact assessment — the notebook flow from this repository,
packaged as a servable Go API plus a bilingual (EN/ES) web UI for live demos.

Model routing is hybrid, in order:

1. **Local Gemma** — a `llama.cpp` `llama-server` exposing the
   OpenAI-compatible API (open weights, no cloud).
2. **Gemini cloud** — the official `google.golang.org/genai` SDK (needs an API key).
3. **Mock** — a deterministic canned analysis (`EIA_MOCK=1`), so the whole
   product can be demoed on any laptop without GPU or keys.

## Quick start (demo mode, no GPU / no key)

```bash
cd service
go build -o eia-service ./cmd/server
EIA_MOCK=1 EIA_ADDR=:8080 ./eia-service
```

Open <http://localhost:8080> — upload an image, pick EN or ES, analyze,
download the JSON. Equivalent curl:

```bash
curl -s http://localhost:8080/health | jq
curl -s -F image=@../assets/alfred_palmer_smokestacks.jpg -F lang=en \
     http://localhost:8080/analyze | jq
```

## Local Gemma mode (llama.cpp)

Serve a multimodal Gemma build locally, then point the service at it:

```bash
# one-time: fetch a small multimodal GGUF (CPU-friendly)
llama-server -hf ggml-org/gemma-4-E2B-it-GGUF --port 8081

cd service
EIA_LLAMA_BASE_URL=http://127.0.0.1:8081 EIA_ADDR=:8080 ./eia-service
```

The service checks `GET {base}/health`; when the local server is down or
unconfigured, requests fall back to Gemini (if a key is set).

## Gemini cloud mode

```bash
export GEMINI_API_KEY=your-key   # or GOOGLE_API_KEY
EIA_ADDR=:8080 ./eia-service
```

Default model id is `gemini-flash-latest` (override with `EIA_GEMINI_MODEL`).
Responses request `application/json` output so parsing stays deterministic.

## API

| Route | Description |
| --- | --- |
| `POST /analyze` | multipart (`image` file + optional `lang=en|es`) or JSON body `{"image_b64": "...", "lang": "es"}`. Returns `{"provider","model","lang","analysis"}`; header `X-EIA-Provider`. Errors: 400 invalid input/image, 502 provider or parse failure, 503 no provider available, 504 timeout. |
| `GET /health` | `{"status":"ok","providers":[{"name","available","model"}...]}` |
| `GET /` | Embedded bilingual web UI |

The `analysis` object mirrors the notebook schema exactly (English keys):
`hazard_level` (`low|medium|high|critical`), `summary`, `visible_evidence`,
`likely_impact_factors`, `likely_processes`, `recommendations`, `uncertainty`,
`confidence` (0–1).

## Configuration (environment)

| Variable | Default | Meaning |
| --- | --- | --- |
| `EIA_ADDR` | `:8080` | HTTP listen address |
| `EIA_MOCK` | `false` | Deterministic demo provider, no network |
| `EIA_LLAMA_BASE_URL` | *(empty = off)* | llama.cpp server base URL |
| `EIA_LLAMA_MODEL` | `gemma-4-E2B-it` | Model id sent to the local server |
| `GEMINI_API_KEY` / `GOOGLE_API_KEY` | *(empty = off)* | Gemini API key |
| `EIA_GEMINI_MODEL` | `gemini-flash-latest` | Gemini model id |
| `EIA_MAX_IMAGE_BYTES` | `10485760` | Image size cap (bytes) |
| `EIA_PROVIDER_TIMEOUT` | `120s` | Per-request provider timeout |

## Development

```bash
go build ./... && go vet ./... && go test ./...
```

Layout: `cmd/server` (entrypoint), `internal/config`, `internal/eia`
(prompt + JSON schema/parse), `internal/llm` (local / gemini / mock / hybrid
providers), `internal/httpapi` (routes), `web/` (embedded UI).
