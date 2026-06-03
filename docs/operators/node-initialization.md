# Node Initialization

This guide explains how to initialize validator and archive node homes.

Peer connectivity should be configured in `config.json`, not passed repeatedly on the `start` command line.

## Validator Node

Use `init validator` when the node will propose, vote, sign consensus messages, and participate in validator rotation.

```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --p2p-listen 0.0.0.0:26656 \
  --rpc-address 0.0.0.0:26657
```

This creates:

- `config.json`
- `genesis.json`
- `validator.key.json`
- `data/`

Start it with config-driven networking:

```bash
vexod start --home .vexo-validator-1 --run
```

## Archive Node

Use `init archive` when the node should keep chain data, expose RPC, sync from peers, and avoid validator signing.

```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656 \
  --p2p-listen 0.0.0.0:26656 \
  --rpc-address 0.0.0.0:26657
```

This creates:

- `config.json`
- `genesis.json`
- `data/`

It does **not** create `validator.key.json`.

Start it with:

```bash
vexod start --home .vexo-archive-1 --run
```

Archive config has:

```json
{
  "node_mode": "archive",
  "validator_id": "",
  "runtime": {
    "consensus": {
      "loop_enabled": false
    }
  }
}
```

## Config-Based Peers

Peer addresses live in `config.json`:

```json
{
  "runtime": {
    "p2p": {
      "enabled": true,
      "listen_address": "0.0.0.0:26656",
      "peers": {
        "validator-1": "seed-1.example.com:26656",
        "validator-2": "seed-2.example.com:26656"
      },
      "seeds": {
        "seed-1": "seed-1.example.com:26656"
      }
    }
  }
}
```

`vexod start` loads these peers automatically:

```bash
vexod start --home .vexo-archive-1 --run
```

Command-line `--peer` and `--seed` remain available for temporary debugging, but production homes should store persistent peers in `config.json`.

## Multi-Validator Local Network

For a generated local network:

```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```

Each generated validator home receives:

- its own `validator.key.json`
- a shared `genesis.json`
- config-level peer entries for the other validators

For containerized single-host networks, generate service-name peers and same internal ports:

```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --p2p-port-step 0 \
  --rpc-port-step 0 \
  --p2p-host-template 'validator-%d' \
  --rpc-host-template 'validator-%d' \
  --p2p-listen-host 0.0.0.0 \
  --rpc-listen-host 0.0.0.0
```
