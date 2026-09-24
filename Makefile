# edgex-foundry-mcp developer targets.
BINARY  := bin/edgex-foundry-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
COMPOSE := docker compose -f dev/docker-compose.yml

.PHONY: build test lint run dev-up dev-down clean release-snapshot

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/edgex-foundry-mcp

test:
	go test -race ./...

lint:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	golangci-lint run ./...

run: build
	$(BINARY)

dev-up:
	$(COMPOSE) up -d

dev-down:
	$(COMPOSE) down

clean:
	rm -rf bin dist

# Local release dry run (no publishing, no Docker): archives and checksums in dist/.
release-snapshot:
	goreleaser release --snapshot --clean --skip=publish,docker
