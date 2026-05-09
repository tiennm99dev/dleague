SHELL := /usr/bin/env bash

GO ?= go
BUF ?= buf
GOLANGCI_LINT ?= golangci-lint

WEB_OUT := web/main.wasm
WASM_EXEC_SRC := $(shell $(GO) env GOROOT)/lib/wasm/wasm_exec.js
WASM_EXEC_DST := web/wasm_exec.js

.PHONY: help dev dev-debug build build-wasm build-wasm-debug build-server test lint compose-up compose-down proto-gen proto-lint proto-breaking tools clean firebase-emulator

help:
	@echo "make targets:"
	@echo "  dev              run server (prod build) + ensure WASM built"
	@echo "  dev-debug        run server + WASM with -tags debug (protojson logging on)"
	@echo "  build            build server + WASM (prod)"
	@echo "  build-wasm       build client/cmd/web -> $(WEB_OUT)"
	@echo "  build-wasm-debug build client with -tags debug"
	@echo "  build-server     build server/cmd/api"
	@echo "  test             go test ./... in every module"
	@echo "  lint             golangci-lint run on every module"
	@echo "  proto-gen        regenerate shared/pb from proto/"
	@echo "  proto-lint       buf lint"
	@echo "  proto-breaking   buf breaking against main branch"
	@echo "  compose-up / compose-down  docker-compose (MongoDB + mongo-express)"
	@echo "  tools            install buf + protoc-gen-go locally"
	@echo "  firebase-emulator  start Firebase Auth emulator (127.0.0.1:9099)"

dev: build-wasm
	cd server && $(GO) run ./cmd/api

dev-debug: build-wasm-debug
	cd server && $(GO) run -tags debug ./cmd/api

build: build-server build-wasm

build-server:
	cd server && $(GO) build -o ../bin/dleague-server ./cmd/api

build-wasm: $(WASM_EXEC_DST)
	cd client && GOOS=js GOARCH=wasm $(GO) build -o ../$(WEB_OUT) ./cmd/web

build-wasm-debug: $(WASM_EXEC_DST)
	cd client && GOOS=js GOARCH=wasm $(GO) build -tags debug -o ../$(WEB_OUT) ./cmd/web

$(WASM_EXEC_DST):
	cp $(WASM_EXEC_SRC) $(WASM_EXEC_DST)

test:
	cd shared && $(GO) test -race ./...
	cd server && $(GO) test -race ./...
	cd client && $(GO) test -race ./...

lint:
	cd shared && $(GOLANGCI_LINT) run
	cd server && $(GOLANGCI_LINT) run
	cd client && $(GOLANGCI_LINT) run

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

clean:
	rm -f $(WEB_OUT)
	rm -rf bin/
