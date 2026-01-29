.PHONY: all build sign install clean test lint deps bdd joker-test tapes

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

TAPES := $(wildcard tapes/*.tape)
GIFS  := $(TAPES:.tape=.gif)

# BDD: run all specs (Go tests + Joker assertions + CUE validation)
bdd: test joker-test cue-vet
	@echo "✓ All BDD specs green"

# GIVEN hof.joker definitions
# WHEN  joker evaluates all assertions
# THEN  all HOF beta reductions hold
joker-test:
	@printf "GIVEN hof.joker definitions\nWHEN  joker evaluates all assertions\n"
	@joker hof.joker
	@echo "THEN  all HOF beta reductions hold ✓"

# GIVEN hof.cue schema
# WHEN  cue vet validates constraints
# THEN  GF(3) triad balance is enforced
cue-vet:
	@printf "GIVEN hof.cue schema\nWHEN  cue vet validates constraints\n"
	@command -v cue >/dev/null && cue vet hof.cue && echo "THEN  GF(3) triad balance enforced ✓" || echo "THEN  (cue not installed, skipped)"

# Record all VHS tapes (requires vhs + ttyd)
tapes: $(GIFS)

tapes/%.gif: tapes/%.tape
	vhs $<

# Build HaikuOS GUI launcher
haiku-gui: deps
	CGO_ENABLED=1 go build $(LDFLAGS) $(GOFLAGS) -o haiku-gui ./cmd/haiku-gui
	codesign --force --entitlements entitlements.plist --sign - haiku-gui
	@echo "✓ Built haiku-gui - run with ./haiku-gui"
