# Guía de observabilidad

> Locale: es · Español
> Las decisiones de seguridad y publicación se confirman con la fuente inglesa y el release gate.

## Resumen

Este documento explica cómo evaluar la salud de un nodo Vexo con estado, métricas, registros y alertas.

Esta guía localizada mantiene sin cambios los comandos, campos JSON, métodos RPC, claves de configuración y nombres de paquetes para que los ejemplos sigan siendo copiables entre idiomas.

## Por qué importa

Esta guía explica cómo saber si un nodo Vexo está sano a partir de RPC, métricas, registros y evidencia de release.

## Qué verificar

- **Height and finality**: `latest_height`, `latest_finalized_height`, height rate, and finality proof availability show whether consensus and execution are progressing.
- **Peer health**: `peer_count` is compatibility summary; prefer `active_peer_count`, `configured_peer_count`, and `scored_peer_count` to separate live sessions from configured addresses.
- **Latency and timeout**: `round_timeouts`, proposal latency, vote latency, and commit latency show whether timeout values still fit the real network.
- **Execution pressure**: `mempool_size`, gas/base-fee behavior, tx count, and commit p95/p99 show whether block capacity and storage are under pressure.
- **Recovery readiness**: `snapshot_healthy`, `replay_healthy`, recovery reports, and state-root checks show whether a node can safely restart or sync.
- **Custody and safety**: `validator_signing_failures`, remote signer logs, ban spikes, and reconciliation failures require immediate operator review.

## Acciones del operador

- **Status flow**: Start with `/v1/status`, then compare `/v1/metrics`, `/metrics/text`, `/v1/diagnostics`, `/v1/finality/latest`, and recovery reports.
- **Alert flow**: Alert on stalled height, stalled finality, zero active peers, timeout spikes, high commit latency, mempool pressure, replay failure, and signer failures.
- **Incident flow**: Preserve logs, metrics, configs, genesis, binary hash, and evidence files before deleting data or restarting repeatedly.

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

- No asuma que los pares configurados están conectados; las sesiones activas deben comprobarse por separado.
- No declare BLS, VRF, EVM, state sync o gobernanza listos para producción sin evidencia de release.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## Referencia normativa

- [Fuente normativa](../../en/operators/observability.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Core Endpoints — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Reading `/v1/status` — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Prometheus Metrics — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Suggested Alert Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Suggested Starting Thresholds — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Incident Triage Matrix — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Log Events to Keep — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: First Response Playbook — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Dashboard Layout — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Evidence From Observability — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `/v1/status` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/metrics` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/metrics/text` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/diagnostics` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/state/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/recovery/report` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/snapshot` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_finalized_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_app_hash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `active_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `configured_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `scored_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `banned_peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `banned_peers=0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_node_running` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_active_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_configured_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_scored_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_banned_peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_height_rate_per_minute` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_round_timeouts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_proposal_latency_p95_nanos` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_vote_latency_p95_nanos` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_commit_latency_p95_nanos` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_mempool_size` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_snapshot_healthy` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_replay_healthy` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_validator_signing_failures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_post_commit_reconciliation_failures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_node_running == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_active_peer_count == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_snapshot_healthy == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_replay_healthy == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_validator_signing_failures > 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_post_commit_reconciliation_failures > 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `max_txs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node_running` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_listening` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_listening` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_configured` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_connected` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_disconnected` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_dial_failed` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_banned` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_loop_running` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `block_committed` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `round_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator_signing_failure` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evidence_received` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evidence_applied` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `snapshot_exported` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `replay_checked` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `upgrade_halt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `upgrade_applied` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `dist/` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
