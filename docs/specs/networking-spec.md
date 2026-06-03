# Networking Spec

## Scope

This spec defines peer communication, handshake expectations, scoring, reconnection, and DoS resistance.

## Transport

The transport interface is modular. Current implementations include:

- in-memory transport for tests
- TCP transport
- gRPC transport

Different binaries can peer if they speak the same transport protocol, chain ID, message topics, and handshake policy.

## Topics

- `consensus`: proposals, votes, timeout votes, QCs, and TCs
- `tx`: transaction gossip
- `commit`: committed block announcements
- `evidence`: evidence gossip

## Handshake

A production handshake should bind:

- chain ID
- node ID
- supported protocol version
- transport protocol version
- optional auth token or stronger peer authentication
- advertised listen address

Peers failing handshake authentication are rejected before gossip admission.

## Peer Scoring

Peers are scored by:

- valid messages
- malformed messages
- rate-limit violations
- invalid consensus/evidence payloads
- repeated dial failures

Score below ban threshold causes temporary ban and disconnect.
Score above max score is capped, and score arithmetic uses saturating operations so long-running honest gossip cannot overflow integer score state.

## Reconnect and Backoff

- Dial failures are tracked in the address book.
- Reconnect uses backoff to avoid dial storms.
- Banned peers are evicted from active dial sets until ban expiration.

## DoS/DDOS Defenses

- request/body size limits
- message size limits
- per-peer score windows
- global score windows
- invalid-message penalties
- peer bans and disconnects
- admin-token protection for mutating RPC endpoints

## Operational Signals

Networking health should be monitored with:

- peer count
- banned peer count
- peer window message count
- reconnect failures
- invalid message rate
- gossip latency
