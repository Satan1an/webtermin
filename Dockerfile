# syntax=docker/dockerfile:1.7
#
# Multi-arch build. Use buildx to target arm64 (OrangePi 5 Pro):
#   docker buildx build --platform linux/arm64 -t webtermin:arm64 --load .
# Or for both:
#   docker buildx build --platform linux/amd64,linux/arm64 -t webtermin:latest --push .

# --- 1. Build the React SPA (always on the build host — pure JS) ---
FROM --platform=$BUILDPLATFORM node:20-bookworm-slim AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm npm install
COPY web/ ./
RUN npm run build

# --- 2. Cross-compile the Go binary natively on the build host ---
# CGO_ENABLED=0 + modernc.org/sqlite means no C toolchain is needed; we just
# set GOARCH from buildx's TARGETARCH and produce a static arm64/amd64 binary.
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS go
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
# Inject the freshly-built SPA so go:embed picks it up.
COPY --from=web /web/dist ./web/dist
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/webtermin ./cmd/webtermin

# --- 3. Minimal runtime (target-arch image) ---
# Use Debian slim instead of distroless: we need journalctl, useradd, chpasswd,
# nft, etc. on PATH for the management modules to function.
FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
        systemd \
        passwd \
        ca-certificates \
        tzdata \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*
WORKDIR /app
COPY --from=go /out/webtermin /app/webtermin
COPY config.example.yaml /app/config.example.yaml

# Data volume holds SQLite db, generated TLS cert, sessions.
VOLUME ["/app/data"]
EXPOSE 8443

# Default config goes to /app/config.yaml; users mount their own to override.
ENV WEBTERMIN_CONFIG=/app/config.yaml
ENTRYPOINT ["/app/webtermin"]
CMD ["-config", "/app/config.yaml"]
