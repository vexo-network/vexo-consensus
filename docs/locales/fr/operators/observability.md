> Locale: fr · Français

# Guide d'observabilité

Ce guide explique comment évaluer la santé d'un noeud Vexo à partir de RPC, des métriques, des logs et des preuves de release. Vérifiez dans l'ordre `running` et `latest_height`, puis `latest_finalized_height` et les peers actifs, les latences et `round_timeout`, enfin le signer, les snapshots, le replay et les bannissements. Un processus vivant ne prouve pas que le consensus progresse en sûreté.

## Endpoints et statut

Utilisez `/v1/status` pour le résumé de hauteur, app hash, finalité et peers; `/v1/metrics` pour JSON; `/metrics/text` pour Prometheus; `/v1/diagnostics` pour readiness; `/v1/finality/latest`, `/v1/state/latest`, `/v1/recovery/report` et `/v1/snapshot` pour les preuves et la récupération. Les endpoints administratifs doivent rester derrière loopback, un réseau opérateur, mTLS ou une gateway authentifiée.

Dans `/v1/status`, `running=true` signifie seulement que le runtime a démarré. `latest_height` et `latest_finalized_height` doivent progresser, `latest_app_hash` doit correspondre entre peers à la même hauteur, et `active_peer_count` représente mieux les sessions réelles que les seuls peers configurés ou scorés.

| `banned_peers` | Pairs actuellement interdits par la politique de score | Les pics indiquent une attaque, une mauvaise configuration des pairs ou des limites trop strictes |

## Métriques Prometheus

Suivez `vexo_node_running`, `vexo_latest_height`, `vexo_active_peer_count`, `vexo_configured_peer_count`, `vexo_quorum_health_ratio`, `vexo_height_rate_per_minute`, `vexo_round_timeouts`, `vexo_adaptive_round_timeout_nanos`, les p95 proposal/vote/commit, `vexo_mempool_size`, `vexo_snapshot_healthy`, `vexo_replay_healthy`, `vexo_validator_signing_failures` et `vexo_recovery_finality_deferrals`.

`vexo_peer_count` est conservé pour les anciens tableaux de bord. Les nouveaux tableaux de bord doivent représenter `vexo_active_peer_count`, `vexo_configured_peer_count` et `vexo_scored_peer_count` séparément.

## Règles d'alerte suggérées

Réglez les numéros pour le nombre réel de validateurs, l'intervalle de bloc, la latence et le matériel. Ce sont des points de départ, pas des constantes universelles.

| Alerte | Condition initiale | Action |
|---|---|---|
| Hauteur bloquée | aucune progression pendant 2 ou 3 intervalles attendus | comparer tous les validateurs, proposer, signer et peers |
| Finalité bloquée | l'exécution avance mais pas la hauteur finalized | vérifier QC, preuve de finalité et validator-set hash |
| Aucun peer actif | `vexo_active_peer_count == 0` pendant une minute | vérifier adresse, identité, auth et chain ID |
| Quorum faible | `vexo_quorum_health_ratio < 0.75` plusieurs fenêtres | rechercher partition, latence et perte de peers |
| Timeout élevé | le compteur ou le timeout adaptatif dépasse la baseline | examiner réseau, proposer, CPU, disque et signer |
| Récupération différée | `vexo_recovery_finality_deferrals` augmente | lancer le rapport de récupération avant toute suppression |

## Seuils de départ suggérés

Utilisez-les comme valeurs d'alerte initiales, puis ajustez-les après une véritable ligne de base à long terme :

| Signal | Avertissement | Critique |
|---|---|---|
| Débit de hauteur | moins de 50 % de la baseline | aucune croissance |
| Peers actifs | sous la cible du quorum | zéro peer |
| Latence p95 | plus de 50 % du budget | plus de 80 % |
| Signer | toute erreur | erreurs répétées dans la même hauteur |
| Snapshot ou replay | un contrôle échoue | échec répété ou divergence |

La règle la plus importante : alerter sur les **changements dans le temps**. Un seul nombre peut être trompeur ; le taux de taille, le décalage de finalité, le taux de désabonnement des pairs, la croissance du mempool et les échecs des signataires racontent ensemble la vraie histoire.

## Matrice de tri des incidents

| Situation | Couche probable | Étape sûre |
|---|---|---|
| Hauteur arrêtée avec peers sains | consensus, signer ou runtime | conserver logs et vérifier proposer/timeout |
| Peers perdus après déploiement | réseau ou config | conserver config et revenir sur le changement d'adresse/auth |
| App hashes différents | exécution ou stockage | arrêter les noeuds concernés et exécuter un replay strict |
| Preuve de finalité rejetée | finalité ou validator set | vérifier hauteur, hash du set et domaine de signature |
| Snapshot non restaurable | state sync ou stockage | restaurer dans un répertoire propre |
| Remote signer refuse | custody ou policy | distinguer rejet de politique et panne de transport |

| Pointe des pairs interdits | P2P/sécurité | instantanés des scores des pairs et raisons de l'interdiction | inspecter les commérages mal formés ou la mauvaise configuration partagée |

Pendant un incident, préservez WAL, addrbook, signer guard, data directory, configs et logs. Leur suppression détruit les éléments qui permettent de distinguer un bug d'une erreur opérateur.

## Logs et première réponse

Conservez les événements structurés avec node ID, validator ID, chain ID, height, round, block hash et peer ID. Les événements essentiels incluent `peer_connected`, `peer_dial_failed`, `block_committed`, `round_timeout`, `validator_signing_failure`, `snapshot_exported`, `replay_checked`, `upgrade_halt` et `upgrade_applied`.

Comparez `/v1/status` sur au moins deux validateurs, puis `/v1/diagnostics`, les logs peers, le mempool et les métriques de fees, le signer et enfin `/v1/recovery/report`. Archivez les métriques, pprof, configs, genesis, checksums du binaire et manifestes de preuve avec les logs de chaque release candidate.
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
