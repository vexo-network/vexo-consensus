# Transaction Format

## Scope

This spec defines Vexo transaction envelope requirements for public-network operation.

## Raw Payload

Current demo payloads are colon-delimited module commands:

```text
bank:send:<from>:<to>:<amount>:fee=<fee>:gas=<gas>:signer=<address>:nonce=<nonce>
```

Production modules may replace payload encoding, but must preserve the ante metadata model.

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
