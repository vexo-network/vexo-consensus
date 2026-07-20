> Locale: es · Español

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| `banned_peers` | Pares actualmente prohibidos por la política de puntuación | Los picos indican ataque, mala configuración de pares o límites demasiado estrictos |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

`vexo_peer_count` se mantiene para los paneles más antiguos. Los nuevos paneles deben mostrar __ VEXO_CODE_1__, __ VEXO_CODE_2__ y __ VEXO_CODE_3__ por separado.

## Reglas de alerta sugeridas

Sintonice los números para el recuento real del validador, el intervalo de bloque, la latencia y el hardware. Estos son puntos de partida, no constantes universales.

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

## Umbrales iniciales sugeridos

Utilice estos como valores de alerta iniciales, luego sintonice después de una línea de base real a largo plazo:

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

La regla más importante: alerta sobre **cambios con el tiempo**. Un solo número puede ser engañoso; la tasa de altura, el retraso en la finalización, la rotación de compañeros, el crecimiento de mempool y las fallas de los firmantes juntos cuentan la historia real.

## Matriz de clasificación de incidentes

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| Pico de compañeros prohibidos | P2P/seguridad | instantáneas de puntajes de compañeros y razones de prohibición | inspeccionar chismes mal formados o configuración incorrecta compartida |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS
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
- `vexo_node_running == 0`
- `vexo_active_peer_count == 0`
- `vexo_snapshot_healthy == 0`
- `vexo_replay_healthy == 0`
- `vexo_validator_signing_failures > 0`
- `vexo_post_commit_reconciliation_failures > 0`
- `timeout_propose`
- `max_txs`
