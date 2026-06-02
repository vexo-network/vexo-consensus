GO ?= go
BINARY ?= vexod
BUILD_DIR ?= bin
GOFLAGS ?=
GOCACHE_DIR ?= .gocache

.PHONY: all build test vet check clean init-demo keys-demo

all: check build

build:
	mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/vexod

test:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -timeout=30000s ./...

vet:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) vet ./...

check: test vet

clean:
	rm -rf $(BUILD_DIR) $(GOCACHE_DIR) coverage.out coverage.html

init-demo: build
	$(BUILD_DIR)/$(BINARY) init --home .vexo --chain-id vexo-local --validator validator-1 --overwrite
	$(BUILD_DIR)/$(BINARY) validate --home .vexo

keys-demo: build
	$(BUILD_DIR)/$(BINARY) keys gen --home .vexo --overwrite
	$(BUILD_DIR)/$(BINARY) keys show --home .vexo --json
