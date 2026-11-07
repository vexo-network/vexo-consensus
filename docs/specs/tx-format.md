# Transaction Format

## Scope

This spec defines Vexo transaction envelope requirements for public-network operation.

## Canonical Payload

Canonical payloads are colon-delimited module commands:

```text
<module>:<action>:<arg...>:fee=<fee>:gas=<gas>:signer=<address>:nonce=<nonce>:priority=<priority>
```

The canonical tag order is:

1. `fee`
2. `gas`
3. `gas_limit`
4. `signer`
5. `nonce`
6. `priority`
6. any custom tags sorted lexicographically

Application modules may define their own positional arguments, but should use the shared canonical
transaction parser/builder for ante metadata.

### Tag Grammar

The mempool, the ante handler and every module read a payload through one shared parser, so no layer
can be steered by a tag another layer reads differently. The grammar is strict: it rejects ambiguity
instead of picking a winner.

- A tag key appears at most once and is never empty.
- `signer` is never empty; `nonce`, `gas`, `gas_limit`, `priority` and `fee` must parse when present.
- `gas` and `gas_limit` may both appear only when they carry the same value.
- A signed envelope must decode, and its payload must not itself be a signed envelope.

A payload that breaks these rules is invalid. `CheckTx`, the mempool and proposal validation reject it,
and a block that carries it anyway executes it as a failed transaction that changes nothing.

## Address Format

Account and validator addresses are derived from public keys:

```text
payload = first20bytes(SHA256("vexo.address.v1:<hrp>:" || public_key || "\n"))
address = bech32(hrp, payload)
```

The built-in human-readable prefixes are:

- `vexo` for account addresses used in transaction `signer` tags.
- `vexovaloper` for validator operator addresses stored in genesis validator records.
- `vexovalcons` for validator consensus-key addresses stored as validator metadata.

Signed transactions that include `signer=<address>` must use the `vexo` address derived from the
envelope public key. A signed transaction with a mismatched signer address is invalid.

The EVM/Web3 bridge also accepts Ethereum `0x` account addresses for Ethereum raw transactions and EVM module calls. Those 20-byte hex addresses are normalized to lowercase storage keys for bank balance reads/writes, while Web3 responses may preserve checksum address text returned by go-ethereum.
Ethereum raw transactions are verified against the explicit execution `evm_chain_id`/`EVMChainID` configured for the chain, not a hash-derived value from the Vexo `chain_id`. Keep this ID stable for Web3 tooling compatibility.

## Signed Envelope

Signed transactions use an envelope over the raw payload:

- schema version
- payload
- public key
- signature

The signature is domain-separated by chain ID. The envelope public key is also used to verify the
payload signer address when the `signer` tag is present.

## Authorization

The ante handler authenticates the `signer` tag: through the signed envelope, or for an Ethereum
transaction through the sender the EVM module recovered from the raw transaction. A module that acts on
behalf of an account named in its positional arguments must require `signer` to be that account, after
address normalization. For the bank module:

- `bank:send:<from>:<to>:<amount>` requires `signer=<from>`.
- `bank:mint:<to>:<amount>` requires the configured bank mint authority as `signer`. An empty mint
  authority disables minting by transaction. A mint authority that no key can sign for (the network
  default is the `governance` module account) leaves minting to the governance module itself.
- Sending to oneself moves nothing; it only checks that the sender can afford the amount.

## Ethereum Transactions

Tags prefixed `eth_` describe a raw Ethereum transaction decoded by the EVM module and carry no
authority of their own. A transaction with any `eth_` tag is admitted only when it is unsigned, is
addressed to the `evm` module, carries `eth_raw` and `eth_hash`, and the EVM module authenticates it
(the signature inside `eth_raw` recovers the `signer`, and every tag the executor reads matches the raw
transaction). Every other transaction with an `eth_` tag is rejected: a signed one, one addressed to
another module, or one the EVM module does not authenticate. Tags never switch off the signature
requirement or the fee.

## Required Ante Metadata

Public networks should require:

- `signer`
- `nonce`
- `fee`
- `gas`

Account sequence state is stored under the `auth` namespace. The first valid native nonce is `1`; after
committing nonce `N`, the next accepted nonce is `N+1`.

A transaction that is admitted into a block pays its fee and consumes its nonce even when its module
rejects it; only the module's own state changes are discarded. A transaction that cannot be admitted
(bad signature, wrong nonce, fee the payer cannot afford, no route) changes nothing and is recorded as a
failed result. `FinalizeBlock` never fails because of transaction content: it fails only for
infrastructure errors such as a store failure or a cancelled context.

## CheckTx Requirements

`CheckTx` must reject:

- malformed signed envelopes
- invalid signature
- missing signer or nonce when required
- nonce replay or wrong sequence
- fee below minimum
- gas below minimum or above maximum
- oversized transactions
- duplicate transactions
- unsupported module route
- a fee the payer cannot afford together with the earlier fees of the same block
- a nonce that does not follow the sender's earlier transactions of the same block

`PrepareProposal` and `ProcessProposal` apply these checks cumulatively over the whole block, so a
sender's nonces must be consecutive and its fees affordable together, and the blob gas of the block must
fit the configured limit.

## Fee and Gas

- `fee` is used for mempool admission and fee collection.
- `fee` accepts raw atomic values or suffixed units: `avxo`, `gvxo`, and `vexo`.
- `gas` is bounded by configured min/max gas. `gas_limit` is accepted as an alias.
- `base_fee` is the initial base fee per gas unit. The base fee in force is application state stored under
  the `feemarket` namespace and moved by every committed block; the required fee is
  `max(min_fee, base_fee * gas)` at the base fee in force. A node that restarts or replays blocks reads the
  same price as the node that produced them.
- `blob_base_fee`, `blob_gas`, `blob_gas_fee_cap`, and blob versioned hashes are tracked for EIP-4844-style raw Ethereum transactions; blob fee cap must cover the current blob base fee and committed blocks persist blob gas usage/excess blob gas.
- Blob sidecars are submitted through Vexo RPC, not Ethereum devp2p. `vexo_sendRawBlobTransaction` carries the signed Ethereum blob transaction plus a sidecar object with `blobHashes`/`blob_hashes`, `blobs`, `commitments`, and `proofs`; the EVM module verifies geth KZG4844 proofs, commitment-derived versioned hashes, strict blob/commitment/proof lengths, persists the committed sidecar, and exposes it through `vexo_getBlobSidecarByTxHash` or `vexo_getBlobSidecarByBlobHash`.
- The geth VM adapter passes decoded blob hashes into `TxContext`, so contracts using the `BLOBHASH` opcode see the transaction's blob hashes when the chain is running a post-London preset such as `latest`. This targets Ethereum execution semantics inside Vexo consensus, not full Ethereum node/network compatibility, and it is not part of the default London preset.
- `evm_chain_id`/`EVMChainID` is the EIP-155/Web3 chain ID used by `eth_chainId`, `net_version`, and `eth_sendRawTransaction` signature validation.
- Mempools may replace an existing pending transaction with the same `signer` and `nonce` only when the new `fee` or `priority` meets the configured replacement bump, and the replacement still pays the minimum fee.
- Mempools are bounded: `max_txs` bounds the pending count, `max_total_bytes` the combined size and `max_txs_per_sender` the share of one signer. A full pool admits a new transaction only by evicting pending transactions that pay strictly less.
- When `dynamic_base_fee` is enabled, each committed block stores the base fee used for that block and the next base fee derived from total gas used versus `target_gas`; the next base fee is written to application state by the block itself.
- When `dynamic_blob_base_fee` is enabled, each committed block stores the blob base fee used for that block and the next blob base fee derived from blob gas used versus `target_blob_gas`.
- Built-in modules expose estimated gas costs and consume gas during `DeliverTx`; under-gas transactions are rejected during context-aware `CheckTx`, `ProcessProposal`, and execution.
- Result metadata should expose gas used and fee paid.

## Load Test Payloads

Public-network load tests should use realistic signed `bank:send` payloads with fee, gas, signer, and incrementing nonce, not unrestricted mint payloads.

## CLI Examples

```bash
vexod keys show --home .vexo
vexod tx build --module bank --action send --args vexo1...,vexo1...,1 --tags fee=1gvxo,gas=100,signer=vexo1...,nonce=7
vexod tx parse --tx bank:send:vexo1...:vexo1...:1:fee=1gvxo:gas=100:signer=vexo1...:nonce=7 --json
vexod evm tx call evm 0xaaaa 0xbbbb call 0x 21000 0x10000000000000000
```

EVM module CLI value arguments accept decimal atomic values or `0x` hex quantities and normalize them
to canonical decimal strings before transaction construction. This keeps CLI-built EVM payloads aligned
with raw Ethereum transaction decoding, where value and fee fields are unsigned 256-bit quantities.
