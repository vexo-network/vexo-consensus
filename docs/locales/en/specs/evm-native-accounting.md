# EVM and Native Accounting

This document is a normative accounting specification for Vexo native balances, fees, and the built-in EVM module.

## Core Rule

Vexo native coin balances and EVM account balances are the same economic asset.

- The atomic unit is `avxo`.
- Display units are `gvxo` (`10^9 avxo`) and `vexo` (`10^18 avxo`).
- Native `bank` transfers, ante fees, staking/reward accounting, and EVM value transfers read and write the same `bank` namespace for account balances.
- Ethereum `0x` account addresses are normalized to lowercase 20-byte hex keys before balance reads and writes.
- Bech32 Vexo account addresses remain plain account keys.

## Amount Encoding

Balances are unsigned 256-bit integers encoded as big-endian bytes.

- New writes use 8-byte big-endian encoding for values that fit in `uint64` to preserve legacy compatibility.
- New writes use minimal big-endian encoding for values above `uint64`.
- Readers must accept any non-empty value up to 32 bytes.
- Values above 256 bits are invalid.
- Missing balance keys are interpreted as zero.

## Fee Accounting

The ante layer parses `fee` as a 256-bit atomic amount.

- `fee=1`, `fee=1avxo`, `fee=1gvxo`, and `fee=1vexo` are valid.
- `base_fee * gas` is computed with arbitrary-precision arithmetic before the 256-bit storage boundary is checked.
- Non-Ethereum Vexo transactions pay fees from the signer balance to the configured fee collector.
- Raw Ethereum transactions do not pay ante-layer fees; their gas/value accounting is executed by the EVM state transition and then persisted back into the same native balance namespace.
- Raw Ethereum transaction fee metadata is still reported for block metrics and RPC surfaces. Balance mutation remains EVM-owned to avoid double charging the same native asset.

## EVM Execution

The built-in EVM adapter preserves Ethereum 256-bit value and fee fields.

- Raw Ethereum transaction `value`, effective gas price, fee cap, priority fee cap, blob fee cap, and total fee are decoded as `uint256`-compatible values.
- Canonical Vexo wrapper tags store those values as decimal strings even when they exceed `uint64`.
- The geth-backed VM adapter receives 256-bit gas price and fee-cap fields through the `contract.Invocation` boundary.
- VM balance writes are persisted as native bank balances, so `eth_getBalance` and `bank query balance` observe the same underlying asset for Ethereum `0x` accounts.
- EVM receipts report `gasUsed` for the transaction and `cumulativeGasUsed` as the sum of receipt gas used earlier in the same block plus the current transaction, matching Web3 client expectations.

## Compatibility Boundary

Vexo does not become an Ethereum node.

- Vexo keeps its own consensus, P2P, state sync, fork choice, validator lifecycle, and block format.
- EVM compatibility means Ethereum execution semantics and Web3-facing account/transaction behavior inside a Vexo network.
- Ethereum devp2p, Ethereum fork-choice, and Ethereum sync semantics are intentionally outside this accounting spec.
- Uncle-related Web3 methods intentionally return zero or `null` because Vexo consensus does not produce Ethereum uncle blocks.
- Vexo does not expose geth stateless execution-witness RPC; use Vexo finality proofs, query proofs, and retained EVM snapshots for verification.

## Failure Modes

Implementations must fail closed when:

- a stored balance is longer than 32 bytes
- a parsed amount is negative or larger than 256 bits
- a fee collector balance would overflow 256 bits
- an EVM state transition returns invalid balance writes
- checksum/lowercase Ethereum address aliases would split account balance state
