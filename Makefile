GO ?= go
BINARY ?= vexod
BUILD_DIR ?= bin
DIST_DIR ?= dist
GOFLAGS ?=
GOCACHE_DIR ?= .gocache
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS ?= -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)
RELEASE_TARGETS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build test vet check coverage release checksums clean init-demo keys-demo

all: check build

build:
	mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/vexod

test:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -timeout=30000s ./...

vet:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) vet ./...

check: test vet

coverage:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -timeout=30000s -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

release: check
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	@for target in $(RELEASE_TARGETS); do \
		goos=$${target%/*}; goarch=$${target#*/}; \
		out="$(DIST_DIR)/$(BINARY)-$(VERSION)-$${goos}-$${goarch}"; \
		if [ "$$goos" = "windows" ]; then out="$$out.exe"; fi; \
		echo "building $$out"; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=0 $(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o "$$out" ./cmd/vexod; \
	done
	$(MAKE) checksums

checksums:
	cd $(DIST_DIR) && shasum -a 256 * > checksums.txt

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(GOCACHE_DIR) coverage.out coverage.html

init-demo: build
	$(BUILD_DIR)/$(BINARY) init --home .vexo --chain-id vexo-local --validator validator-1 --overwrite
	$(BUILD_DIR)/$(BINARY) validate --home .vexo

keys-demo: build
	$(BUILD_DIR)/$(BINARY) keys gen --home .vexo --overwrite
	$(BUILD_DIR)/$(BINARY) keys show --home .vexo --json
