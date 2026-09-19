# Feature: eia-real-verification

Cerrar las dos brechas de verificación que quedaron abiertas tras mergear PR #1 y PR #2:
el camino real a un modelo local (llama.cpp) nunca se ejercitó, y la UI nunca se abrió en un
navegador real. Pedido del usuario: "puedes mejorar esos errores, si la respuesta es si
mejoralo y luego commitea y pushea".

## Contexto

Lo que ya está en `main`: servicio Go con ruteo híbrido (mock → llama.cpp → Gemini), API REST,
UI bilingüe embebida, Dockerfile Alpine, Makefile y CI (test + docker). Cuatro hallazgos de
review cerrados. Deuda de código abierta: ninguna.

Brechas de verificación detectadas (declaradas al usuario, no ocultas):

1. **El proveedor local (`internal/llm/local.go`) nunca habló con un `llama-server` real.**
   Los tests usan `httptest` con un fake que imita el shape OpenAI. Nadie verificó que el
   formato de request (`image_url` con data URI base64), el `mmproj` multimodal y el parseo
   de `choices[0].message.content` funcionen contra llama.cpp de verdad.
2. **La UI nunca se abrió en un navegador real.** Solo se verificó con `curl` (200 + HTML).
   Sin ejercitar: toggle EN/ES, drag&drop, preview, barra de confianza, descarga JSON,
   re-render del health al cambiar idioma (R3-003) y revocación de object URLs.

## Entorno verificado (2026-09-18)

- 200 GB libres, 4 cores, `g++`, `make` presentes; `cmake 4.4.3` instalado sin sudo en
  `~/.local/cmake` (symlink en `~/.local/bin`).
- `node v26.8.2` + `playwright-core` instalado en `/tmp/eia-ui-test`;
  **Chrome for Testing 153.0.8010.12** ya presente en `~/.cache/ms-playwright/chromium-1243/`.
- Modelo elegido: `ggml-org/gemma-4-E2B-it-GGUF` (**no gated**) con
  `mmproj-gemma-4-E2B-it-Q8_0.gguf` (proyector de visión obligatorio para multimodal).
  Descarga a `~/models/gemma-4-E2B/` (Q4_0 ~99 MB + mmproj).

## Alcance

1. Compilar `llama-server` desde `ggml-org/llama.cpp` (Release, `-DLLAMA_CURL=OFF`).
2. Levantar el server con el modelo + mmproj y verificar `GET /health`.
3. Correr el servicio EIA en modo real (`EIA_LLAMA_BASE_URL`) — **sin** `EIA_MOCK` — y hacer
   `POST /analyze` con `assets/alfred_palmer_smokestacks.jpg`. Criterio: 200, `provider:"local"`,
   JSON EIA válido con las 8 claves y `hazard_level` coherente.
4. Probar la UI con Playwright sobre el server real: toggle EN→ES, subida de archivo,
   preview visible, análisis renderizado, descarga JSON y estado de health traducido.
5. Documentar los resultados reales en `service/README.md` (sección de verificación) y
   registrar cualquier defecto encontrado.

## Criterios de aceptación

1. `POST /analyze` contra Gemma local real devuelve 200 con JSON EIA válido (evidencia textual).
2. La UI pasa las comprobaciones de Playwright en EN y ES, incluyendo el re-render del health.
3. Cualquier bug encontrado se arregla con test de regresión y se documenta.
4. README actualizado con la receta real verificada (flags exactos de llama-server).
5. Commit por work unit + push; review nativo del candidato si hay cambios de código.

## Tasks (tracking)

1. Relevar entorno y preparar toolchain (cmake, playwright). ✅
2. Compilar llama-server. ⏳
3. Descargar Gemma 4 E2B + mmproj. ⏳
4. Ejercitar /analyze contra el modelo real y registrar evidencia. ⏳
5. Probar la UI en navegador real (Playwright, EN/ES). ⏳
6. Arreglar defectos encontrados + tests de regresión. ⏳
7. Actualizar README con la receta verificada. ⏳
8. Commit por work unit + push + review. ⏳

## Hallazgos

### H1 — Gemma 4 gasta todo el presupuesto en `reasoning_content` (bloqueante en la práctica)

Con `max_tokens: 512` y razonamiento activo por defecto, el modelo emitió **298 tokens de
`reasoning_content` y cero de `content`**: el análisis nunca llegaba a producirse. El servicio
respondía 504 por timeout.

**Fix verificado:** `llama-server --reasoning off --reasoning-budget 0`. Con eso la respuesta
sale limpia y directa (verificado: 2 tokens, `content: "OK"`).

### H2 — BUG REAL: el modelo colapsa listas de un ítem a string y el parser explotaba

Respuesta real del modelo:
`"uncertainty":"operation status unclear"` (string, no array).

Resultado: `json: cannot unmarshal string into Go struct field Analysis.uncertainty of type []string`
→ HTTP 502. **El servicio se caía con una respuesta perfectamente razonable de un modelo real.**
Ningún test con fakes podía detectarlo: los fakes devolvían siempre el schema ideal.

**Fix:** tipo `listField` con `UnmarshalJSON` que acepta array de strings, string suelto, `null`,
listas vacías y escalares sueltos dentro del array (se renderizan a texto). Aplicado a los cinco
campos de lista. Test de regresión con el JSON real que provocó el fallo.

### H3 — El health de la UI puede mostrar `—` por carrera de arranque (no es bug)

Si el `fetch('/health')` de la UI sale antes de que el servicio esté escuchando, la lista de
proveedores queda vacía y el texto muestra `providers: —`. Verificado que con el servicio ya
levantado la UI muestra `Service up · providers: local` y el dot en verde. Es comportamiento
esperado de un fetch único sin reintento; no se cambia (el contrato del endpoint no lo pide).

### H4 — Inferencia de visión en CPU: ~8-12 min por análisis

Medido en 4 cores: prompt processing ~1.8 tok/s, generación ~0.43 tok/s. Un análisis EIA
completo (200-300 tokens de salida) tarda **entre 8 y 12 minutos**. El default de
`EIA_PROVIDER_TIMEOUT` (120s) es insuficiente para Gemma local en CPU y produce 504.

**Acción:** documentado en el README con el timeout recomendado para modo local CPU.

### H5 — BUG CRÍTICO DE UI: `show()` no estaba definida y el botón Analizar no hacía nada

Encontrado **sólo** abriendo la UI en un navegador real. La consola del browser reportaba:

```
PAGEERROR: show is not defined
```

El handler `$("go").onclick` llamaba a `show("", false)` en su segunda línea, pero la función
nunca se había definido: la `ReferenceError` cortaba el handler **antes del `fetch`**, así que
`POST /analyze` nunca salía y el botón quedaba colgado en "Analyzing…" para siempre.

Impacto: **el botón principal de la demo no funcionaba**. Un usuario humano habría visto un
botón que no responde. `curl` no podía detectarlo (el endpoint funcionaba bien); ningún test
unitario tampoco (el JS no se ejecutaba).

**Fix:** definir `show(message, isError)` que renderiza el mensaje en `#status` con el estilo
adecuado (`.err` / `.info`) y limpia el área cuando el mensaje es vacío.
Verificado: `typeof show: function` → `POST /analyze` enviado → análisis completo renderizado.

### H6 — BUG DE UI: el toggle de JSON crudo nunca podía mostrarse

El elemento arranca con `.json{display:none}` y el handler alternaba la clase `hidden`, que
también es `display:none` — el panel **nunca** se hacía visible, sin importar cuántas veces
se hiciera clic.

**Fix:** clase explícita `.json.open{display:block}` y el toggle alterna `open`.
Verificado en navegador: oculto → visible → oculto.

## Evidencia de verificación real (completa)

- `llama-server` compilado desde `ggml-org/llama.cpp` (build 18a04f0, llama-server 0.4.1-dev).
- Modelo: `gemma-4-E2B-it-Q4_0.gguf` (2.7 GB) + `mmproj-gemma-4-E2B-it-Q8_0.gguf` (532 MB),
  cargado como multimodal (`loaded multimodal model`).
- `POST /analyze` **real** (sin mock) → HTTP 200:
  `provider: "local"`, `model: "gemma-4-E2B-it"`, `hazard_level: "medium"`, `confidence: 0.8`,
  las 8 claves presentes, 7m49s.
- UI en Chrome for Testing 153 con Playwright, **contra el modelo real**: 15/16 checks PASS en la
  primera pasada (los dos FAIL destaparon H5 y H6), y **16/16 tras los fixes**, incluyendo:
  - badge `High · high`, resumen renderizado, 20 ítems de evidencia, barra de confianza al 90%
  - `local · gemma-4-E2B-it` mostrado como proveedor
  - descarga de `gemma4_eia_analysis.json` con las 8 claves del schema
  - toggle EN/ES con re-render del health (fix R3-003 confirmado en navegador)
  - cero errores de consola/página tras los fixes
- Captura de pantalla de la UI con el análisis real: `/tmp/eia-ui-proof.png`