# Docker Deployment

This directory contains Docker Compose files for release-candidate style validation on Docker.

The files intentionally split initialization from execution:

- `compose.single-host.init.yml`: initialize a 4-validator network in a Docker volume.
- `compose.single-host.yml`: run the 4 validators on one Docker host.
- `compose.multi-host.init.yml`: generate network homes into a bind-mounted directory for distribution.
- `compose.multi-host.yml`: run one validator per host from a copied validator home.
- `compose.*.build.nocgo.yml`: build/run the no-cgo image.
- `compose.*.build.cgo.yml`: build/run the cgo image with `supranational/blst` support.

## Build Image

No-cgo image:

```bash
docker build -f Dockerfile.nocgo -t vexo-consensus:nocgo .
```

cgo image, required for BLS-capable release-style binaries:

```bash
docker build -f Dockerfile.cgo -t vexo-consensus:cgo .
```

Set a prebuilt image with:

```bash
export VEXO_IMAGE=ghcr.io/vexo-network/vexo-consensus:0.1.0-rc.1
```

## Single-Host 4-Validator Network

Initialize the network files:

```bash
docker compose -f deployments/docker/compose.single-host.init.yml run --rm init
```

Build and initialize with the no-cgo image:

```bash
docker compose \
  -f deployments/docker/compose.single-host.init.yml \
  -f deployments/docker/compose.single-host.init.build.nocgo.yml \
  run --rm init
```

Build and initialize with the cgo image:

```bash
docker compose \
  -f deployments/docker/compose.single-host.init.yml \
  -f deployments/docker/compose.single-host.init.build.cgo.yml \
  run --rm init
```

Run the validators:

```bash
docker compose -f deployments/docker/compose.single-host.yml up
```

Build and run validators with no-cgo:

```bash
docker compose \
  -f deployments/docker/compose.single-host.yml \
  -f deployments/docker/compose.single-host.build.nocgo.yml \
  up --build
```

Build and run validators with cgo:

```bash
docker compose \
  -f deployments/docker/compose.single-host.yml \
  -f deployments/docker/compose.single-host.build.cgo.yml \
  up --build
```

The init compose writes peer, listen, and advertised addresses into each validator home's split config files. The run compose does not pass `--peer`, `--seed`, `--rpc-address`, or `--p2p-listen` flags.

Single-host listen and peer hosts are defined in `topology.single-host.json`; edit that file before initialization instead of adding host flags to compose commands. Runtime peer lists live in each node's `network_config.json`.

Query validator RPC endpoints from the host:

```bash
curl http://127.0.0.1:28657/v1/status
curl http://127.0.0.1:28667/v1/status
curl http://127.0.0.1:28677/v1/status
curl http://127.0.0.1:28687/v1/status
```

Stop the network:

```bash
docker compose -f deployments/docker/compose.single-host.yml down
```

Remove the generated Docker volume:

```bash
docker volume rm vexo-single-data
```

## Multi-Host 4-Validator Template

Generate all validator homes once on a trusted machine:

```bash
docker compose -f deployments/docker/compose.multi-host.init.yml run --rm init
```

Before running init, edit `deployments/docker/topology.multi-host.json` so `p2p_host_template` and `rpc_host_template` match dialable hostnames for the machines that will run the validators. `p2p_advertise_host_template` and `rpc_advertise_host_template` should be stable public DNS names or public IP addresses if external peers need to discover the validators. To use a different topology file:

```bash
VEXO_TOPOLOGY_CONFIG="$PWD/my-topology.json" \
docker compose -f deployments/docker/compose.multi-host.init.yml run --rm init
```

Distribute each directory to its host:

```text
.vexo-network/validator-1 -> host-1:/srv/vexo/validator-1
.vexo-network/validator-2 -> host-2:/srv/vexo/validator-2
.vexo-network/validator-3 -> host-3:/srv/vexo/validator-3
.vexo-network/validator-4 -> host-4:/srv/vexo/validator-4
```

Run one validator per host. Example for validator 1:

```bash
export VEXO_VALIDATOR_ID=validator-1
export VEXO_VALIDATOR_HOME=/srv/vexo/validator-1
docker compose -f deployments/docker/compose.multi-host.yml up
```

Build and run the multi-host validator image locally with cgo:

```bash
docker compose \
  -f deployments/docker/compose.multi-host.yml \
  -f deployments/docker/compose.multi-host.build.cgo.yml \
  up --build
```

For validator 2, set:

```bash
export VEXO_VALIDATOR_ID=validator-2
export VEXO_VALIDATOR_HOME=/srv/vexo/validator-2
```

Repeat the same pattern for validators 3 and 4.

## Notes

- The single-host compose file uses Docker service names for peer dialing.
- The multi-host template writes dial targets into `network_config.json` and advertised validator metadata into `genesis.json` during init from `topology.multi-host.json`.
- Use `Dockerfile.cgo` for BLS-capable validator binaries. Use `Dockerfile.nocgo` only for portability smoke, private development, or networks that do not claim cgo-backed BLS release capability.
- The generated network uses Ed25519 key documents and conservative example defaults. Audit and tune the generated `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before relying on it.
- Do not use these compose files as a public launch recipe without strict config audit, external security review, signer/KMS validation, and long-run evidence.
