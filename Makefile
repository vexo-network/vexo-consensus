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
RELEASE_CGO_ENABLED ?= 0
RELEASE_REQUIRE_BLS ?= 1
DEFAULT_RELEASE_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
HOST_RELEASE_TARGET := $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)
ifeq ($(RELEASE_CGO_ENABLED),1)
RELEASE_TARGETS ?= $(HOST_RELEASE_TARGET)
else
RELEASE_TARGETS ?= $(DEFAULT_RELEASE_TARGETS)
endif
IMAGE ?= vexo-consensus
IMAGE_TAG ?= $(VERSION)
GPG ?= gpg
RC_LOAD_DURATION ?= 1h
RC_LONGRUN_DURATION ?= 168h
RC_CHAOS_DURATION ?= 24h
RC_CHAOS_TIMEOUT ?= 60s
RC_RATE ?= 50
RC_EVM_CONFORMANCE_FLAGS ?=
FUZZ_PARALLEL ?= 1
NETWORK_E2E_GO_TIMEOUT ?= 120000s

.PHONY: all build test vet race check docs-check evm-conformance fuzz-smoke ops-verify network-e2e coverage release release-preflight release-portable checksums sbom release-manifest release-audit-pack release-evidence-manifest sign-release docker-image release-candidate release-candidate-real release-candidate-plan clean

all: check build

build:
	mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/vexod

test:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -p 1 -timeout=30000s ./...

vet:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) vet ./...

race:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -race -p 1 -timeout=30000s ./...

check: test vet docs-check evm-conformance

docs-check:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./cmd/vexod -run TestDocsLocalesMirrorCanonicalTree -count=1
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod release docs-quality --docs docs --json > /tmp/vexo-docs-quality.json

evm-conformance:
	mkdir -p $(GOCACHE_DIR)
	rm -rf /tmp/vexo-evm-conformance
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod init validator --home /tmp/vexo-evm-conformance --chain-id vexo-evm-conformance --validator validator-1 --overwrite
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod ops conformance --home /tmp/vexo-evm-conformance --evm-default-fixtures --json > /tmp/vexo-evm-conformance.json

fuzz-smoke:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./app -run '^$$' -fuzz=FuzzDecodeSignedTx -fuzztime=1s -parallel=$(FUZZ_PARALLEL)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./consensus -run '^$$' -fuzz=FuzzDecodeWireMessage -fuzztime=1s -parallel=$(FUZZ_PARALLEL)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./dataavailability -run '^$$' -fuzz=FuzzVerify -fuzztime=1s -parallel=$(FUZZ_PARALLEL)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./fairordering -run '^$$' -fuzz=FuzzSortTxsWithSalt -fuzztime=1s -parallel=$(FUZZ_PARALLEL)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./mempool -run '^$$' -fuzz=FuzzFIFOAddAndBuildBatch -fuzztime=1s -parallel=$(FUZZ_PARALLEL)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./rpc -run '^$$' -fuzz=FuzzSubmitTxRequest -fuzztime=1s -parallel=$(FUZZ_PARALLEL)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./rpc -run '^$$' -fuzz=FuzzSubmitEvidenceRequest -fuzztime=1s -parallel=$(FUZZ_PARALLEL)

ops-verify: check fuzz-smoke
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod config audit-pack --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod config deployment-template --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod config tune --validators 64 --tps 5000 --regions 4 --latency 120ms --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod release launch-checklist --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod release readiness --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod consensus adversarial --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod upgrade plan --json --name ops-drill --height 100 --rollback-binary previous > /tmp/vexo-upgrade-plan.json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod upgrade rollback-plan --plan-file /tmp/vexo-upgrade-plan.json --last-safe-height 99 --snapshot /tmp/vexo-snapshot.json --json
	printf '{"schema_version":"v1","chain_id":"vexo-chain","state":{"Height":10,"AppHash":[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"LastBlockHash":[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"ValidatorSetHash":[3,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]},"state_roots":[{"Height":10,"Namespace":"bank","Root":[4,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}],"modules":["bank"],"kv":[{"Namespace":"bank","Key":"YQ==","Value":"Yg=="}]}' > /tmp/vexo-snapshot-drill.json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod snapshot drill-plan --input /tmp/vexo-snapshot-drill.json --chain-id vexo-chain --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod slashing lifecycle-plan --type conflicting_vote --validator validator-1 --height 1 --current-height 101 --json
	printf '{"latest_height":10,"round_timeouts":0,"snapshot_healthy":true,"replay_healthy":true}' > /tmp/vexo-metrics-prev.json
	printf '{"latest_height":15,"round_timeouts":0,"snapshot_healthy":true,"replay_healthy":true}' > /tmp/vexo-metrics-current.json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod ops incident --metrics-file /tmp/vexo-metrics-current.json --previous-metrics-file /tmp/vexo-metrics-prev.json --window 1m --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network scale-plan --validators 64 --regions 4 --hosts 8 --duration 24h --rate 100 --json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network chaos-plan --validators 4 --duration 24h --regions 3
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network load --validators 4 --duration 1h --rate 50 --dry-run > /tmp/vexo-network-load-plan.txt
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun --validators 4 --duration 1h --rate 50 --dry-run > /tmp/vexo-longrun-plan.txt

network-e2e: build
	mkdir -p $(GOCACHE_DIR)
	VEXO_NETWORK_E2E=1 VEXO_NETWORK_E2E_BINARY=$$(pwd)/$(BUILD_DIR)/$(BINARY) GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -timeout=$(NETWORK_E2E_GO_TIMEOUT) ./cmd/vexod -run TestNetworkUpBuiltBinaryE2E -count=1

coverage:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -p 1 -timeout=30000s -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

release-preflight:
	@if [ "$(RELEASE_REQUIRE_BLS)" = "1" ] && [ "$(RELEASE_CGO_ENABLED)" != "1" ]; then \
		echo "release requires RELEASE_CGO_ENABLED=1 because network-safe BLS uses the cgo-backed supranational/blst adapter"; \
		echo "for non-BLS smoke artifacts only, run: make release-portable RELEASE_REQUIRE_BLS=0"; \
		exit 1; \
	fi

release: check release-preflight
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	@for target in $(RELEASE_TARGETS); do \
		goos=$${target%/*}; goarch=$${target#*/}; \
		out="$(DIST_DIR)/$(BINARY)-$(VERSION)-$${goos}-$${goarch}"; \
		if [ "$$goos" = "windows" ]; then out="$$out.exe"; fi; \
		echo "building $$out"; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=$(RELEASE_CGO_ENABLED) $(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o "$$out" ./cmd/vexod; \
	done
	$(MAKE) checksums
	$(MAKE) sbom
	$(MAKE) release-manifest
	$(MAKE) release-audit-pack

checksums:
	cd $(DIST_DIR) && LC_ALL=C LANG=C shasum -a 256 * > checksums.txt

sbom:
	mkdir -p $(DIST_DIR)
	$(GO) version > $(DIST_DIR)/sbom-go-version.txt
	@for artifact in $(DIST_DIR)/$(BINARY)-$(VERSION)-*; do \
		if [ -f "$$artifact" ]; then $(GO) version -m "$$artifact" >> $(DIST_DIR)/sbom-go-version.txt 2>/dev/null || true; fi; \
	done
	$(GO) list -m -json all > $(DIST_DIR)/sbom-go-modules.json

release-manifest:
	mkdir -p $(DIST_DIR)
	printf '{\n  "version": "$(VERSION)",\n  "commit": "$(COMMIT)",\n  "build_date": "$(BUILD_DATE)",\n  "binary": "$(BINARY)",\n  "targets": "$(RELEASE_TARGETS)",\n  "checksums": "checksums.txt",\n  "sbom": "sbom-go-modules.json"\n}\n' > $(DIST_DIR)/release-manifest.json

release-audit-pack:
	mkdir -p $(DIST_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod release pack --dist $(DIST_DIR) --version $(VERSION) --output $(DIST_DIR)/release-audit-pack.json

release-evidence-manifest:
	mkdir -p $(DIST_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod release evidence-manifest --dist $(DIST_DIR) --output $(DIST_DIR)/evidence-manifest.json

sign-release:
	test -f $(DIST_DIR)/checksums.txt
	$(GPG) --batch --yes --armor --detach-sign --output $(DIST_DIR)/checksums.txt.asc $(DIST_DIR)/checksums.txt

docker-image:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE):$(IMAGE_TAG) .

release-portable: RELEASE_REQUIRE_BLS=0
release-portable: release

release-candidate: release-candidate-real

release-candidate-real: release ops-verify network-e2e
	@test -n "$(RC_EVM_CONFORMANCE_FLAGS)" || { echo "release-candidate-real requires RC_EVM_CONFORMANCE_FLAGS with externally pinned EVM/Web3 fixture corpora"; exit 1; }
	rm -rf /tmp/vexo-rc-conformance
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod init validator --home /tmp/vexo-rc-conformance --chain-id vexo-rc --validator validator-1 --overwrite
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod ops conformance --home /tmp/vexo-rc-conformance --strict $(RC_EVM_CONFORMANCE_FLAGS) --json > $(DIST_DIR)/sdk-conformance-evidence.json
	@set -e; \
		stop_network() { GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network stop --validators 4 || true; }; \
		trap stop_network EXIT; \
		GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network up --validators 4 --timeout 30s --overwrite --keep-running; \
		GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network load --validators 4 --duration $(RC_LOAD_DURATION) --rate $(RC_RATE); \
		GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun --validators 4 --duration $(RC_LONGRUN_DURATION) --rate $(RC_RATE) --output $(DIST_DIR)/longrun-evidence.json; \
		GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network chaos --validators 4 --timeout $(RC_CHAOS_TIMEOUT)
	$(MAKE) release-evidence-manifest

release-candidate-plan: RELEASE_REQUIRE_BLS=0
release-candidate-plan: release ops-verify
	rm -rf /tmp/vexo-rc-conformance
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod init validator --home /tmp/vexo-rc-conformance --chain-id vexo-rc --validator validator-1 --overwrite
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod ops conformance --home /tmp/vexo-rc-conformance --evm-default-fixtures --json > $(DIST_DIR)/sdk-conformance-plan.json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network load --validators 4 --duration 10m --rate 25 --dry-run > $(DIST_DIR)/network-load-plan.txt
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun --validators 4 --duration 10m --rate 25 --dry-run > $(DIST_DIR)/longrun-plan.txt
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network chaos-plan --validators 4 --duration $(RC_CHAOS_DURATION) --regions 3 > $(DIST_DIR)/chaos-plan.txt
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun-plan --validators 4 --duration $(RC_LONGRUN_DURATION) --regions 3 --hosts 4 > $(DIST_DIR)/longrun-topology-plan.txt
	$(MAKE) release-evidence-manifest

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(GOCACHE_DIR) coverage.out coverage.html
