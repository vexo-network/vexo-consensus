# Adding a Validator

This guide describes the operator flow for adding a validator to a Vexo network.

The exact admission path depends on the chain's staking and governance policy. At minimum, the validator must be represented in chain state, have valid credentials, and become part of a height-versioned validator set update.

## 1. Initialize Validator Home

```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```

For a BLS validator key:

```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```

Set `VEXO_KEY_PASSPHRASE` before running these commands, or pass `--passphrase` for a one-off local setup.

When admitting a BLS validator to an existing chain, include the generated `bls_pop` metadata in the validator update proposal.
The default BLS key path uses `blst-bls12381-minpk-v1`; use `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` only for reference/compatibility testing.

Archive the generated public key:

```bash
vexod keys show --home .vexo-validator-new --json
```

## 2. Configure Network Addresses and Peers

Edit `.vexo-validator-new/network_config.json` and set local listen addresses plus persistent peers:

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
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```

Do not rely on long-lived command-line networking overrides for production validators. Keep persistent peer addresses in `network_config.json`.

Use separate address roles:

- `p2p.listen_address` and `rpc.address` are local bind addresses for this machine or container.
- `p2p.peers` contains dial targets this node uses to reach other peers.
- validator metadata `p2p_address` and `rpc_address` should contain public advertised addresses, not Docker-only service names, unless the network is intentionally private.

## 3. Submit Validator Admission

For example staking flows, build a staking transaction:

```bash
vexod staking --help
```

The validator admission transaction should include:

- validator ID
- validator address
- consensus public key
- voting power or stake reference
- validator commission basis points, if the chain allows self-service commission updates
- public P2P address metadata
- public RPC address metadata, if public
- BLS proof-of-possession metadata when BLS is enabled

The validator update must become effective at a specific height and produce a new validator-set hash.

After the validator is active, operators can expose reward state through the staking module:

```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```

## 4. Verify Validator Set Update

After the update height:

```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```

Check:

- validator appears in the height-specific set
- voting power is correct
- validator-set hash changed as expected
- finality proofs reference the correct validator-set height

## 5. Plan Validator Key Rotation

Validator keys can be rotated by preparing a next key document with non-overlapping `active_from` and `active_until` metadata, then starting the node with the extra rotation key:

```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```

At signing time, the node uses the key whose active window contains the consensus height. Remote signer key documents keep the same policy, auth-token, and double-sign guard requirements.

## 6. Start Validator

```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```

Startup has no network mode switch. Use `config audit --strict` before startup when the network is expected to satisfy public-network safety assumptions.

## 7. Monitor

Watch:

- proposal/vote latency
- round timeouts
- validator signing failures
- peer bans
- mempool size
- commit latency
- snapshot/replay health

Use:

```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```

## Safety Notes

- Never reuse validator keys across independent chains.
- Keep remote signer policy enabled for production validators.
- Do not admit a BLS validator without proof-of-possession or equivalent rogue-key defense.
- Do not slash or jail a validator without verified evidence tied to the correct evidence-height validator set.
