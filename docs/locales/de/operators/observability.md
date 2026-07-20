> Locale: de · Deutsch

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| `banned_peers` | Peers, die derzeit durch die Score-Richtlinie gesperrt sind | Spikes deuten auf einen Angriff, eine schlechte Peer-Konfiguration oder zu strenge Limits hin |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

`vexo_peer_count` wird für ältere Dashboards aufbewahrt. Neue Dashboards sollten `vexo_active_peer_count`, `vexo_configured_peer_count` und `vexo_scored_peer_count` separat darstellen.

## Vorgeschlagene Warnungsregeln

Stellen Sie die Zahlen für die tatsächliche Validatoranzahl, das Blockintervall, die Latenz und die Hardware ein. Dies sind Ausgangspunkte, keine universellen Konstanten.

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

## Empfohlene Startschwellen

Verwenden Sie diese als anfängliche Warnwerte und stimmen Sie sie dann nach einer echten langfristigen Basislinie ab:

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

Die wichtigste Regel: Warnung bei **Veränderung im Laufe der Zeit**. Eine einzelne Zahl kann irreführend sein; Größenrate, Endgültigkeitsverzögerung, Peer-Abwanderung, Mempool-Wachstum und Unterzeichnerfehler erzählen zusammen die wahre Geschichte.

## Vorfall-Triage-Matrix

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| Verbotener Peers-Spike | P2P/Sicherheit | Peer-Score-Snapshots und Verbotsgründe | Ungeformten Klatsch oder geteilte falsche Konfiguration überprüfen |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS
<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

In diesem Anhang werden technische Namen festgehalten, die mit der kanonischen Version identisch bleiben müssen:

- `rpc_listening` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `p2p_listening` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `peer_configured` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `peer_connected` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `peer_disconnected` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `peer_dial_failed` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `peer_banned` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `consensus_loop_running` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `block_committed` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `round_timeout` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `validator_signing_failure` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `evidence_received` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `evidence_applied` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `snapshot_exported` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `replay_checked` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `upgrade_halt` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `upgrade_applied` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
- `dist/` — Dieser Name wird in Laufzeitbeispielen und der Konfigurationsprüfung unverändert verwendet.
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
