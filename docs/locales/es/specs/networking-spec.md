# Networking Spec

> Locale: es · Español
> Este documento es una guía traducida desde la documentación canónica en inglés. Las decisiones de protocolo, seguridad y publicación siguen siendo normativas en inglés.

## Propósito

Este documento cubre P2P handshake, gossip, peer scoring y política de ban. Los comandos, campos JSON, nombres RPC, config key e identificadores de código usados en implementación y operación se mantienen en inglés por compatibilidad.

## Alcance principal

- Revise los siguientes puntos al leer este documento. Los comandos, campos JSON, métodos RPC, claves de configuración e identificadores de código se mantienen en inglés por compatibilidad.
- Para redacción normativa detallada, use el documento inglés.
- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/es/specs/networking-spec.md`

## Identificadores que se conservan

- `consensus`
- `tx`
- `commit`
- `evidence`
- `network_config.json`
- `rpc.address`
- `p2p.listen_address`
- `p2p.peers`
- `p2p.seeds`
- `p2p_address`
- `rpc_address`
- `host:port`
- `0.0.0.0:26656`
- `[::]:26656`
- `0`
- `p2p.tls_cert_path`
- `p2p.tls_key_path`
- `p2p.tls_ca_path`

## Secciones en inglés

- Networking Spec
- Scope
- Transport
- Topics
- Handshake
- Address Roles
- Transport TLS
- Peer Scoring
- Reconnect and Backoff
- DoS/DDOS Defenses
- Operational Signals

## Notas operativas

- `MUST`, `SHOULD`, `MAY`, ejemplos de comandos, ejemplos JSON y nombres RPC mantienen la grafía inglesa.
- Después de cambiar esta traducción, ejecute `make docs-check`.
- Si esta página contradice la fuente inglesa, use la fuente inglesa y actualice este archivo locale en el mismo cambio.

## Fuente canónica

- [English canonical document](../../en/specs/networking-spec.md)
