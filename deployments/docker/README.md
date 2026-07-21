# Docker Deployment

This directory contains Docker Compose files for running Vexo nodes with Docker. Start here if you want a real multi-validator process layout without manually editing every node home.

The files intentionally split initialization from execution:

- `compose.single-host.init.yml`: initialize a 4-validator network in a Docker volume.
- `compose.single-host.yml`: run the 4 validators on one Docker host.
- `compose.multi-host.init.yml`: generate network homes into a bind-mounted directory for distribution.
- `compose.multi-host.yml`: run one validator per host from a copied validator home.
- `compose.*.build.nocgo.yml`: build/run the no-cgo image.
- `compose.*.build.cgo.yml`: build/run the cgo image with `supranational/blst` support.

## Choose the Right File Set

| Goal | Compose files |
|---|---|
| Initialize single-host network with prebuilt image | `compose.single-host.init.yml` |
| Initialize single-host network and build no-cgo image | `compose.single-host.init.yml` + `compose.single-host.init.build.nocgo.yml` |
| Initialize single-host network and build cgo/BLS image | `compose.single-host.init.yml` + `compose.single-host.init.build.cgo.yml` |
| Run single-host network with prebuilt image | `compose.single-host.yml` |
| Run single-host network and build no-cgo image | `compose.single-host.yml` + `compose.single-host.build.nocgo.yml` |
| Run single-host network and build cgo/BLS image | `compose.single-host.yml` + `compose.single-host.build.cgo.yml` |
| Generate multi-host node homes | `compose.multi-host.init.yml` |
| Run one validator on one host | `compose.multi-host.yml` plus optional build override |

Use the cgo image when the network claims built-in `supranational/blst` BLS support. Use the no-cgo image for portable smoke tests, constrained environments, or networks that intentionally do not claim cgo-backed BLS release capability.

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

This is the easiest way to see four validators connect, gossip, and commit blocks on one machine. It uses Docker service names inside the network, but exposes each validator RPC port to the host.

Initialize the network files:

```bash
export VEXO_KEY_PASSPHRASE='change-me'

docker compose -f deployments/docker/compose.single-host.init.yml run --build --rm init
```

Build and initialize with the no-cgo image:

```bash
docker compose \
  -f deployments/docker/compose.single-host.init.yml \
  -f deployments/docker/compose.single-host.init.build.nocgo.yml \
  run --build --rm init
```

Build and initialize with the cgo image:

```bash
docker compose \
  -f deployments/docker/compose.single-host.init.yml \
  -f deployments/docker/compose.single-host.init.build.cgo.yml \
  run --build --rm init
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
curl -s http://127.0.0.1:28657/v1/status
curl -s http://127.0.0.1:28667/v1/status
curl -s http://127.0.0.1:28677/v1/status
curl -s http://127.0.0.1:28687/v1/status
```

Check Web3/Remix JSON-RPC on validator 1:

```bash
curl -s http://127.0.0.1:28657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

For Remix, use:

```text
http://127.0.0.1:28657/web3
```

The single-host init flow now also seeds a local Web3 managed account for each validator, so Remix contract deployment can use `eth_sendTransaction` without extra wallet setup. That account is intended for local development only.

Do not use `http://127.0.0.1:26657/web3` from the host for the single-host compose network. `26657` is the container-internal RPC port; `28657` is the host port for validator 1.

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
export VEXO_KEY_PASSPHRASE='change-me'

docker compose -f deployments/docker/compose.multi-host.init.yml run --build --rm init
```

Before running init, edit `deployments/docker/topology.multi-host.json` so `p2p_host_template` and `rpc_host_template` match dialable hostnames for the machines that will run the validators. `p2p_advertise_host_template` and `rpc_advertise_host_template` should be stable public DNS names or public IP addresses if external peers need to discover the validators. To use a different topology file:

```bash
VEXO_TOPOLOGY_CONFIG="$PWD/my-topology.json" \
docker compose -f deployments/docker/compose.multi-host.init.yml run --build --rm init
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

After a host starts, verify its local RPC:

```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
```

Then check another host can dial the advertised P2P address from `network_config.json:p2p.peers`. If the generated peer map contains Docker-only names such as `validator-1`, regenerate with a multi-host topology file that uses real DNS names or IP addresses.

## What the Init Step Writes

The init container generates a separate home for each validator:

```text
.vexo-network/
  validator-1/
    config.json
    module_config.json
    network_config.json
    consensus_config.json
    mempool_config.json
    log_config.json
    genesis.json
    validator.key.json
    node.key.json
    validator.vrf.key.json
    data/
  validator-2/
  validator-3/
  validator-4/
```

Runtime behavior comes from those files. The run compose file does not pass peer addresses or listen hosts as command flags. To change RPC or P2P addresses, edit the topology JSON before init or edit the generated `network_config.json` files before starting.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| No more logs after startup | Normal if only startup events are emitted or logs are waiting for block commits | Query `/v1/status`; if `latest_height` increases, the network is healthy |
| `latest_height` is stuck | Empty mempool with empty blocks disabled, validator quorum missing, or peers not connected | Check `peer_count`, validator logs, and `consensus_config.json:create_empty_blocks` |
| Remix says `Failed to fetch eth_chainId` | Wrong URL or host port | Use `http://127.0.0.1:28657/web3` for single-host validator 1 |
| Peer count is lower than expected | Wrong peer hostnames, Docker network mismatch, or firewall | Check generated `network_config.json:p2p.peers` |
| BLS build fails in CI | cgo cross-compilation attempted without a matching C toolchain | Use `Dockerfile.cgo` on a cgo-capable runner or build no-cgo only for portability smoke |

## Notes

- The single-host compose file uses Docker service names for peer dialing.
- The multi-host template writes dial targets into `network_config.json` and advertised validator metadata into `genesis.json` during init from `topology.multi-host.json`.
- Use `Dockerfile.cgo` for BLS-capable validator binaries. Use `Dockerfile.nocgo` only for portability smoke, private development, or networks that do not claim cgo-backed BLS release capability.
- The generated network uses Ed25519 key documents and conservative example defaults. Audit and tune the generated `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before relying on it.
- Do not use these compose files as a public launch recipe without strict config audit, external security review, signer/KMS validation, and long-run evidence.
