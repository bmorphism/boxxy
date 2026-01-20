.PHONY: all build sign install clean test lint deps

BINARY := boxxy
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
GOFLAGS := -tags=darwin

all: deps build sign

deps:
	go mod download
	go mod tidy

build:
	CGO_ENABLED=1 go build $(LDFLAGS) $(GOFLAGS) -o $(BINARY) ./cmd/boxxy

sign: build
	@echo "Signing binary with virtualization entitlements..."
	codesign --force --entitlements entitlements.plist --sign - $(BINARY)
	@echo "✓ Signed $(BINARY)"

install: sign
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY) $(HOME)/.local/bin/
	@echo "✓ Installed to $(HOME)/.local/bin/$(BINARY)"

clean:
	rm -f $(BINARY)
	go clean -cache

test:
	go test -v ./...

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed"

# Generate std namespace bindings
generate:
	cd std && go generate ./...

# Create a release build
release:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) $(GOFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/boxxy
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) $(GOFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/boxxy
	@echo "Built release binaries"

# Run the REPL
repl: sign
	./$(BINARY) repl

# Run an example
example: sign
	./$(BINARY) examples/haiku-vm.joke

# Build HaikuOS GUI launcher
haiku-gui: deps
	CGO_ENABLED=1 go build $(LDFLAGS) $(GOFLAGS) -o haiku-gui ./cmd/haiku-gui
	codesign --force --entitlements entitlements.plist --sign - haiku-gui
	@echo "✓ Built haiku-gui - run with ./haiku-gui"
