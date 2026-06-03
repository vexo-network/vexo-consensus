# Node Initialization

This guide explains how to initialize validator and archive node homes.

Peer connectivity should be configured in `network_config.json`, not passed repeatedly on the `start` command line.

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
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
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
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `data/`

It does **not** create `validator.key.json`.

Start it with:

```bash
vexod start --home .vexo-archive-1 --run
```

## Split Configuration Files

Node homes use separate config files so operators can edit one subsystem without mixing unrelated settings:

- `config.json` contains node identity, chain ID, data path, and pointers to the split config files.
- `module_config.json` contains application module selection, execution/ante policy, and module-level governance policy.
- `network_config.json` contains RPC, P2P listen/peer/seed settings, and peer-scoring policy.
- `consensus_config.json` contains consensus loop timing, empty-block policy, crypto backend, VRF, validator admission, and committee policy.
- `mempool_config.json` contains mempool size, fee, priority, WAL, duplicate, and TTL policy.
- `log_config.json` contains log format, level, block commit event logging, and peer event logging.
- `genesis.json` contains immutable genesis validators, validator metadata, and genesis module state.

`config.json` points to the split config files:

```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```

Each path may be absolute or relative to the node home. If omitted, `vexod` uses the default `<home>/<name>_config.json` file.

Example `module_config.json`:

```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```

Example `network_config.json`:

```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```

Example `consensus_config.json`:

```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false
  }
}
```

Use `vexod config paths --home <home>` to inspect all resolved paths.

Archive config has:

```json
{
  "schema_version": "v1",
  "node_mode": "archive",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```

Archive `consensus_config.json` disables the local consensus loop:

```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```

## Config-Based Peers

Peer and listen addresses live in `network_config.json`:

```json
{
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
```

`vexod start` loads these peers automatically:

```bash
vexod start --home .vexo-archive-1 --run
```

Command-line `--peer` and `--seed` remain available for temporary debugging, but production homes should store persistent peers in `network_config.json`.

Do not put long-lived host or `host:port` settings on the `vexod start` command line. Edit `rpc.address`, `p2p.listen_address`, `p2p.peers`, and `p2p.seeds` in `network_config.json` instead.

## Consensus Timing

Consensus loop timing lives in `consensus_config.json`:

```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false
  }
}
```

- `timeout_propose` controls how long a round waits for a proposal.
- `timeout_prevote` controls the vote collection window.
- `timeout_precommit` controls the commit-certificate collection window.
- `timeout_commit` controls the minimum delay after a committed block.
- `create_empty_blocks: false` means the node only proposes when transactions are available.

`round_timeout` is kept only as a compatibility aggregate. Prefer the Tendermint-style timeout fields above.

## Multi-Validator Network

For a generated network:

```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```

Each generated validator home receives:

- its own `validator.key.json`
- its own split config files: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json`
- a shared `genesis.json`
- `network_config.json` peer entries for the other validators

For containerized or multi-host networks, put topology values in a JSON file:

```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```

- `p2p_host_template` and `rpc_host_template` are dial targets written into each node's `network_config.json` peer list. In Docker, these can be service names such as `validator-%d`.
- `p2p_advertise_host_template` and `rpc_advertise_host_template` are public addresses written into validator metadata in `genesis.json`. Use DNS names or public IPs here for public networks.
- `p2p_listen_host` and `rpc_listen_host` are local bind hosts. Use `0.0.0.0` for containers or servers that should listen on all interfaces.
- Do not reuse Docker-only service names as advertised public addresses unless the network is intentionally private.

Then generate node homes from that file:

```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
