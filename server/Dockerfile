# Multi-stage build for the dleague server image.
# Build context: repo root (so we can include the sibling `shared/` module
# referenced by the `replace` directive in `server/go.mod`, and the
# `client/web/` source for the Svelte+Phaser bundle).
#
# Final image bundles two Go binaries + the static web build:
#   /usr/local/bin/dleague           — HTTP+WS API server (cmd/api)
#   /usr/local/bin/dleague-export    — migration export CLI (cmd/dleague-export)
#   /app/web/                        — Vite/Svelte build output (served by API)
#
# Multi-arch: build with `docker buildx build --platform linux/amd64,linux/arm64`
# (see `make image`). Coolify host is OCI Ampere A1 = linux/arm64.

ARG GO_VERSION=1.25.5
ARG NODE_VERSION=22
ARG ALPINE_VERSION=3.20

# ─── Stage 1: Go binaries ─────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine AS gobuild
WORKDIR /src

COPY shared/ ./shared/
COPY server/ ./server/

WORKDIR /src/server
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dleague ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dleague-export ./cmd/dleague-export

# ─── Stage 2: Web client (SvelteKit + adapter-static → client/web/build) ─
FROM node:${NODE_VERSION}-alpine AS webbuild
WORKDIR /src/client/web

COPY client/web/package.json client/web/package-lock.json ./
RUN npm ci

COPY client/web/ ./
RUN npm run build-nolog

# ─── Stage 3: Runtime ─────────────────────────────────────────────────
FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache ca-certificates wget

COPY --from=gobuild /out/dleague          /usr/local/bin/dleague
COPY --from=gobuild /out/dleague-export   /usr/local/bin/dleague-export
COPY --from=webbuild /src/client/web/build /app/web

ENV DLEAGUE_ADDR=:8080 \
    DLEAGUE_WEB=/app/web

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=4 \
  CMD wget -qO- --tries=1 --timeout=2 http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/dleague"]
