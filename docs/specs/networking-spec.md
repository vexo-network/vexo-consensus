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

## Address Roles

Vexo separates local bind addresses, peer dial addresses, and public advertised addresses:

- Local bind addresses live in `network_config.json` as `rpc.address` and `p2p.listen_address`.
- Peer dial addresses live in `network_config.json` under `p2p.peers` and `p2p.seeds`.
- Public advertised addresses live in validator metadata as `p2p_address` and `rpc_address`.

Container-only names such as Docker service names may be valid peer dial targets inside a private bridge network, but they should not be written as public validator metadata for a public network. Public validator metadata should use stable DNS names or public IP addresses that external peers can resolve.

## Peer Scoring

Peers are scored by:

- valid messages
- malformed messages
- rate-limit violations
- invalid consensus/evidence payloads
- repeated dial failures

Score behavior:

- Score below `BanThreshold` causes temporary ban and disconnect.
- Score above `MaxScore` is capped.
- Score arithmetic uses saturating operations so long-running honest gossip cannot overflow integer score state.
- Existing persisted scores above `MaxScore` are capped when peer score state is restored.

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
