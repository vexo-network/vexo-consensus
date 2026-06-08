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

## Required Ante Metadata

Public networks should require:

- `signer`
- `nonce`
- `fee`
- `gas`

Account sequence state is stored under the `auth` namespace and is advanced only after successful
transaction delivery. The first valid nonce is `1`; after committing nonce `N`, the next accepted
nonce is `N+1`.

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

## Fee and Gas

- `fee` is used for mempool admission and fee collection.
- `fee` accepts raw atomic values or suffixed units: `avxo`, `gvxo`, and `vexo`.
- `gas` is bounded by configured min/max gas. `gas_limit` is accepted as an alias.
- `base_fee` is configured per gas unit; required fee is `max(min_fee, base_fee * gas)`.
- `blob_base_fee`, `blob_gas`, `blob_gas_fee_cap`, and blob versioned hashes are tracked for EIP-4844-style raw Ethereum transactions; blob fee cap must cover the current blob base fee and committed blocks persist blob gas usage/excess blob gas.
- Blob sidecars are submitted through Vexo RPC, not Ethereum devp2p. `vexo_sendRawBlobTransaction` carries the signed Ethereum blob transaction plus a sidecar object with `blobHashes`/`blob_hashes`, `blobs`, `commitments`, and `proofs`; the EVM module verifies geth KZG4844 proofs, commitment-derived versioned hashes, strict blob/commitment/proof lengths, persists the committed sidecar, and exposes it through `vexo_getBlobSidecarByTxHash` or `vexo_getBlobSidecarByBlobHash`.
- The geth VM adapter passes decoded blob hashes into `TxContext`, so contracts using the `BLOBHASH` opcode see the transaction's blob hashes. This targets Ethereum execution semantics inside Vexo consensus, not full Ethereum node/network compatibility.
- `evm_chain_id`/`EVMChainID` is the EIP-155/Web3 chain ID used by `eth_chainId`, `net_version`, and `eth_sendRawTransaction` signature validation.
- Mempools may replace an existing pending transaction with the same `signer` and `nonce` only when the new `fee` or `priority` meets the configured replacement bump.
- When `dynamic_base_fee` is enabled, each committed block stores the base fee used for that block and the next base fee derived from total gas used versus `target_gas`.
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
```
