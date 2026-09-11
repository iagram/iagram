VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/iagram/iagram/internal/cli.Version=$(VERSION)

.PHONY: all build web go test lint dev clean

all: build

## build: frontend + Go binary with the UI embedded -> ./iagram
build: web go

web:
	cd web && npm ci --silent && npm run build
	@touch internal/web/dist/.gitkeep

go:
	go build -ldflags "$(LDFLAGS)" -o iagram ./cmd/iagram

## test: Go tests + frontend typecheck
test:
	go vet ./...
	go test ./...
	cd web && npm run typecheck

lint:
	gofmt -l . | tee /dev/stderr | test -z "$$(cat)"

## dev: run the API on :7777 and Vite with HMR on :5173 (needs an iagram.json in cwd)
dev:
	go run ./cmd/iagram up --no-open & \
	cd web && npm run dev

clean:
	rm -rf iagram dist internal/web/dist/* web/dist
	@touch internal/web/dist/.gitkeep
