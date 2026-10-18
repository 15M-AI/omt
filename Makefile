PREFIX ?= $(HOME)/.local
GO ?= go

.PHONY: build install test clean

build:
	mkdir -p dist
	$(GO) build -trimpath -ldflags="-s -w" -o dist/omt ./cmd/omt

install:
	mkdir -p "$(PREFIX)/bin"
	tmp="$(PREFIX)/bin/.omt.tmp"; \
	trap 'rm -f "$$tmp"' EXIT; \
	$(GO) build -trimpath -ldflags="-s -w" -o "$$tmp" ./cmd/omt; \
	mv -f "$$tmp" "$(PREFIX)/bin/omt"

test:
	$(GO) test ./...

clean:
	rm -rf dist
