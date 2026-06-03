# Node Initialization

This guide explains how to initialize validator and archive node homes.

Peer connectivity should be configured in `config.json`, not passed repeatedly on the `start` command line.

## Validator Node

Use `init validator` when the node will propose, vote, sign consensus messages, and participate in validator rotation.

```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1
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
  --bootstrap-peer validator-1=seed-1.example.com:26656
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

Peer and listen addresses live in `config.json`:

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

Do not put long-lived host or `host:port` settings on the `vexod start` command line. Edit `runtime.rpc.address`, `runtime.p2p.listen_address`, `runtime.p2p.peers`, and `runtime.p2p.seeds` in `config.json` instead.

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

For containerized or multi-host networks, put topology values in a JSON file:

```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```

Then generate node homes from that file:

```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
