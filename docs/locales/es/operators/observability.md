> Locale: es · Español

# Guía de observabilidad

Esta guía explica cómo evaluar la salud de un nodo Vexo mediante RPC, métricas, logs y evidencia de release. Revise primero `running` y `latest_height`; después `latest_finalized_height` y peers activos; luego latencias y `round_timeout`; por último signer, snapshots, replay y bans. Que el proceso esté vivo no demuestra progreso seguro del consenso.

## Endpoints y estado

Use `/v1/status` para altura, app hash, finalidad y peers; `/v1/metrics` para JSON; `/metrics/text` para Prometheus; `/v1/diagnostics` para readiness; y `/v1/finality/latest`, `/v1/state/latest`, `/v1/recovery/report`, `/v1/snapshot` para pruebas y recuperación. Los endpoints administrativos deben quedar detrás de loopback, red de operadores, mTLS o gateway autenticado.

En `/v1/status`, `running=true` solo indica que el runtime inició. `latest_height` y `latest_finalized_height` deben avanzar, `latest_app_hash` debe coincidir entre peers a la misma altura y `active_peer_count` representa mejor las sesiones reales que los peers configurados o puntuados.

| `banned_peers` | Pares actualmente prohibidos por la política de puntuación | Los picos indican ataque, mala configuración de pares o límites demasiado estrictos |

## Métricas Prometheus

Supervise `vexo_node_running`, `vexo_latest_height`, `vexo_active_peer_count`, `vexo_configured_peer_count`, `vexo_quorum_health_ratio`, `vexo_height_rate_per_minute`, `vexo_round_timeouts`, `vexo_adaptive_round_timeout_nanos`, p95 de proposal/vote/commit, `vexo_mempool_size`, `vexo_snapshot_healthy`, `vexo_replay_healthy`, `vexo_validator_signing_failures` y `vexo_recovery_finality_deferrals`.

`vexo_peer_count` se mantiene para los paneles más antiguos. Los nuevos paneles deben mostrar `vexo_active_peer_count`, `vexo_configured_peer_count` y `vexo_scored_peer_count` por separado.

## Reglas de alerta sugeridas

Sintonice los números para el recuento real del validador, el intervalo de bloque, la latencia y el hardware. Estos son puntos de partida, no constantes universales.

| Alerta | Condición inicial | Acción |
|---|---|---|
| Altura detenida | sin avance durante 2 o 3 intervalos | comparar validadores, proposer, signer y peers |
| Finalidad detenida | ejecución avanza pero finalized height no | revisar QC, prueba y validator-set hash |
| Sin peers activos | `vexo_active_peer_count == 0` un minuto | revisar dirección, identidad, auth y chain ID |
| Quorum bajo | `vexo_quorum_health_ratio < 0.75` varias ventanas | investigar partición, latencia y pérdida de peers |
| Timeout alto | contador o timeout adaptativo sobre baseline | revisar red, proposer, CPU, disco y signer |
| Recovery diferido | aumenta `vexo_recovery_finality_deferrals` | exportar reporte antes de modificar datos |

## Umbrales iniciales sugeridos

Utilice estos como valores de alerta iniciales, luego sintonice después de una línea de base real a largo plazo:

| Señal | Advertencia | Crítica |
|---|---|---|
| Tasa de altura | menos de 50 % del baseline | crecimiento cero |
| Peers activos | bajo objetivo de quorum | cero peers |
| Latencia p95 | más de 50 % del presupuesto | más de 80 % |
| Signer | cualquier error | errores repetidos en una altura |
| Snapshot o replay | falla una comprobación | fallo repetido o divergencia |

La regla más importante: alerta sobre **cambios con el tiempo**. Un solo número puede ser engañoso; la tasa de altura, el retraso en la finalización, la rotación de compañeros, el crecimiento de mempool y las fallas de los firmantes juntos cuentan la historia real.

## Matriz de clasificación de incidentes

| Situación | Capa probable | Paso seguro |
|---|---|---|
| Altura detenida con peers sanos | consenso, signer o runtime | conservar logs y revisar proposer/timeout |
| Peers perdidos tras deploy | red o config | conservar config y revertir dirección/auth |
| App hashes distintos | ejecución o storage | detener nodos afectados y ejecutar strict replay |
| Prueba de finalidad rechazada | finalidad o validator set | comprobar altura, set hash y dominio de firma |
| Snapshot no restaura | state sync o storage | restaurar en directorio limpio |
| Remote signer rechaza | custody o policy | separar rechazo de política de fallo de transporte |

| Pico de compañeros prohibidos | P2P/seguridad | instantáneas de puntajes de compañeros y razones de prohibición | inspeccionar chismes mal formados o configuración incorrecta compartida |

Durante un incidente preserve WAL, addrbook, signer guard, data directory, configs y logs. Borrarlos destruye la evidencia para distinguir un bug de un error operativo.

## Logs y primera respuesta

Los eventos estructurados deben incluir node ID, validator ID, chain ID, height, round, block hash y peer ID. Conserve `peer_connected`, `peer_dial_failed`, `block_committed`, `round_timeout`, `validator_signing_failure`, `snapshot_exported`, `replay_checked`, `upgrade_halt` y `upgrade_applied`.

Compare `/v1/status` en al menos dos validadores, después `/v1/diagnostics`, logs de peers, mempool y fees, signer y finalmente `/v1/recovery/report`. Archive métricas, pprof, configs, genesis, checksums binarios y manifests de evidencia junto con los logs del release candidate.
<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice conserva nombres técnicos que deben permanecer iguales a la versión canónica:

- `rpc_listening` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `p2p_listening` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `peer_configured` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `peer_connected` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `peer_disconnected` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `peer_dial_failed` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `peer_banned` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `consensus_loop_running` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `block_committed` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `round_timeout` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `validator_signing_failure` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `evidence_received` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `evidence_applied` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `snapshot_exported` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `replay_checked` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `upgrade_halt` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `upgrade_applied` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `dist/` — Este nombre se usa sin cambios en los ejemplos de ejecución y en la validación de configuración.
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `/v1/snapshot`
- `configured_peer_count`
- `scored_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `latest_app_hash`
- `banned_peers=0`
- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`
- `vexo_adaptive_round_timeout_enabled`
- `vexo_adaptive_round_timeout_nanos`
- `vexo_quorum_health_ratio`
- `vexo_recovery_finality_gate_enabled`
- `vexo_recovery_finality_deferrals`
- `vexo_node_running == 0`
- `vexo_active_peer_count == 0`
- `vexo_adaptive_round_timeout_enabled == 0`
- `vexo_quorum_health_ratio < 0.75`
- `vexo_recovery_finality_gate_enabled == 0`
- `vexo_snapshot_healthy == 0`
- `vexo_replay_healthy == 0`
- `vexo_validator_signing_failures > 0`
- `vexo_post_commit_reconciliation_failures > 0`
- `timeout_propose`
- `max_txs`
