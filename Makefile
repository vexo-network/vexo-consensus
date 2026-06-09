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
IMAGE ?= vexo-consensus
IMAGE_TAG ?= $(VERSION)
GPG ?= gpg
RC_DRY_RUN ?= 0
RC_DRY_RUN_FLAG = $(if $(filter 1 true yes,$(RC_DRY_RUN)),--dry-run,)

.PHONY: all build test vet check docs-check fuzz-smoke ops-verify network-e2e coverage release checksums sbom release-manifest release-audit-pack sign-release docker-image release-candidate release-candidate-real clean

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

check: test vet

docs-check:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./cmd/vexod -run TestDocsLocalesMirrorCanonicalTree -count=1

fuzz-smoke:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./app -run '^$$' -fuzz=FuzzDecodeSignedTx -fuzztime=1s
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./consensus -run '^$$' -fuzz=FuzzDecodeWireMessage -fuzztime=1s
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./dataavailability -run '^$$' -fuzz=FuzzVerify -fuzztime=1s
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./fairordering -run '^$$' -fuzz=FuzzSortTxsWithSalt -fuzztime=1s
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./mempool -run '^$$' -fuzz=FuzzFIFOAddAndBuildBatch -fuzztime=1s
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./rpc -run '^$$' -fuzz=FuzzSubmitTxRequest -fuzztime=1s
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test ./rpc -run '^$$' -fuzz=FuzzSubmitEvidenceRequest -fuzztime=1s

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
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network load --validators 4 --duration 1h --rate 50 --dry-run
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun --validators 4 --duration 1h --rate 50 --output longrun-evidence.json --dry-run

network-e2e: build
	mkdir -p $(GOCACHE_DIR)
	VEXO_NETWORK_E2E=1 VEXO_NETWORK_E2E_BINARY=$$(pwd)/$(BUILD_DIR)/$(BINARY) GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -timeout=30000s ./cmd/vexod -run TestNetworkUpBuiltBinaryE2E -count=1

coverage:
	mkdir -p $(GOCACHE_DIR)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) test -p 1 -timeout=30000s -coverprofile=coverage.out ./...
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
	$(MAKE) sbom
	$(MAKE) release-manifest
	$(MAKE) release-audit-pack

checksums:
	cd $(DIST_DIR) && shasum -a 256 * > checksums.txt

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

sign-release:
	test -f $(DIST_DIR)/checksums.txt
	$(GPG) --batch --yes --armor --detach-sign --output $(DIST_DIR)/checksums.txt.asc $(DIST_DIR)/checksums.txt

docker-image:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE):$(IMAGE_TAG) .

release-candidate: release ops-verify network-e2e
	rm -rf /tmp/vexo-rc-conformance
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod init validator --home /tmp/vexo-rc-conformance --chain-id vexo-rc --validator validator-1 --overwrite
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod ops conformance --home /tmp/vexo-rc-conformance --evm-default-fixtures --json > $(DIST_DIR)/sdk-conformance-evidence.json
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network load --validators 4 --duration 10m --rate 25 $(RC_DRY_RUN_FLAG)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun --validators 4 --duration 10m --rate 25 --output longrun-evidence.json $(RC_DRY_RUN_FLAG)
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network chaos-plan --validators 4 --duration 24h --regions 3
	GOCACHE=$$(pwd)/$(GOCACHE_DIR) $(GO) run ./cmd/vexod network longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4

release-candidate-real:
	$(MAKE) release-candidate RC_DRY_RUN=0

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(GOCACHE_DIR) coverage.out coverage.html
