# Cosmos/Tendermint Comparison Gate

This document maps common Tendermint/CometBFT/Cosmos SDK maturity advantages to Vexo release gates.

Vexo should not claim public production readiness unless every row has attached evidence in `release gate`.

| Area | Tendermint/Cosmos Advantage | Vexo Gate |
|---|---|---|
| Operational history | Many years of public-network incidents, fixes, and operator practice | `--longrun-evidence`, `--chaos-evidence`, `--ops-runbook-evidence` |
| Independent review | External audits, ecosystem scrutiny, and battle-tested assumptions | `--external-audit`, `--formal-safety-evidence`, `--fuzz-evidence` |
| Ecosystem | Mature SDK modules, IBC, wallets, explorers, tooling, and tutorials | `--sdk-conformance-evidence` plus chain-specific integration evidence |
| EVM/Web3 surface | Widely tested wallets, JSON-RPC clients, transaction formats, VM execution, traces, fees, blobs, and accounting | `--evm-web3-conformance-evidence` with pinned transaction/execution fixture corpora |
| P2P maturity | Proven seed/addrbook behavior, reconnects, peer exchange, and DoS hardening | `--p2p-scale-evidence` |
| State sync/light clients | Widely exercised snapshot and light-client verification flows | `--state-sync-light-client-evidence`, `--snapshot-evidence` |
| Validator economics | Mature staking, slashing, unbonding, commission, rewards, and tombstone flows | `--validator-economics-evidence` |
| Governance upgrades | On-chain upgrade coordination with known failure playbooks | `--upgrade-governance-evidence` |
| Operations | Known metrics, alerts, runbooks, incident response, and archive procedures | `--ops-runbook-evidence` |
| Fee market/MEV | More ecosystem experience with fee pressure, spam, censorship, and ordering | `--mev-fee-market-evidence` |
| Signer/KMS | Operational signer tooling and validator custody practice | `--kms-evidence`, `--bls-audit` when BLS is enabled |

## Required Evidence Properties

- Evidence must be generated from the exact release candidate binary and config schema.
- Multi-host evidence must run on independent machines or independent failure domains.
- Metrics must include thresholds, not only raw values.
- Slashing and validator economics evidence must include negative tests for false slashing.
- Light-client evidence must bind finality proof height to the correct validator-set hash.
- SDK/Web3 evidence must cover Vexo's supported Ethereum execution/RPC surface and must not imply Ethereum devp2p, Ethereum fork-choice, or geth stateless execution-witness compatibility.
- MEV/fee-market evidence must include congested mempool, empty-block disabled, base-fee movement, fair-ordering consistency, and censorship-resistance drills.
- Upgrade evidence must include failed migration and rollback-required handling.

## Release Rule

If a Vexo release lacks any required evidence, the correct public statement is:

> The code has the framework hooks, but the network has not yet met the Vexo release gate for public production launch.
