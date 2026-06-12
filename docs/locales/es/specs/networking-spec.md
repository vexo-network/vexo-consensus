# Networking Spec

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Resumen

Este documento ayuda a entender P2P handshake, gossip, peer scoring y política de ban y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/es/specs/networking-spec.md`

## Por qué leer este documento

- P2P handshake, gossip, peer scoring y política de ban
- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- Este documento localizado ayuda a comprender; auditoría, release y seguridad se deciden con la fuente inglesa.

## Qué debería poder hacer después

- Explicar qué decisión de implementación u operación apoya este documento.
- Relacionar los requisitos normativos de la fuente inglesa con la configuración actual de la red.
- Verificar chain ID, validator ID, fee/gas y direcciones peer antes de copiar ejemplos.

## Checklist de uso seguro

- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- No traduzca comandos, config key, nombres RPC, campos JSON ni identificadores de código.
- Antes de copiar ejemplos, adapte chain ID, validator ID, fee/gas y direcciones peer a su red.
- Después de modificar documentación, ejecute `make docs-check` para verificar locale tree y guards de traducción.

## Puntos de atención

- Este documento localizado ayuda a comprender; auditoría, release y seguridad se deciden con la fuente inglesa.
- Si cambia la implementación, actualice la fuente inglesa y todos los documentos localizados en el mismo cambio.

## Interfaces que deben conservarse

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
- `p2p.tls_server_name`
- `start`
- `BanThreshold`
- `MaxScore`

- `validator_id`
- `p2p.node_id`
- `node.key.json`
- `p2p.node_key_path`
- `signature_nonce`
- `node_public_key`
- `signature`
- `Wire Compatibility`
## Estructura de la fuente inglesa

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

## Fuente canónica

- [Documento canónico en inglés](../../en/specs/networking-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Timing de peers y peers permanentes

Un fallo temporal de dial no banea por sí solo a un configured peer o seed. El fallo queda en backoff y diagnóstico; el ban debe venir de evidencia de comportamiento como gossip malicioso, error de autenticación o rate-limit abuse. Ajuste `p2p.dial_timeout` según latencia multi-región y coste TLS/auth.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Transport — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Topics — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Handshake — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Wire Compatibility — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Address Roles — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Peer Scoring — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Reconnect and Backoff — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: DoS/DDOS Defenses — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Operational Signals — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `validator_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.auth_replay_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.dial_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `signature_nonce` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node_public_key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.listen_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.seeds` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `host:port` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `0.0.0.0:26656` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `[::]:26656` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_server_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_server_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.admin_token` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.admin_tokens` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
