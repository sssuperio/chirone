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
- accepts project writes (`PUT /api/project`)
- dumps each project snapshot to one JSON file on every change

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
