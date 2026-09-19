# Feature: eia-reasoning-guard

Blindar el provider local del servicio para que el análisis JSON salga aunque el
`llama-server` esté arrancado con razonamiento encendido, y cerrar el finding
informativo pendiente del review.

## Contexto

De la verificación real (feature `eia-real-verification`) quedó un aprendizaje de campo: con
Gemma 4 y razonamiento activo, el modelo gasta **todo** el presupuesto de tokens en
`reasoning_content` y nunca emite el JSON, y el servicio respondía 502/504. En ese momento la
solución fue arrancar el server con `--reasoning off --reasoning-budget 0` — pero eso depende de
quien opera el server, no del servicio.

Además quedó abierto un finding informativo del review (`R3-lenient-object-elements`): un objeto
anidado dentro de una lista de strings se convertía en texto basura tipo `map[a:1]`.

## Verificación empírica previa (real, no simulada)

Contra `llama-server` (build 18a04f0) + `gemma-4-E2B-it-Q4_0` + mmproj, **arrancado SIN flags de
reasoning**, mismo prompt:

| Request | `reasoning_content` | completion tokens |
| --- | --- | --- |
| sin campos (comportamiento previo del servicio) | presente (29 palabras) | ~300 (agota el budget) |
| `reasoning_effort: "none"` | ausente | 2 |
| `chat_template_kwargs: {"enable_thinking": false}` | ausente | 2 |

Soporte confirmado en el código de llama.cpp (`tools/server/server-common.cpp:1346-1353`:
`reasoning_effort == "none"` desactiva el razonamiento por request).

## Alcance

1. `internal/llm`: el provider local envía `reasoning_effort: "none"` en cada request
   (`localRequestBody`, extraído para poder testearlo).
2. `internal/eia`: los elementos anidados (objetos/arrays) dentro de listas se **descartan** en
   vez de stringificarse (`scalarText` con `ok bool`), cerrando `R3-lenient-object-elements`.
3. Tests de regresión para ambos.
4. README del servicio: documenta que el servicio ya no depende de los flags del server.

## Criterios de aceptación

1. `gofmt` limpio, `go build`, `go vet`, `go test -race` verdes.
2. Test que falla si el request local no incluye `reasoning_effort: "none"`.
3. Test que prueba que `{"depth":2}` y `[1,2]` se descartan y los escalares se conservan.
4. Commit por work unit + PR + review nativo.

## Tasks (tracking)

1. Verificación empírica del soporte por request. ✅
2. Fix provider local + test. ✅
3. Fix elementos anidados + test. ✅
4. Docs (README servicio). ✅
5. Commits + push + PR #4 + review. ✅ PR #4 MERGEADO (rebase), review review-60325df55c22b7e2 aprobado sin hallazgos y quemado, CI verde (test 18s + docker 43s). main = 236f6dd.

## Notas

- **Verificación E2E completa no ejecutada por costo**: un `/analyze` real contra el modelo en
  CPU tarda 8-12 minutos. El usuario indicó saltearla. La evidencia que sí existe es la de la
  tabla de arriba (server real, mismo modelo) más el unit test que prueba que el servicio manda
  el campo en cada request; la composición de ambas cierra el caso.
- Queda como deuda el finding informativo del review de esta iteración, si aparece.