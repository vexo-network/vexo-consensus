# Docker Deployment

This directory contains Docker Compose files for local release-candidate validation.

The files intentionally split initialization from execution:

- `compose.single-host.init.yml`: initialize a 4-validator network in a Docker volume.
- `compose.single-host.yml`: run the 4 validators on one Docker host.
- `compose.multi-host.init.yml`: generate network homes into a bind-mounted directory for distribution.
- `compose.multi-host.yml`: run one validator per host from a copied validator home.

## Build Image

```bash
docker build -t vexo-consensus:dev .
```

Set a different image with:

```bash
export VEXO_IMAGE=ghcr.io/vexo-network/vexo-consensus:0.1.0-rc.1
```

## Single-Host 4-Validator Network

Initialize the network files:

```bash
docker compose -f deployments/docker/compose.single-host.init.yml run --rm init
```

Run the validators:

```bash
docker compose -f deployments/docker/compose.single-host.yml up
```

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
VEXO_NETWORK_DIR=./.vexo-network \
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
export VEXO_PEER_1=validator-2=host-2.example.com:26656
export VEXO_PEER_2=validator-3=host-3.example.com:26656
export VEXO_PEER_3=validator-4=host-4.example.com:26656
docker compose -f deployments/docker/compose.multi-host.yml up
```

For validator 2, set:

```bash
export VEXO_VALIDATOR_ID=validator-2
export VEXO_VALIDATOR_HOME=/srv/vexo/validator-2
export VEXO_PEER_1=validator-1=host-1.example.com:26656
export VEXO_PEER_2=validator-3=host-3.example.com:26656
export VEXO_PEER_3=validator-4=host-4.example.com:26656
```

Repeat the same pattern for validators 3 and 4.

## Notes

- The single-host compose file uses Docker service names for peer dialing.
- The multi-host template expects routable hostnames or IP addresses in `VEXO_PEER_*`.
- The generated network uses Ed25519 key documents and pre-production defaults.
- Do not use these compose files as a public mainnet launch recipe without production config audit, external security review, signer/KMS validation, and long-run evidence.
