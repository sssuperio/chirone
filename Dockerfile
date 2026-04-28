FROM alpine:3.20

# Install required tools
RUN apk --no-cache add curl tar

# Create app user
RUN addgroup -S app && adduser -S -G app app

# Download and extract the latest binary from GitHub releases directly to destination
RUN curl -L https://github.com/sssuperio/chirone/releases/latest/download/chirone-Linux-x86_64.tar.gz | tar -xzf - -C /usr/local/bin/ && \
    chmod +x /usr/local/bin/chirone

WORKDIR /app

RUN mkdir -p /app/data && chown -R app:app /app

USER app

VOLUME ["/app/data"]

# Root-friendly build for container runtime (no GitHub Pages base path).
ENV PUBLIC_CHIRONE_BASE_PATH=
# Keep sync enabled in-browser via same-origin API calls.
ENV PUBLIC_CHIRONE_SYNC_API_BASE=/
ENV PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE=false
ENV PUBLIC_CHIRONE_SYNC_PROJECT=default

ENV PORT=8080

CMD ["sh", "-c", "exec /usr/local/bin/chirone --addr :${PORT:-8080} --data-dir /app/data --allow-origin '*'"]
