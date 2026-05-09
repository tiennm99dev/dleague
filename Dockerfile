# Multi-stage Dockerfile for dleague.
#
# Stage 1: Build Go server binary (static, no CGO).
# Stage 2: Build SvelteKit web client.
# Stage 3: Minimal distroless runtime image.
#
# Build context: repo root (go.work + server/ + web/ must all be present).

# ── Stage 1: Go build ─────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS go-builder

WORKDIR /src

# Copy workspace files first (enables module caching).
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY server/ ./server/

# Build static binary; -s -w strips debug symbols for smaller output.
# GOTOOLCHAIN=auto allows Go 1.24 to auto-download the 1.26 toolchain declared
# in go.work if needed. CGO_ENABLED=0 produces a fully static binary for distroless.
RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto go build \
    -ldflags='-s -w' \
    -o /out/server \
    ./server/cmd/api

# ── Stage 2: Node / SvelteKit build ───────────────────────────────────────────
FROM node:20-alpine AS node-builder

WORKDIR /src/web

# Install dependencies (respects package-lock.json).
COPY web/package.json web/package-lock.json ./
RUN npm ci --prefer-offline

# Copy source and generated proto stubs.
COPY web/ .
COPY shared/pb/ /src/shared/pb/

RUN npm run build

# ── Stage 3: Minimal runtime ──────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:latest

# Server binary.
COPY --from=go-builder /out/server /server

# SvelteKit static output (served by Go FileServer at /).
COPY --from=node-builder /src/web/dist/ /web/dist/

# Default environment.
ENV DLEAGUE_ADDR=:8080
ENV DLEAGUE_WEB=/web/dist
ENV DLEAGUE_ENV=production

EXPOSE 8080

ENTRYPOINT ["/server"]
