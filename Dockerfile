FROM node:20-alpine AS web-builder

WORKDIR /app

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH

RUN corepack enable

COPY package.json pnpm-lock.yaml .npmrc ./
RUN pnpm install --frozen-lockfile

COPY . .

# Root-friendly build for container runtime (no GitHub Pages base path).
ARG PUBLIC_CHIRONE_BASE_PATH=
# Keep sync enabled in-browser via same-origin API calls.
ARG PUBLIC_CHIRONE_SYNC_API_BASE=/
ARG PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=false
ARG PUBLIC_CHIRONE_SYNC_PROJECT=default

ENV PUBLIC_CHIRONE_BASE_PATH=$PUBLIC_CHIRONE_BASE_PATH
ENV PUBLIC_CHIRONE_SYNC_API_BASE=$PUBLIC_CHIRONE_SYNC_API_BASE
ENV PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=$PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE
ENV PUBLIC_CHIRONE_SYNC_PROJECT=$PUBLIC_CHIRONE_SYNC_PROJECT

RUN pnpm run build

FROM golang:1.22-alpine AS collab-builder

WORKDIR /src

COPY go.mod main.go ./
COPY --from=web-builder /app/web/dist ./web/dist

RUN go build -o /out/chirone .

FROM alpine:3.20

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=collab-builder /out/chirone /usr/local/bin/chirone

RUN mkdir -p /app/data && chown -R app:app /app

USER app

VOLUME ["/app/data"]

ENV PORT=8080

CMD ["sh", "-c", "exec /usr/local/bin/chirone --addr :${PORT:-8080} --data-dir /app/data --allow-origin '*'"]
