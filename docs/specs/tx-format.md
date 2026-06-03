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
- Result metadata should expose gas used and fee paid.

## Load Test Payloads

Public-network load tests should use realistic signed `bank:send` payloads with fee, gas, signer, and incrementing nonce, not unrestricted mint payloads.

## CLI Examples

```bash
vexod keys show --home .vexo
vexod tx build --module bank --action send --args vexo1...,vexo1...,1 --tags fee=1gvxo,gas=100,signer=vexo1...,nonce=7
vexod tx parse --tx bank:send:vexo1...:vexo1...:1:fee=1gvxo:gas=100:signer=vexo1...:nonce=7 --json
```
