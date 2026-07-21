# Node Initialization

This guide explains how to initialize validator and archive node homes.

Peer connectivity should be configured in `network_config.json`, not passed repeatedly on the `start` command line.

Runtime behavior that affects consensus, RPC, P2P, logging, or managed Web3 accounts is config-file only. `vexod start` rejects flags such as `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env`, and `--evm-account-key`; edit the split config files instead so every operator reviews the same deterministic node behavior.

There is no node-mode switch. A node home is defined by its config files, genesis, key material, and whether `validator_id` plus a signer are present.

## Validator Node

Use `init validator` when the node will propose, vote, sign consensus messages, and participate in validator rotation.

```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```

Set `VEXO_KEY_PASSPHRASE` before running this command, or pass `--passphrase` for a one-off local setup. `--encrypt-keys` encrypts `validator.key.json`, `node.key.json`, and `validator.vrf.key.json`.

Key custody rule of thumb:

- `validator.key.json` signs consensus proposals, votes, timeout votes, and finality-related messages.
- `node.key.json` signs P2P handshakes only; it must never be reused as the validator consensus key.
- `validator.vrf.key.json` proves committee randomness and should be treated like validator custody material.
- Public listeners must use encrypted local key documents or remote signer/KMS-style key documents. If a node exposes public RPC or authenticated public P2P while `require_network_safety=true`, startup rejects plaintext local validator keys.
- Generated keys are written with filesystem mode `0600`; still prefer a remote signer/KMS for long-lived validators.

For a BLS consensus key:

```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```

`--key-type bls` writes a `blst-bls12381-minpk-v1` BLS key document and copies the proof-of-possession into `genesis.json` validator metadata as `bls_pop`.

This creates:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` is the consensus signer. `node.key.json` is the P2P handshake signer referenced by `network_config.json:p2p.node_key_path`. They are deliberately separate so archive nodes and validators can use the same transport without giving every peer a validator signing key.

Start it with config-driven networking:

```bash
vexod start --home .vexo-validator-1
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
- `node.key.json`
- `data/`

It does **not** create `validator.key.json`.

Start it with:

```bash
vexod start --home .vexo-archive-1
```

## Split Configuration Files

Node homes use separate config files so operators can edit one subsystem without mixing unrelated settings:

- `config.json` contains node identity, chain ID, data path, and pointers to the split config files.
- `module_config.json` contains application module selection, execution/ante policy, and module-level governance policy.
- `network_config.json` contains RPC, P2P node identity, listen/peer/seed settings, TLS/auth settings, and peer-scoring policy.
- `consensus_config.json` contains consensus loop timing, empty-block policy, crypto backend, VRF, validator admission, and committee policy.
- `mempool_config.json` contains mempool size, fee, priority, WAL, duplicate, and TTL policy.
- `log_config.json` contains log format, level, block commit event logging, and peer event logging.
- `genesis.json` contains immutable genesis validators, validator metadata, and genesis module state.

`network_config.json` RPC settings also include `shutdown_timeout`, `web3_max_subscriptions_per_connection`, and `web3_idle_timeout`. `shutdown_timeout` bounds graceful shutdown for the consensus loop, RPC server, and node transport so operators do not wait forever on a stuck stop path. The generated default is `10s`; Web3 subscriptions default to 256 per connection with a `2m` idle timeout so public RPC endpoints cannot accumulate unbounded idle subscriptions.

`network_config.json` P2P settings include `auth_replay_path`, `require_auth_replay_store`, and `dial_timeout`. The generated default writes nonce replay evidence to `data/p2p_auth_replay.jsonl` and uses a `10s` outbound dial timeout. For private loopback testing the replay store is mostly harmless bookkeeping; for public authenticated P2P it is a safety requirement because it prevents a captured signed handshake nonce from being replayed after restart. `dial_timeout` should be long enough for TLS, signed handshake verification, and cross-region latency; setting it too low makes healthy peers look flaky and can slow liveness after restarts.

`network_config.json` also owns startup state sync. This is useful for archive nodes, replacement validators, or nodes restored onto a clean machine. When `state_sync.enabled` is true, `vexod start` downloads the first valid snapshot from `state_sync.snapshot_urls`, verifies chain ID, checksum, state roots, and KV namespaces, restores it into LevelDB, rebuilds indexes, and only then starts the node. If local state already satisfies `state_sync.min_height` and `state_sync.trust_local_higher` is true, startup logs `state_sync_skipped` and keeps the local store.

Example `state_sync` block:

```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```

Startup logs `state_sync_candidate_failed` for a fetch error, `state_sync_candidate_rejected` for an invalid or stale snapshot, and `state_sync_applied` after a verified restore. Keep `max_snapshot_bytes` below the largest snapshot your infrastructure intentionally serves, but high enough for normal state growth. Do not point public nodes at an unauthenticated third-party snapshot source unless the operator has an out-of-band trust policy and finality/light-client evidence for that source.

If a field changes network behavior, edit the split config file and commit or distribute that reviewed file. Do not rely on long `vexod start` flags for runtime behavior. The start command intentionally rejects consensus timing, empty-block, P2P auth, RPC admin, and managed Web3 key flags so operators do not accidentally run different behavior from the reviewed config.

## Key Types

Validator init defaults to `--key-type bls` because network-safety validation requires audited BLS aggregate finality. `--key-type ed25519` remains available for private experiments and custom deployments outside the network-safety gate. `--encrypt-keys` should be used for any non-throwaway node home. Standalone key generation also supports VRF keys:

```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```

VRF keys are not consensus signers. They are used for VRF-backed committee selection and should be referenced from `consensus_config.json` through `vrf_key_paths` plus validator metadata key `vrf_public_key` when that backend is enabled.

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
    "CoreModules": ["bank", "staking", "governance", "params", "ibc"],
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
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

`CoreModules` is the chain-wide stable backbone. `Modules` may extend that backbone with optional capabilities, but the core prefix must remain intact across all validators that participate in the same consensus state.

Governance policy also lives in `module_config.json`. Generated network-safe configs require a proposal deposit:

```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```

The deposit is native balance escrowed from the proposal submitter. Passing proposals refund the deposit; rejected proposals move it to `RejectedDeposits`. Use an address controlled by your treasury/community-pool module if rejected deposits should fund a treasury instead of the default module account.

Example `network_config.json`:

```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
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

`rpc.evm_account_key_envs` and `rpc.evm_account_private_keys` are optional and back Web3 managed-account methods such as `eth_accounts`, `eth_sign`, `eth_signTransaction`, and `eth_sendTransaction`. Prefer `evm_account_key_envs` so the private key is injected by the process environment or secret manager instead of stored in JSON. Keep both lists empty for normal validator operation unless this node is intentionally acting as a local Web3 hot-wallet endpoint. Startup safety rejects managed EVM hot keys on public RPC listeners.

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
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```

`vrf_key_paths` are resolved relative to the directory containing `consensus_config.json`. Use encrypted key documents and provide `VEXO_KEY_PASSPHRASE` to the node process when local VRF key custody is unavoidable. Do not put raw VRF private scalars directly in `consensus_config.json` for operator-run networks.

Use `vexod config paths --home <home>` to inspect all resolved paths.

Archive config has:

```json
{
  "schema_version": "v1",
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

Generated validator homes set `"require_network_safety": true` in `config.json` by default. This is not a mode; it is a startup safety gate that rejects deterministic crypto, unsigned/nonced-off transactions, missing fee/gas floors, missing durable mempool WAL, missing replacement policy for same signer/nonce transactions, unsafe committee randomness, and `execution_commit` values other than `finalized`.

When `require_network_safety` is enabled, run:

```bash
vexod config audit --home <home> --strict
```

before starting the node. The audit should pass for every validator and archive home that participates in the same network.

## Config-Based Peers

Peer and listen addresses live in `network_config.json`:

```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
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
vexod start --home .vexo-archive-1
```

Persistent peers and seeds are configured in `network_config.json`; `vexod start` does not accept peer or seed host overrides.

Do not put long-lived host or `host:port` settings on the `vexod start` command line. Edit `rpc.address`, `p2p.listen_address`, `p2p.peers`, and `p2p.seeds` in `network_config.json` instead.

Keep `p2p.node_id` stable for the lifetime of the node home. `p2p.node_key_path` should point to `node.key.json` or another local/managed key document used only for peer handshake signing. Peer maps should use peer node IDs, not account addresses or validator operator names unless those are intentionally the same.

For encrypted and authenticated gRPC peer transport, also set `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path`, and optionally `p2p.tls_server_name` in `network_config.json`. Relative TLS paths are resolved from the node home directory. Keep `p2p.dial_timeout` in the same file so every operator uses the same reconnect behavior; do not hide peer timing in shell scripts.

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
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```

- `timeout_propose` controls how long a round waits for a proposal.
- `timeout_prevote` controls the vote collection window.
- `timeout_precommit` controls the commit-certificate collection window.
- `timeout_commit` controls the minimum delay after a committed block.
- `create_empty_blocks: false` means the node only proposes when transactions are available.
- `execution_commit: "finalized"` waits for the HotStuff three-chain finality decision before executing the finalized ancestor and is the generated validator default. `execution_commit: "qc"` executes and persists QC-certified blocks immediately, but the safety gate rejects it.

`round_timeout` is kept only as a compatibility aggregate. Prefer the Tendermint-style timeout fields above.

When `create_empty_blocks` is false, height can remain unchanged while the mempool is empty. That is expected: the chain is waiting for useful work instead of committing empty blocks. When a transaction appears and local consensus round state has drifted past another proposer, the node advances to the next round where its validator is proposer and builds from the mempool. This recovery path keeps transaction-triggered liveness without re-enabling empty-block spam.

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

`vexod network up` and `make network-e2e` use a process-level timeout while waiting for all validators to start, submit the smoke transaction, and observe height growth. The default command timeout is intentionally longer than the consensus interval because it covers process startup, LevelDB open, P2P signed handshakes, TLS/auth checks, transaction admission, and finality. If you lower consensus timeouts aggressively, keep the network-up timeout large enough to diagnose startup errors instead of killing the harness too early.

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

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
