FROM node:20-alpine AS web-builder

WORKDIR /app

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH

RUN corepack enable

COPY package.json pnpm-lock.yaml .npmrc ./
RUN pnpm install --frozen-lockfile

COPY . .

# Root-friendly build for container runtime (no GitHub Pages base path).
ARG PUBLIC_BASE_PATH=
# Keep collab enabled in-browser via same-origin API calls.
ARG VITE_COLLAB_SERVER=/
ARG VITE_COLLAB_PROJECT=default

ENV PUBLIC_BASE_PATH=$PUBLIC_BASE_PATH
ENV VITE_COLLAB_SERVER=$VITE_COLLAB_SERVER
ENV VITE_COLLAB_PROJECT=$VITE_COLLAB_PROJECT

RUN pnpm run build

FROM golang:1.22-alpine AS collab-builder

WORKDIR /src

COPY collab-server ./collab-server

WORKDIR /src/collab-server

RUN go build -o /out/chirone-collab .

FROM alpine:3.20

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=collab-builder /out/chirone-collab /usr/local/bin/chirone-collab
COPY --from=web-builder /app/docs /app/ui

RUN mkdir -p /app/data && chown -R app:app /app

USER app

VOLUME ["/app/data"]

ENV PORT=8080

CMD ["sh", "-c", "exec /usr/local/bin/chirone-collab --addr :${PORT:-8080} --data-dir /app/data --allow-origin '*' --ui-dir /app/ui"]
