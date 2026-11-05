export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod

.PHONY: build test check package
build:
	mkdir -p bin
	go build -trimpath -o bin/agent-linux ./cmd/agent-linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/agent-linux-guest ./cmd/agent-linux-guest

test:
	go test -race ./...
	python3 -m unittest discover -s scripts -p test_install.py

check:
	go vet ./...
	git diff --check

# Local only: dirty trees require explicit direct script invocation with --allow-dirty.
package:
	python3 scripts/package.py --output dist/agent-linux-0.5.0-rc.1
