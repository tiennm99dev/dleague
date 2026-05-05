SHELL := /usr/bin/env bash

GO ?= go
BUF ?= buf
GOLANGCI_LINT ?= golangci-lint
NPM ?= npm

WEB_DIR := client/web

.PHONY: help dev dev-debug build build-server web-dev web-build web-install test lint db-up db-down compose-up compose-down proto-gen proto-lint proto-breaking tools clean

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
	@echo "  compose-up       docker compose up -d (couchbase + redis + dleague)"
	@echo "  compose-down     docker compose down"
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

compose-up:
	docker compose up -d couchbase redis

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

clean:
	rm -rf bin/
	rm -rf $(WEB_DIR)/build $(WEB_DIR)/.svelte-kit
