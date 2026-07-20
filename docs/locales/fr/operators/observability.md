> Locale: fr · Français

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| `banned_peers` | Pairs actuellement interdits par la politique de score | Les pics indiquent une attaque, une mauvaise configuration des pairs ou des limites trop strictes |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

`vexo_peer_count` est conservé pour les anciens tableaux de bord. Les nouveaux tableaux de bord doivent représenter `vexo_active_peer_count`, `vexo_configured_peer_count` et `vexo_scored_peer_count` séparément.

## Règles d'alerte suggérées

Réglez les numéros pour le nombre réel de validateurs, l'intervalle de bloc, la latence et le matériel. Ce sont des points de départ, pas des constantes universelles.

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

## Seuils de départ suggérés

Utilisez-les comme valeurs d'alerte initiales, puis ajustez-les après une véritable ligne de base à long terme :

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

La règle la plus importante : alerter sur les **changements dans le temps**. Un seul nombre peut être trompeur ; le taux de taille, le décalage de finalité, le taux de désabonnement des pairs, la croissance du mempool et les échecs des signataires racontent ensemble la vraie histoire.

## Matrice de tri des incidents

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| Pointe des pairs interdits | P2P/sécurité | instantanés des scores des pairs et raisons de l'interdiction | inspecter les commérages mal formés ou la mauvaise configuration partagée |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS
<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe conserve les noms techniques qui doivent rester identiques à la version canonique :

- `rpc_listening` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `p2p_listening` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `peer_configured` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `peer_connected` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `peer_disconnected` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `peer_dial_failed` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `peer_banned` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `consensus_loop_running` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `block_committed` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `round_timeout` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `validator_signing_failure` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `evidence_received` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `evidence_applied` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `snapshot_exported` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `replay_checked` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `upgrade_halt` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `upgrade_applied` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
- `dist/` — Ce nom est conservé tel quel dans les exemples d'exécution et la validation de configuration.
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
