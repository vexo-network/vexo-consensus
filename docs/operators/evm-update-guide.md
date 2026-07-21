# EVM Update Guide

This guide explains how to update the built-in EVM stack without breaking chain ID handling, Web3 compatibility, or release evidence. It is written for operators and maintainers who need to bump go-ethereum, adjust fork presets, or change EVM behavior in a controlled release.

## What Counts as an EVM Update

An EVM update is any change that can affect Ethereum-style execution or Web3-facing behavior:

- `go-ethereum` version bumps in `modules/evm/backend/geth`
- changes to `modules/evm/ethcompat`
- changes to `modules/evm`
- changes to `execution.evm_fork_preset`
- changes to `execution.evm_chain_config_json`
- changes to raw transaction admission, gas accounting, receipts, traces, proofs, or block response fields
- changes to managed Web3 account handling such as `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction`, or `eth_sendTransaction`

If any of those change, treat the update as a release-sensitive feature update, not a simple refactor.

## Safe Update Order

Use this order so the code, config, and docs stay aligned:

1. Update the isolated geth-backed adapter first.
2. Update the fixture corpus and conformance tests next.
3. Update `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md`, and `docs/sdk/rpc-api-versioning.md` when semantics change.
4. Update `docs/release/release-pipeline.md` when the release evidence shape changes.
5. Update the node configuration docs if the operator-facing knobs change.
6. Re-run the validation matrix before merging.

Do not bump the EVM runtime version and ship it at the same time unless the conformance suites, RPC smoke checks, and Docker deployment checks have passed.

## Update Workflow

### 1. Pin the change

Record the exact intent of the update:

- fork behavior only
- transaction admission only
- execution semantics only
- RPC compatibility only
- blob / receipt / trace handling only
- managed account or wallet behavior only

That split keeps the review focused and prevents unrelated code from moving together.

### 2. Make the code change in the narrowest layer

Prefer these boundaries:

- `modules/evm/backend/geth` for upstream go-ethereum integration changes
- `modules/evm/ethcompat` for raw transaction decoding, hash preservation, and fixture handling
- `modules/evm` for state transition, receipts, logs, storage, and snapshot behavior
- `rpc` for Web3 request/response surface changes
- `cmd/vexod` only when the CLI or release workflow must expose the new behavior

If the change reaches application modules, keep the module boundary explicit and preserve deterministic state writes.

### 3. Refresh configuration defaults

When semantics change, update the default config in the same patch:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- `network_config.json` RPC fields for managed accounts, when needed
- `module_config.json` EVM chain ID

Never rely on a hidden CLI flag to explain runtime behavior. Config should make the node behavior obvious from files alone.

### 4. Run the conformance stack

At minimum, run:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Then verify the user-visible flows that usually break first:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

For Docker single-host deployments, also verify:

```text
http://127.0.0.1:28657/web3
```

Check at least these behaviors:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

Then deploy a simple contract, deploy a proxy contract, and exercise the upgrade path with the same RPC endpoint the wallet or tool will use in production.

### 5. Confirm proxy and upgrade behavior

The EVM update is not done until all of these are true:

- a plain contract deploy succeeds
- a proxy deploy succeeds
- a UUPS upgrade call succeeds
- post-upgrade reads return the expected storage and code
- nonce tracking remains monotonic
- the block producer accepts the resulting transactions without unsafe proposal errors

If a proxy deploy works but upgrade fails, the change is not shippable yet. Treat that as a release blocker, not a warning.

### 6. Refresh evidence

When the EVM surface changes, update the release evidence bundle:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- any pinned SHA-256 fixture references

Release evidence should say what changed, what was tested, and which commit or version was verified. Do not describe an EVM update as complete unless the evidence matches the code that was actually exercised.

## Validation Matrix

Use this as the merge gate:

| Check | Why it matters |
| --- | --- |
| `make evm-conformance` | Catches fork-rule and execution regressions |
| `go test ./modules/evm -count=1` | Verifies receipts, logs, storage, balances, and snapshots |
| `go test ./rpc -count=1` | Verifies Web3 request and response compatibility |
| `make network-e2e` | Confirms the node still starts, peers, and commits |
| Docker single-host smoke | Confirms the path used by Remix and browser tools |
| Contract deploy | Confirms transaction admission and receipt generation |
| Proxy deploy | Confirms ABI and storage layout assumptions |
| UUPS upgrade | Confirms upgrade semantics and post-upgrade reads |

If any check is red, do not call the update done.

## Rollback Criteria

Roll back the EVM update when any of the following happens:

- `eth_chainId` changes unexpectedly
- `eth_sendRawTransaction` starts rejecting valid transactions
- `eth_call` or `eth_estimateGas` diverge from the expected fork rules
- receipts, logs, or proofs stop matching the committed state
- proxy or upgrade transactions begin to fail
- the release evidence no longer matches the current code path

Rollback should restore the last known good adapter version, config defaults, and fixture set together.

## Technical Parity Appendix

This appendix keeps the update guide aligned with the rest of the documentation tree.

- Keep `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc`, and `cmd/vexod` as the stable implementation boundaries.
- Keep `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction`, and `eth_sendTransaction` unchanged in spelling.
- Keep `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures`, and `--evm-web3-conformance-evidence` unchanged in spelling.
- Keep the operational question simple: did the update preserve Ethereum-style execution while still fitting Vexo consensus and release safety?

<!-- vexo-docs:technical-parity -->
