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
3. `signer`
4. `nonce`
5. `priority`
6. any custom tags sorted lexicographically

Application modules may define their own positional arguments, but should use the shared canonical
transaction parser/builder for ante metadata.

## Signed Envelope

Signed transactions use an envelope over the raw payload:

- schema version
- payload
- public key
- signature

The signature is domain-separated by chain ID.

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
- `gas` is bounded by configured min/max gas.
- Result metadata should expose gas used and fee paid.

## Load Test Payloads

Public-network load tests should use realistic signed `bank:send` payloads with fee, gas, signer, and incrementing nonce, not unrestricted mint payloads.

## CLI Examples

```bash
vexod tx build --module bank --action send --args alice,bob,1 --tags fee=2,gas=100,signer=alice,nonce=7
vexod tx parse --tx bank:send:alice:bob:1:fee=2:gas=100:signer=alice:nonce=7 --json
```
