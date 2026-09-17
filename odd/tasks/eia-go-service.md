# Feature: eia-go-service

Servicio Go servible para demo ante cliente: análisis de impacto ambiental por imagen con
modelo híbrido (Gemma local vía llama.cpp → fallback Gemini cloud) y UI web bilingüe.

## Contexto

El repo actual tiene 2 notebooks Gemma 4 (inferencia single-shot imagen → JSON EIA).
El cliente quiere algo servible en Go para mostrar. Decisiones de producto (usuario, 2026-09):

1. **Motor**: híbrido con fallback — Gemma local (llama.cpp OpenAI-compatible) primero, fallback Gemini cloud.
2. **Entrega**: API REST + UI web autocontenida.
3. **Alcance**: análisis base fiel al notebook (mismas claves JSON).
4. **Idioma**: UI/prompt EN por defecto con toggle ES (claves JSON siempre en inglés).

APIs oficiales verificadas (2026):
- Gemini Go SDK: `google.golang.org/genai` (repo googleapis/go-genai, v1.66). Cliente API key:
  `genai.NewClient(ctx, &genai.ClientConfig{APIKey: ..., Backend: genai.BackendGeminiAPI})`.
  Llamada multimodal: `client.Models.GenerateContent(ctx, model, contents, config)` con
  `Part{Text, InlineData: &genai.Blob{Data, MIMEType}}` y `GenerateContentConfig{ResponseMIMEType: "application/json"}`.
  Modelo actual: `gemini-flash-latest` (configurable).
- Gemma local: `llama-server` expone `/v1/chat/completions` (OpenAI shape, part `image_url`
  con data URI base64) y `GET /health`. Modelo GGUF multimodal chico: `ggml-org/gemma-4-E2B-it-GGUF`.

## Arquitectura (stdlib net/http, Go 1.27, sin router externo)

```
service/
  go.mod                      module github.com/GhandyP/environmental-impact-gemma4/service
  cmd/server/main.go          entrypoint, flags, wiring
  internal/config/config.go   env config: addr, timeouts, modelo ids, llama base url, API key
  internal/eia/prompt.go      prompt builder EN/ES (claves JSON idénticas al notebook)
  internal/eia/parse.go       extraction robusta de bloque JSON + validación
  internal/eia/result.go      struct EIA + Validate()
  internal/llm/llm.go         interfaz Analyzer + tipo ProviderInfo
  internal/llm/local.go       cliente llama.cpp OpenAI-compatible
  internal/llm/gemini.go      cliente genai SDK
  internal/llm/hybrid.go      router local→gemini con health checks
  internal/httpapi/server.go  mux: POST /analyze, GET /health, GET / (UI embebida)
  internal/httpapi/embed.go   go:embed web/
  web/index.html              UI autocontenida bilingüe (EN/ES toggle)
```

## Contrato API

- `POST /analyze`
  - body multipart `image` (archivo) + campo `lang=en|es`; o JSON `{"image_b64": "...", "lang": "es"}`.
  - response 200: `{"provider":"local"|"gemini","model":"...","lang":"...","analysis":{...claves EIA...}}`
  - errores 400 (imagen inválida/tamaño), 502 (ningún provider disponible), 504 (timeout).
  - header `X-EIA-Provider`.
  - límite 10 MB; validar decodificación de imagen; timeouts por provider.
- `GET /health` → `{"status":"ok","providers":{"local":{"available":bool,"model":"..."},"gemini":{"available":bool,"model":"..."}}}`.
  local = GET {base}/health responde; gemini = API key configurada (sin llamada).
- `GET /` → UI embebida.

## Claves JSON del análisis (fiel al notebook)

`hazard_level` (low|medium|high|critical), `summary`, `visible_evidence[]`, `likely_impact_factors[]`,
`likely_processes[]`, `recommendations[]`, `uncertainty[]`, `confidence` (0..1).

## Criterios de aceptación

1. `go build ./...`, `go vet ./...`, `go test ./...` verdes (tests: parser, providers con httptest, handler).
2. Demo sin GPU ni key: provider fake (httptest) responde /analyze con JSON válido — viable en esta máquina.
3. Con `EIA_LLAMA_BASE_URL` apuntando a llama-server, el tráfico va local; sin él o con error → fallback Gemini (si hay key).
4. UI funcional: subida, preview, análisis formateado, descarga JSON, toggle EN/ES.
5. README de demo por cada modo (fake / llama / gemini) + ejemplos curl.

## Tasks (tracking)

1. Scaffold módulo Go + config (go.mod, internal/config, cmd/server stub). [inline] ✅ build+vet verdes
2. Paquete EIA: prompt bilingüe + parser + validación + tests. [worker] ✅
3. Paquete LLM: interfaz, local, gemini, híbrido + tests (httptest). [worker] ✅ (iteración extra: worker omitió tests al primer pase; completados en continue)
4. HTTP API: handlers + embed UI + tests. [worker] ✅
5. UI web index.html bilingüe. [inline] ✅ (+ provider mock EIA_MOCK=1 para demo sin GPU/key)
6. Docs demo (README servicio + README raíz + .gitignore) + verificación final. ✅ gentle-ai-verify: build/vet/test -race PASS; demo en vivo PASS (health, analyze EN/ES con imagen real, UI HTML, 400 en no-imagen). Códigos HTTP explícitos confirmados: /health 200, / 200, analyze EN 200, analyze ES 200, no-imagen 400, lang inválido 400.