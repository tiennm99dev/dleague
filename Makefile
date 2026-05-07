SHELL := /usr/bin/env bash

GO ?= go
BUF ?= buf
GOLANGCI_LINT ?= golangci-lint
NPM ?= npm

WEB_DIR := client/web

IMAGE ?= dleague-server
IMAGE_TAG ?= dev
IMAGE_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help dev dev-debug build build-server web-dev web-build web-install test lint atlas-smoke compose-up compose-down image image-load image-push proto-gen proto-lint proto-breaking tools clean grep-isolation

help:
	@echo "make targets:"
	@echo "  dev              run Go server (prod build)"
	@echo "  dev-debug        run server with -tags debug (protojson logging on)"
	@echo "  build            build server + web client"
	@echo "  build-server     build server/cmd/api"
	@echo "  web-install      cd $(WEB_DIR) && npm install"
	@echo "  web-dev          run Vite dev server (Svelte+Phaser)"
	@echo "  web-build        production build of the web client"
	@echo "  test             go test ./... across server + shared"
	@echo "  lint             golangci-lint run on server + shared modules"
	@echo "  atlas-smoke      run server/cmd/atlas-smoke against MONGODB_URI"
	@echo "  grep-isolation   verify mongo-driver imports stay in internal/store/mongodb"
	@echo "  compose-up       docker compose up -d (dleague server only; data plane is Atlas)"
	@echo "  compose-down     docker compose down"
	@echo "  image            buildx multi-arch image (\$(IMAGE_PLATFORMS)); does not load"
	@echo "  image-load       buildx single-arch (host) image, load into local docker"
	@echo "  image-push       buildx multi-arch image, push to registry (set IMAGE=<registry>/<repo>)"
	@echo "  proto-gen        regenerate shared/pb from proto/"
	@echo "  proto-lint       buf lint"
	@echo "  proto-breaking   buf breaking against main branch"
	@echo "  tools            install buf + protoc-gen-go locally"

dev:
	cd server && $(GO) run ./cmd/api

dev-debug:
	cd server && $(GO) run -tags debug ./cmd/api

build: build-server web-build

build-server:
	cd server && $(GO) build -o ../bin/dleague-server ./cmd/api

web-install:
	cd $(WEB_DIR) && $(NPM) install

web-dev:
	cd $(WEB_DIR) && $(NPM) run dev

web-build:
	cd $(WEB_DIR) && $(NPM) run build-nolog

test:
	cd shared && $(GO) test ./...
	cd server && $(GO) test ./...

lint:
	cd shared && $(GOLANGCI_LINT) run
	cd server && $(GOLANGCI_LINT) run

atlas-smoke:
	cd server && $(GO) run ./cmd/atlas-smoke

# Verifies the mongo-driver import stays inside internal/store/mongodb.
# The seam is the migration safety net; this fails CI if anything leaks.
grep-isolation:
	@if grep -rl '"go.mongodb.org/mongo-driver/v2' server/ \
	  | grep -v 'internal/store/mongodb' >/dev/null; then \
	  echo "leak: mongo-driver imported outside internal/store/mongodb"; \
	  grep -rl '"go.mongodb.org/mongo-driver/v2' server/ | grep -v 'internal/store/mongodb'; \
	  exit 1; \
	fi
	@echo "grep-isolation: ok"

compose-up:
	docker compose up -d

compose-down:
	docker compose down

# Multi-arch buildx target. No --load (buildx can't load multi-platform into
# the local docker daemon); use `image-load` for a single-arch local image.
image:
	docker buildx build \
	  --platform $(IMAGE_PLATFORMS) \
	  -f server/Dockerfile \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  .

image-load:
	docker buildx build \
	  --load \
	  -f server/Dockerfile \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  .

image-push:
	docker buildx build \
	  --platform $(IMAGE_PLATFORMS) \
	  --push \
	  -f server/Dockerfile \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  .

proto-gen:
	cd proto && $(BUF) generate

proto-lint:
	cd proto && $(BUF) lint

proto-breaking:
	cd proto && $(BUF) breaking --against '../.git#branch=main,subdir=proto'

tools:
	$(GO) install github.com/bufbuild/buf/cmd/buf@latest
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest

clean:
	rm -rf bin/
	rm -rf $(WEB_DIR)/build $(WEB_DIR)/.svelte-kit
