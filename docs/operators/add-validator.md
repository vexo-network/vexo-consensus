# Adding a Validator

This guide describes the operator flow for adding a validator to a Vexo network.

The exact admission path depends on the chain's staking and governance policy. At minimum, the validator must be represented in chain state, have valid credentials, and become part of a height-versioned validator set update.

## 1. Initialize Validator Home

```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new
```

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

For demo staking flows, build a staking transaction:

```bash
vexod staking --help
```

The production admission transaction should include:

- validator ID
- validator address
- consensus public key
- voting power or stake reference
- public P2P address metadata
- public RPC address metadata, if public
- BLS proof-of-possession metadata when BLS is enabled

The validator update must become effective at a specific height and produce a new validator-set hash.

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

## 5. Start Validator

```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new --run
```

Startup has no network mode switch. Use `config audit --strict` before startup when the network is expected to satisfy public-network safety assumptions.

## 6. Monitor

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
