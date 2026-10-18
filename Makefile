PREFIX ?= $(HOME)/.local
GO ?= go

.PHONY: build install test clean

build:
	mkdir -p dist
	$(GO) build -trimpath -ldflags="-s -w" -o dist/open-mind-tui ./cmd/open-mind-tui

install:
	mkdir -p "$(PREFIX)/bin"
	tmp="$(PREFIX)/bin/.open-mind-tui.tmp"; \
	trap 'rm -f "$$tmp"' EXIT; \
	$(GO) build -trimpath -ldflags="-s -w" -o "$$tmp" ./cmd/open-mind-tui; \
	mv -f "$$tmp" "$(PREFIX)/bin/open-mind-tui"

test:
	$(GO) test ./...

clean:
	rm -rf dist
