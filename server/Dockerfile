# Phase 1 stub — minimal image so `docker compose up dleague` works.
# Phase 11 may refine (multi-arch buildx, layer cache, smaller base, healthcheck).
# Build context: repo root (so we can include the sibling `shared/` module
# referenced by the `replace` directive in `server/go.mod`).
#
# Build arg matches Phase 2's planned downgrade to Go 1.25.5; today's go.mod
# still says 1.26 — that gets reconciled in Phase 2.
ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Bring in only the modules the server needs.
COPY shared/ ./shared/
COPY server/ ./server/

WORKDIR /src/server
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dleague ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/dleague /usr/local/bin/dleague
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dleague"]
