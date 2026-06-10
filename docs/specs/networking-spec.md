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
- optional P2P auth proof or stronger peer authentication
- advertised listen address

Peers failing handshake authentication are rejected before gossip admission. When a shared P2P auth secret is configured, Vexo handshakes send a derived HMAC proof over the protocol/network/chain/genesis/node tuple rather than the raw secret.

## Address Roles

Vexo separates local bind addresses, peer dial addresses, and public advertised addresses:

- Local bind addresses live in `network_config.json` as `rpc.address` and `p2p.listen_address`.
- Peer dial addresses live in `network_config.json` under `p2p.peers` and `p2p.seeds`.
- Public advertised addresses live in validator metadata as `p2p_address` and `rpc_address`.

Container-only names such as Docker service names may be valid peer dial targets inside a private bridge network, but they should not be written as public validator metadata for a public network. Public validator metadata should use stable DNS names or public IP addresses that external peers can resolve.

Peer dial and discovered addresses must be dialable `host:port` values. Address-book and handshake discovery reject unspecified bind addresses such as `0.0.0.0:26656` or `[::]:26656`, malformed host strings, and port `0`. This prevents one node's local bind config from poisoning other nodes' peer books.

### Transport TLS

gRPC transport TLS is configured only through `network_config.json`:

- `p2p.tls_cert_path` and `p2p.tls_key_path` configure the local node certificate and must be paired with `p2p.tls_ca_path`.
- `p2p.tls_ca_path` enables peer certificate verification and requires client certificates; Vexo rejects TLS identity config without a CA trust root.
- `p2p.tls_server_name` sets the expected server name for outbound verification.

Relative TLS paths are resolved against the node home directory. Operators should keep public dial addresses in `p2p.peers`/`p2p.seeds`, local bind addresses in `p2p.listen_address`, and certificate trust material in these TLS fields rather than passing long-lived host or TLS state on the `start` command line.

### RPC TLS

HTTP RPC can also be bound with TLS from `network_config.json`:

- `rpc.tls_cert_path` and `rpc.tls_key_path` configure the RPC server certificate and must be set together.
- `rpc.tls_ca_path` enables client-certificate verification and turns the listener into mTLS.
- `rpc.tls_server_name` is allowed only when `rpc.tls_ca_path` is configured, so pinned-name validation cannot be accidentally used without a trust root.

Relative RPC TLS paths are resolved against the node home directory. Public RPC listeners should combine TLS/mTLS with `rpc.admin_token` or scoped `rpc.admin_tokens`; private loopback listeners may omit TLS for local operator workflows.

## Peer Scoring

Peers are scored by:

- valid messages
- malformed messages
- rate-limit violations
- invalid consensus/evidence payloads
- repeated dial failures

Score behavior:

- Score at or below `BanThreshold` causes temporary ban and disconnect.
- Score above `MaxScore` is capped.
- Score arithmetic uses saturating operations so long-running honest gossip cannot overflow integer score state.
- Existing persisted scores above `MaxScore` are capped when peer score state is restored.
- Persisted peer-score documents carry a schema version; unsupported schema versions must fail startup or operator recovery rather than being silently ignored.

Score persistence should not be on the consensus hot path. Implementations should persist on shutdown, periodic score-window maintenance, or explicit operator snapshot paths.

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

Admin RPC endpoints must fail closed when no admin token is configured.

## Operational Signals

Networking health should be monitored with:

- peer count
- banned peer count
- peer window message count
- reconnect failures
- invalid message rate
- gossip latency
