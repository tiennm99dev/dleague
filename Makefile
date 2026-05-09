SHELL := /usr/bin/env bash

GO ?= go
BUF ?= buf
GOLANGCI_LINT ?= golangci-lint

.PHONY: help dev dev-debug build build-server web-install web-build web-dev \
        test lint compose-up compose-down proto-gen proto-lint proto-breaking \
        tools clean firebase-emulator seed-wordlists deploy deploy-staging

help:
	@echo "make targets:"
	@echo "  dev              run Go server (hot-reloadable via air or go run)"
	@echo "  dev-debug        run server with -tags debug (protojson logging on)"
	@echo "  build            build server binary"
	@echo "  build-server     build server/cmd/api"
	@echo "  web-install      npm ci in web/ (requires Node 20+)"
	@echo "  web-build        build SvelteKit app → web/dist/"
	@echo "  web-dev          start Vite dev server (:5173, proxies /ws+/health to :8080)"
	@echo "  test             go test ./... in every module"
	@echo "  lint             golangci-lint run on every module"
	@echo "  proto-gen        regenerate shared/pb (Go) + web/src/lib/pb (TS) from proto/"
	@echo "                   NOTE: requires npm install in web/ first"
	@echo "  proto-lint       buf lint"
	@echo "  proto-breaking   buf breaking against main branch"
	@echo "  compose-up / compose-down  docker-compose (MongoDB + mongo-express)"
	@echo "  tools            install buf + protoc-gen-go locally"
	@echo "  firebase-emulator  start Firebase Auth emulator (127.0.0.1:9099)"

dev:
	cd server && $(GO) run ./cmd/api

dev-debug:
	cd server && $(GO) run -tags debug ./cmd/api

build: build-server

build-server:
	cd server && $(GO) build -o ../bin/dleague-server ./cmd/api

web-install:
	cd web && npm ci

web-build:
	cd web && npm run build

web-dev:
	cd web && npm run dev

test:
	cd shared && $(GO) test -race ./...
	cd server && $(GO) test -race ./...

lint:
	cd shared && $(GOLANGCI_LINT) run
	cd server && $(GOLANGCI_LINT) run

compose-up:
	docker compose up -d

compose-down:
	docker compose down

proto-gen:
	cd proto && $(BUF) generate

proto-lint:
	cd proto && $(BUF) lint

proto-breaking:
	cd proto && $(BUF) breaking --against '../.git#branch=main,subdir=proto'

tools:
	$(GO) install github.com/bufbuild/buf/cmd/buf@latest
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest

firebase-emulator:
	bash scripts/start-firebase-emulator.sh

# seed-wordlists uploads the embedded answers.txt and dictionary.txt into the
# Mongo `wordlists` collection so the server uses Mongo rather than the
# embedded fallback. Requires DLEAGUE_MONGO_URI to be set.
# Usage: DLEAGUE_MONGO_URI=mongodb://localhost:27017 make seed-wordlists
seed-wordlists:
	cd server && $(GO) run ./cmd/seed-wordlists

deploy:
	fly deploy --remote-only

deploy-staging:
	fly deploy --remote-only --app dleague-staging

seed-wordlists-prod:
	@echo "Set MONGO_URI to the production Atlas URI first."
	MONGO_URI=$${MONGO_URI} cd server && $(GO) run ./cmd/seed-wordlists

clean:
	rm -rf bin/ web/dist/ web/.svelte-kit/
