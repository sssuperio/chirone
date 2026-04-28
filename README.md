# Chirone

Font design playground with a Svelte web UI embedded inside a single Go binary.

## Install

### Linux/macOS with curl | tar

macOS Apple Silicon:

```bash
curl -L https://github.com/sssuperio/chirone/releases/latest/download/chirone-Darwin-arm64.tar.gz | sudo tar -xzf - -C /usr/local/bin/
sudo chmod +x /usr/local/bin/chirone
```

Linux x86_64:

```bash
curl -L https://github.com/sssuperio/chirone/releases/latest/download/chirone-Linux-x86_64.tar.gz | sudo tar -xzf - -C /usr/local/bin/
sudo chmod +x /usr/local/bin/chirone
```

macOS Intel:

```bash
curl -L https://github.com/sssuperio/chirone/releases/latest/download/chirone-Darwin-x86_64.tar.gz | sudo tar -xzf - -C /usr/local/bin/
sudo chmod +x /usr/local/bin/chirone
```

Linux arm64:

```bash
curl -L https://github.com/sssuperio/chirone/releases/latest/download/chirone-Linux-aarch64.tar.gz | sudo tar -xzf - -C /usr/local/bin/
sudo chmod +x /usr/local/bin/chirone
```

### With mise

```bash
mise use -g github:sssuperio/chirone@latest
```

### With Go

```bash
go install github.com/sssuperio/chirone@latest
```

## Local development

```bash
pnpm install
pnpm run dev -- --open
```

## Realtime collaboration (SSE + Go)

This repository includes a Go server that:

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
go run . --addr :8090 --data-dir ./data
```

or with Task:

```bash
task collab:server
```

Set frontend env vars (`.env`, see `.env.example`):

```bash
PUBLIC_CHIRONE_SYNC_API_BASE=http://localhost:8090
PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=false
PUBLIC_CHIRONE_SYNC_PROJECT=default
```

When `PUBLIC_CHIRONE_SYNC_API_BASE` is set, the app syncs `glyphs`, `syntaxes`, and `metrics` in realtime.

`PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE` controls whether users may change the sync backend from the frontend settings page:

- `false` (default): the sync backend is fixed at build time
- `true`: the `Impostazioni` page exposes a backend field and stores the override in browser localStorage

### What `Collab non configurato` means

If the UI shows:

```text
Collab non configurato.
```

the frontend was started without a collaboration server URL. In that state:

- realtime sync is disabled
- the `Revisioni` page cannot talk to the Go backend
- edits stay local to the browser session unless you export manually

To enable collaboration in local frontend development:

1. Start the Go server:

```bash
go run . --addr :8090 --data-dir ./data
```

2. Create a `.env` file or export the variables before starting Vite:

```bash
PUBLIC_CHIRONE_SYNC_API_BASE=http://localhost:8090
PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=true
PUBLIC_CHIRONE_SYNC_PROJECT=default
```

3. Start the frontend:

```bash
pnpm run dev -- --open
```

After that, the navbar should show the collaboration status and the `Revisioni` page will work.

### Collaboration modes

There are two normal ways to use collaboration:

1. Separate frontend + backend during development

```bash
go run . --addr :8090 --data-dir ./data
PUBLIC_CHIRONE_SYNC_API_BASE=http://localhost:8090 PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=true PUBLIC_CHIRONE_SYNC_PROJECT=default pnpm run dev
```

2. Same-origin from the single binary or Docker image

In release builds, Docker, and the embedded single binary, the web app and the API are served by the same `chirone` process. In that setup the frontend should use:

```bash
PUBLIC_CHIRONE_SYNC_API_BASE=/
PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=false
PUBLIC_CHIRONE_SYNC_PROJECT=default
```

That is already what the Dockerfile and release workflow build with.

### Production recommendation

For production, keep:

```bash
PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=false
```

This prevents users from pointing the frontend at arbitrary collaboration backends from the browser settings page.

## Single binary build

```bash
pnpm install
pnpm run build
go build -o chirone .
./chirone
./chirone version
```

The release build embeds the static web app from `web/dist` using `go:embed`, so published binaries serve the UI and the collaboration API from the same executable.

## Docker

Build a single image containing the embedded web app and the Go server.

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
BUILD_CHIRONE_BASE_PATH= \
BUILD_CHIRONE_SYNC_API_BASE=/ \
BUILD_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=false \
BUILD_CHIRONE_SYNC_PROJECT=default \
docker compose build
```

Runtime variables (container process):

- `PORT` controls the Chirone listen port inside the container (and published port in compose)

Version endpoint:

- `GET /api/version` returns the release version and git SHA (`{"version":"...","sha":"..."}`)
- the binary version is injected by GoReleaser via `-X main.version=...`
- SHA is still detected automatically from build info or local git metadata.

### Publish to GHCR

```bash
docker tag chirone:latest ghcr.io/<owner>/chirone:latest
docker push ghcr.io/<owner>/chirone:latest
```
