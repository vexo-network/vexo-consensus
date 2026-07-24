> Locale: de · Deutsch

# Observability-Leitfaden

Dieser Leitfaden erklärt, wie die Gesundheit eines Vexo-Knotens anhand von RPC, Metriken, Logs und Release-Nachweisen beurteilt wird. Prüfen Sie zuerst `running` und `latest_height`, danach `latest_finalized_height` und aktive Peers, dann Latenzen und `round_timeout`, zuletzt Signer, Snapshots, Replay und Bans. Ein laufender Prozess beweist keinen sicheren Konsensfortschritt.

## Endpoints und Status

`/v1/status` liefert Höhe, app hash, Finalität und Peers; `/v1/metrics` JSON-Metriken; `/metrics/text` Prometheus; `/v1/diagnostics` Readiness; `/v1/finality/latest`, `/v1/state/latest`, `/v1/recovery/report` und `/v1/snapshot` Beweise und Recovery-Daten. Admin-Endpoints gehören hinter Loopback, Operatornetz, mTLS oder authentifiziertes Gateway.

In `/v1/status` bedeutet `running=true` nur, dass die Runtime läuft. `latest_height` und `latest_finalized_height` müssen fortschreiten, `latest_app_hash` muss bei gleicher Höhe übereinstimmen, und `active_peer_count` ist für reale Sessions aussagekräftiger als konfigurierte oder nur gescorte Peers.

| `banned_peers` | Peers, die derzeit durch die Score-Richtlinie gesperrt sind | Spikes deuten auf einen Angriff, eine schlechte Peer-Konfiguration oder zu strenge Limits hin |

## Prometheus-Metriken

Überwachen Sie `vexo_node_running`, `vexo_latest_height`, `vexo_active_peer_count`, `vexo_configured_peer_count`, `vexo_quorum_health_ratio`, `vexo_height_rate_per_minute`, `vexo_round_timeouts`, `vexo_adaptive_round_timeout_nanos`, proposal/vote/commit p95, `vexo_mempool_size`, `vexo_snapshot_healthy`, `vexo_replay_healthy`, `vexo_validator_signing_failures` und `vexo_recovery_finality_deferrals`.

`vexo_peer_count` wird für ältere Dashboards aufbewahrt. Neue Dashboards sollten `vexo_active_peer_count`, `vexo_configured_peer_count` und `vexo_scored_peer_count` separat darstellen.

## Vorgeschlagene Warnungsregeln

Stellen Sie die Zahlen für die tatsächliche Validatoranzahl, das Blockintervall, die Latenz und die Hardware ein. Dies sind Ausgangspunkte, keine universellen Konstanten.

| Alarm | Startbedingung | Aktion |
|---|---|---|
| Höhe steht | keine Änderung für 2 bis 3 erwartete Intervalle | alle Validatoren, proposer, signer und peers vergleichen |
| Finalität steht | Ausführung steigt, finalized height nicht | QC, Finalitätsnachweis und validator-set hash prüfen |
| Keine aktiven Peers | `vexo_active_peer_count == 0` eine Minute | Adresse, Identität, auth und chain ID prüfen |
| Schwaches Quorum | `vexo_quorum_health_ratio < 0.75` mehrere Fenster | Partition, Latenz und Peer-Verlust untersuchen |
| Hohes Timeout | Zähler oder adaptives Timeout über Baseline | Netzwerk, proposer, CPU, Disk und signer untersuchen |
| Recovery-Aufschub | `vexo_recovery_finality_deferrals` steigt | Recovery-Bericht vor Änderungen exportieren |

## Empfohlene Startschwellen

Verwenden Sie diese als anfängliche Warnwerte und stimmen Sie sie dann nach einer echten langfristigen Basislinie ab:

| Signal | Warnung | Kritisch |
|---|---|---|
| Höhenrate | unter 50 % der Baseline | kein Wachstum |
| Aktive Peers | unter Quorumziel | null Peers |
| p95-Latenz | über 50 % des Budgets | über 80 % |
| Signer | jeder Fehler | wiederholte Fehler in einer Höhe |
| Snapshot oder Replay | eine Prüfung fehlschlägt | wiederholter Fehler oder Divergenz |

Die wichtigste Regel: Warnung bei **Veränderung im Laufe der Zeit**. Eine einzelne Zahl kann irreführend sein; Größenrate, Endgültigkeitsverzögerung, Peer-Abwanderung, Mempool-Wachstum und Unterzeichnerfehler erzählen zusammen die wahre Geschichte.

## Vorfall-Triage-Matrix

| Situation | Wahrscheinliche Ebene | Sicherer Schritt |
|---|---|---|
| Höhe steht bei gesunden Peers | Konsens, Signer oder Runtime | Logs sichern und proposer/timeout prüfen |
| Peers nach Deployment verloren | Netzwerk oder Config | Config sichern und Adresse/auth zurückrollen |
| Verschiedene app hashes | Ausführung oder Storage | betroffene Knoten stoppen und strict replay ausführen |
| Finalitätsnachweis abgelehnt | Finalität oder validator set | Höhe, Set-Hash und Signaturdomäne prüfen |
| Snapshot nicht wiederherstellbar | State Sync oder Storage | in sauberes Verzeichnis restaurieren |
| Remote signer lehnt ab | Custody oder Policy | Policy-Ablehnung von Transportausfall trennen |

| Verbotener Peers-Spike | P2P/Sicherheit | Peer-Score-Snapshots und Verbotsgründe | Ungeformten Klatsch oder geteilte falsche Konfiguration überprüfen |

Während eines Incidents müssen WAL, Addrbook, Signer Guard, Data Directory, Configs und Logs erhalten bleiben. Löschen vernichtet Beweise, die einen Bug von einem Bedienfehler unterscheiden.

## Logs und Erstreaktion

Strukturierte Ereignisse enthalten node ID, validator ID, chain ID, height, round, block hash und peer ID. Wichtig sind `peer_connected`, `peer_dial_failed`, `block_committed`, `round_timeout`, `validator_signing_failure`, `snapshot_exported`, `replay_checked`, `upgrade_halt` und `upgrade_applied`.

Vergleichen Sie `/v1/status` auf mindestens zwei Validatoren, danach `/v1/diagnostics`, Peer-Logs, Mempool und Fee-Metriken, Signer und schließlich `/v1/recovery/report`. Für Release Candidates werden Metriken, pprof, Configs, Genesis, Binär-Checksums und Evidence Manifests zusammen mit den Logs archiviert.
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
