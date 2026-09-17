# Feature: eia-hardening-infra

Cerrar la deuda informativa del review nativo (A) y agregar infraestructura de entrega (C)
al servicio Go (`service/`) del repo environmental-impact-gemma4, sobre rama nueva con PR.

## Contexto

Los reviews aprobados (review-d30d6e4cfce3d9ab workspace, review-60f129ac0dca9758 committed)
dejaron 3 hallazgos informativos no bloqueantes. En paralelo, el usuario quiere la infra de
entrega (Dockerfile, CI, Makefile, .env.example) y un PR. Decisiones de producto (usuario):

1. **Validación EIA**: flexible + normalizar — validar invariantes (hazard_level, confidence,
   summary) y normalizar arrays (trim, drop vacíos, dedupe) sin rechazar por cantidad.
2. **Imagen Docker**: **Alpine** (elegida sobre distroless para debuggabilidad en vivo).
3. **CI**: Go build/vet/test -race + job de docker build.

## Alcance

### A — Deuda de review
- **R3-001** (`service/internal/eia/eia.go:36-52`): validación incompleta del schema.
  Fix: `(*Analysis).Normalize()` (trim, drop empties, dedupe en orden) aplicado en `Extract`
  antes de `Validate`; `Validate` estricto en invariantes + rechazo de NaN en confidence;
  los conteos del prompt (3-5, 2-4, 1-3) NO rechazan.
- **R3-002** (`service/internal/httpapi/httpapi.go:113`): `ParseMultipartForm(maxMemory)` no
  limita el body real (spillea a disco). Fix: `http.MaxBytesReader` sobre `r.Body` antes de
  parsear, en el path multipart (como ya se hace en el path JSON).
- **R3-003** (`service/web/index.html:203`): el estado de `/health` no se re-renderiza al
  cambiar idioma y los object URLs no se revocan. Fix: guardar estado de health y renderizar
  desde `renderHealth()` invocado en `applyLang()`; revocar object URLs de preview y download.

### C — Infraestructura de entrega
- `service/Dockerfile` multi-stage Alpine (build `golang:1.27-alpine` → runtime `alpine:3`,
  CGO_ENABLED=0, usuario nonroot, EXPOSE 8080) + `service/.dockerignore`.
- `.github/workflows/service-ci.yml`: job Go (build+vet+test -race, working-directory service)
  + job docker build.
- `service/Makefile`: build, run, demo, test, vet, fmt, tidy, docker-build, docker-run, clean.
- ~~`service/.env.example`~~ → **bloqueada por política de paths sensibles del runtime**; decisión del usuario:
  documentar las variables como bloque copiable de `export` en `service/README.md` en su lugar.
- PR de la rama `feat/eia-hardening-infra` a `main` vía `gh` (cuenta GhandyP, autenticada).

## Desviaciones y límites de verificación

- `docker build` local imposible: el daemon deniega al usuario (`/var/run/docker.sock`) y `sudo` pide
  password. La validación de la imagen queda a cargo del job `docker` del CI.
- Actions actualizadas a los mayores vigentes verificados por API: `actions/checkout@v7`,
  `actions/setup-go@v7` (el worker había puesto v5/v6). Base images verificadas: `golang:1.27-alpine`,
  `alpine:3.22` (tag existe en Docker Hub).

## Criterios de aceptación

1. Tests nuevos: Normalize (trim/dedupe/empties), NaN, arrays cortos OK, multipart body
   oversized → 400, y los existentes siguen verdes (`go build/vet/test -race`).
2. `docker build` local exitoso (si el daemon lo permite) y verificado en CI.
3. `make test`, `make build` y `make docker-build` funcionan.
4. PR abierto con descripción que liga los 3 hallazgos y la infra.
5. Review nativo del candidato aprobado (RDD on).

## Tasks (tracking)

1. Tracking ODD (este documento + mirror + todo). ✅
2. A: eia Normalize/Validate + tests. ✅ [worker]
3. A: httpapi MaxBytesReader multipart + tests. ✅ [worker]
4. A: UI health re-render + revocar object URLs. ✅ [worker]
5. C: Dockerfile + .dockerignore + Makefile + .env.example. ✅ (env.example → bloque export en README)
6. C: CI workflow GitHub Actions. ✅ (checkout@v7, setup-go@v7)
7. Commit por work unit + rama + push + PR. ⏳ [inline]
8. Review nativo del candidato + verificación final. ⏳