export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod

.PHONY: build test check
build:
	mkdir -p bin
	go build -trimpath -o bin/agent-linux ./cmd/agent-linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/agent-linux-guest ./cmd/agent-linux-guest

test:
	go test -race ./...

check:
	go vet ./...
	git diff --check
