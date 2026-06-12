# Production Readiness Guide

> Locale: es · Español
> Las decisiones de seguridad y publicación se confirman con la fuente inglesa y el release gate.

## Resumen

Este documento explica qué debe verificarse antes de llamar listo para producción a una red basada en Vexo.

Esta guía localizada mantiene sin cambios los comandos, campos JSON, métodos RPC, claves de configuración y nombres de paquetes para que los ejemplos sigan siendo copiables entre idiomas.

## Por qué importa

Vexo combines BFT consensus, application modules, native accounting, optional EVM execution, validator economics, peer networking, and release evidence. A reader should be able to explain not just that a feature exists, but how to operate it safely and how to prove that it works on the target network.

## Qué verificar

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## Acciones del operador

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

## Nombres de interfaz que no cambian

- `vexod validate --home <home>`
- `vexod config audit --home <home> --strict`
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `peer_count`
- `active_peer_count`
- `configured_peer_count`
- `scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `network_config.json`
- `consensus_config.json`
- `module_config.json`
- `mempool_config.json`
- `release gate`

## Errores comunes

- Do not assume configured peers are connected peers; active sessions must be checked separately.
- Do not call BLS, VRF, EVM, state sync, or governance production-ready without release evidence.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## Referencia normativa

- [Fuente normativa](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: The Short Version — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: How To Use This Guide — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Readiness Levels — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: System Map — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Configuration Review Order — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Consensus and Finality Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Runtime and Storage Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: EVM and Native Coin Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Crypto and Key Custody Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Networking Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Observability Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Evidence Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Common Failure Modes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: What This Guide Does Not Claim — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `docs/specs/consensus-spec.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/specs/finality-proof-format.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `modules/staking` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/specs/validator-lifecycle.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `modules/*` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/sdk/app-module-guide.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/specs/storage-schema.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `modules/bank` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/specs/tx-format.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/specs/evm-native-accounting.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `modules/evm` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/sdk/rpc-api-versioning.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `cmd/vexod keys` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/sdk/custom-crypto-backend.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/security/audit-readiness.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/specs/networking-spec.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/operators/node-initialization.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/release/launch-runbook.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `cmd/vexod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/operators/observability.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/release/release-pipeline.md` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `mempool_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `log_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod validate --home <home>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod config audit --home <home> --strict` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `allow_unsafe_qc_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_prevote` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_precommit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `create_empty_blocks` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getProof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `go.mod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `max_score` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make check` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `v1/status` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `active_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_web3Capabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
