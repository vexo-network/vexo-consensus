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
- The priority fee of a raw Ethereum transaction (`min(max_priority_fee_per_gas, max_fee_per_gas - base_fee)` per gas used) is credited to the configured bank fee collector (`execution.fee_collector`) in the same atomic write batch as the rest of the execution. The base fee portion is burned. Execution runs under a fixed coinbase alias and the executor sweeps whatever that alias holds into the fee collector, so no balance is stranded on an address nobody controls.

## EVM Execution

The built-in EVM adapter preserves Ethereum 256-bit value and fee fields.

- Raw Ethereum transaction `value`, effective gas price, fee cap, priority fee cap, blob fee cap, and total fee are decoded as `uint256`-compatible values.
- Canonical Vexo wrapper tags store those values as decimal strings even when they exceed `uint64`.
- The geth-backed VM adapter receives 256-bit gas price and fee-cap fields through the `contract.Invocation` boundary.
- Web3 transaction, pending transaction, and receipt responses expose `value`, `gasPrice`, `maxFeePerGas`, `maxPriorityFeePerGas`, `maxFeePerBlobGas`, and `effectiveGasPrice` as Ethereum hex quantities without truncating values above `uint64`.
- EVM module call queries accept `value_hex`, `gas_price_hex`, `max_fee_per_gas_hex`, and `max_priority_fee_per_gas_hex` so simulation paths can preserve full `uint256` semantics. The legacy numeric fields remain for small callers and are rejected when they conflict with the explicit hex encoding.
- VM balance writes are persisted as native bank balances, so `eth_getBalance` and `bank query balance` observe the same underlying asset for Ethereum `0x` accounts.
- EVM receipts report `gasUsed` for the transaction and `cumulativeGasUsed` as the sum of receipt gas used earlier in the same block plus the current transaction, matching Web3 client expectations.
- Blob transactions are accepted through `eth_sendRawBlobTransaction` or `vexo_sendRawBlobTransaction` with an explicit sidecar. `eth_sendRawTransaction` rejects blob transactions that reference blob hashes without sidecar data, because Vexo does not rely on Ethereum devp2p sidecar propagation.

## Authorization

A transaction acts only as the account that signed it.

- A native `evm call` or `evm deploy` names its caller in the first positional argument after the VM. The transaction must carry a `signer` tag that names the same account; addresses are compared canonically (`0x` hex addresses are lowercased and left-padded), so a differently cased spelling of the same address is accepted and any other account is rejected.
- Native callers and call targets must be `0x` followed by 1 to 40 hex digits. Any other string would collapse onto a truncated or zero address inside the EVM and is rejected.
- `eth_deploy` and Ethereum-flavored `call` transactions must carry the raw Ethereum transaction (`eth_raw`). The sender recovered from the raw signature must equal the `signer` tag and the caller argument, and every executable tag (nonce, gas, value, input, fee caps, access list, blob fields, `eth_hash`) must equal what the raw transaction encodes. The unsigned `fee` tag is bounded by what the signature authorizes (`gas * max_fee_per_gas`, plus blob gas times the blob fee cap).
- The `eth_hash` tag is never trusted. An Ethereum transaction is known by the keccak256 of its raw bytes and any other transaction by the SHA-256 of its payload. A receipt is written once per hash; a transaction whose receipt already exists is rejected instead of replacing it.
- The EVM account nonce and the ante account sequence are the same value under the same key (`auth` namespace, `nonce/<canonical address>`), so a nonce advanced by either side is the nonce the other side checks.

## Rejection and Reverts

An included transaction never fails the block because of its content.

- A transaction that fails a deterministic pre-check is rejected: a non-zero result and no writes. This covers a nonce that does not match the account, insufficient funds for value plus maximum gas cost, gas below the intrinsic cost, a fee cap below the base fee, a gas limit above the execution limit, an unknown VM and malformed payloads. A rejected transaction leaves no receipt, no state change, no nonce bump and pays the fee collector nothing; the application runtime charges the fee tag and advances the sender sequence where the payer can afford it.
- An EVM revert, out-of-gas or invalid opcode is an included transaction: result code 0, a receipt with status 0 and the gas it used.
- Reader and store failures are infrastructure errors and are not recorded as a transaction outcome.
- Execution results, receipts and consensus data carry no opcode trace. Traces are produced only on request by the debug and trace RPC methods and are size-bounded (4 MiB).

## Resource Bounds

- One execution never uses more gas than the transaction's own gas limit and the policy cap `execution.rpc_gas_cap` (default 50,000,000). The same cap bounds `eth_call` and `eth_estimateGas`; a request without a gas limit receives the cap and a limit above it is clamped to it. `eth_estimateGas` stops when the request context is cancelled.
- `Snapshot` and `RevertToSnapshot` inside the interpreter cost in proportion to the state changed inside the reverted frame, not to the state the transaction touched so far.
- `eth_getLogs` and the EVM `logs` query read only the blocks in `[fromBlock, toBlock]`. A missing `toBlock` is the latest block, a `toBlock` past the head stops at the head and a range wider than 10,000 blocks is rejected before any block is read. Log subscriptions deliver from the head at subscribe time.
- A constructor that returns no runtime code deploys no code; the init code is never stored as contract code.

## State Root Policy

Web3 `stateRoot` responses are fail-closed.

- Web3 block responses and `newHeads` require the retained EVM state root for the requested height.
- The app hash is not a safe substitute for Ethereum-compatible `stateRoot` in Web3 responses, so the RPC server never falls back to it.
- `execution.strict_evm_state_root` is retained for config compatibility; it does not permit app-hash substitution.
- Release evidence should include retained EVM snapshots or replay results for the heights used by Web3 clients.

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
- the caller of a native EVM transaction is not the account that signed it
- an Ethereum transaction's sender, nonce or fee tags do not match its signed raw bytes
- a Web3 block response cannot load the retained EVM state root while strict state-root mode is enabled
- a blob transaction is submitted without the explicit blob sidecar required by Vexo RPC
