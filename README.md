# Chirone

Font design playground built with SvelteKit.

## Local development

```bash
pnpm install
pnpm run dev -- --open
```

## Realtime collaboration (SSE + Go)

This repository includes a Go collaboration server that:

- streams live project updates over SSE (`/api/events`)
- accepts versioned writes (`baseVersion`) and rejects stale updates with `409 Conflict`
- supports per-entity realtime writes:
  - glyph upsert/delete (`PUT/DELETE /api/glyph`)
  - syntax upsert/delete (`PUT/DELETE /api/syntax`)
  - metrics update (`PUT /api/metrics`)
- keeps compatibility with full snapshot writes (`PUT /api/project`)
- dumps both aggregate snapshots (`data/<project>.json`) and split entity files (`data/<project>/glyphs`, `data/<project>/syntaxes`, `data/<project>/metrics.json`)
  - split glyph/syntax filenames are based on entity `name` (for example `A.json`, `b.json`)
  - if multiple entities share the same name, the server appends the id suffix to avoid overwrite

Start the server:

```bash
cd collab-server
go run . --addr :8090 --data-dir ./data
```

or with Task:

```bash
task collab:server
```

Set frontend env vars (`.env`, see `.env.example`):

```bash
VITE_COLLAB_SERVER=http://localhost:8090
VITE_COLLAB_PROJECT=default
```

When `VITE_COLLAB_SERVER` is set, the app syncs `glyphs`, `syntaxes`, and `metrics` in realtime.

## Docker

Build a single image containing:

- prebuilt SvelteKit static app (`docs`)
- Go collaboration server (serving UI + `/api/*` + SSE)

Build:

```bash
docker build -t chirone:latest .
```

Run:

```bash
docker run --rm -p 8080:8080 -v chirone-data:/app/data chirone:latest
```

Open: `http://localhost:8080/glyphs`

With Docker Compose:

```bash
docker compose up --build
```

Use a custom port:

```bash
PORT=8090 docker compose up --build
```

Build-time variables (for static frontend build):

```bash
BUILD_PUBLIC_BASE_PATH= \
BUILD_VITE_COLLAB_SERVER=/ \
BUILD_VITE_COLLAB_PROJECT=default \
docker compose build
```

Runtime variables (container process):

- `PORT` controls the collab server listen port inside the container (and published port in compose)

SHA endpoint:

- `GET /api/version` returns the current git SHA (`{"sha":"..."}`)
- SHA is detected automatically from git metadata at build/runtime (no version build-arg required).

### Publish to GHCR

```bash
docker tag chirone:latest ghcr.io/<owner>/chirone:latest
docker push ghcr.io/<owner>/chirone:latest
```
