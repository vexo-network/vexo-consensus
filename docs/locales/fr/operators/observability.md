> Locale: fr · Français

# Guide d'observabilité

Ce guide explique comment savoir si un nœud Vexo est sain à partir du RPC, des métriques, des journaux et des preuves de version.

Il est destiné aux opérateurs qui ont besoin de signaux pratiques : que surveiller, que signifie chaque chiffre et quand une valeur doit être considérée comme dangereuse.

## En un coup d'œil

Si un nœud semble erroné, vérifiez-les dans l'ordre :

1. `running` et `latest_height` dans `/v1/status`
2. `latest_finalized_height` et nombre de pairs
3. `round_timeout`, latence de proposition/vote, taille du pool de mémoire et mesures de latence de validation
4. Échecs des signataires, état de santé des instantanés et état de relecture
5. Interdictions par les pairs et échecs de numérotation par les pairs

Cet ordre est important car il sépare « le processus est vivant » de « la chaîne progresse réellement en toute sécurité ».

## Points de terminaison principaux

| Point de terminaison | Utiliser |
|---|---|
| `/v1/status` | Processus rapide, hauteur, hachage de l'application, finalité et résumé des pairs |
| `/v1/metrics` | Métriques JSON pour les tableaux de bord et l'automatisation |
| `/metrics/text` | Métriques de texte compatibles Prometheus |
| `/v1/diagnostics` | Vérifications combinées de la préparation, des capacités, de l'état, des pairs, du stockage et des métriques |
| `/v1/finality/latest` | Dernière preuve de finalité pour les contrôles client léger et de sécurité |
| `/v1/state/latest` | Dernière liaison racine d'état et ensemble de validateurs |
| `/v1/recovery/report` | Diagnostics de cohérence crash/redémarrage |
| `/v1/snapshot` | État de santé des instantanés et exportation des métadonnées |

Les points de terminaison d'administration tels que l'élagage, la relecture et le contrôle de consensus ne doivent normalement être accessibles que via le bouclage, un réseau d'opérateur, mTLS ou une passerelle authentifiée. Les jetons d’administrateur étendus restent facultatifs et sont appliqués une fois configurés.

## Lecture `/v1/status`

Champs importants :

| Champ | Signification | Remarque de l'opérateur |
|---|---|---|
| `running` | Le processus de nœud a démarré et possède l'état d'exécution | `true` ne prouve pas à lui seul la vivacité du consensus |
| `latest_height` | Dernière hauteur d'application validée localement | Doit augmenter avec le temps sur un réseau de validateurs en direct |
| `latest_finalized_height` | Dernière hauteur finalisée HotStuff à trois chaînes | Ne doit pas être en retard indéfiniment sur la hauteur exécutée/engagée |
| `latest_app_hash` | Hachage de validation de l'application | Doit correspondre à des pairs de même hauteur |
| `peer_count` | Résumé des pairs connectés/notés rétrocompatibles | Préférez les champs homologues plus spécifiques ci-dessous |
| `active_peer_count` | Sessions de transport actif, lorsque le transport peut les signaler | Meilleur signal rapide pour la connectivité P2P en direct |
| `configured_peer_count` | Adresses homologues configurées ou apprises | L'accessibilité n'est pas garantie |
| `scored_peer_count` | Pairs connus de la table de score | Utile pour l'historique des bannissements/limites de débit, pas pour la preuve des sessions en direct |
| `banned_peers` | Pairs actuellement bannis par la politique de score | Les pics indiquent une attaque, une mauvaise configuration des homologues ou des limites trop strictes |

Exemple sain pour un réseau à hôte unique à 4 validateurs : `running=true`, `latest_height` en augmentation, `latest_finalized_height` présent, `active_peer_count` proche de `3` et `banned_peers=0`.

## Métriques Prometheus

Le point de terminaison du texte expose des jauges telles que :

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
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

`vexo_peer_count` est conservé pour les anciens tableaux de bord. Les nouveaux tableaux de bord doivent représenter `vexo_active_peer_count`, `vexo_configured_peer_count` et `vexo_scored_peer_count` séparément.

## Règles d'alerte suggérées

Ajustez les chiffres pour le nombre réel de validateurs, l’intervalle de bloc, la latence et le matériel. Ce sont des points de départ et non des constantes universelles.

| Alerte | Condition de départ | Pourquoi |
|---|---|---|
| Nœud en panne | `vexo_node_running == 0` pendant 1 minute | Processus/exécution arrêté |
| Hauteur bloquée | `latest_height` inchangé pendant 2-3 intervalles de bloc attendus | Consensus ou exécution bloquée |
| Finalité bloquée | `latest_finalized_height` inchangé pendant que les blocs continuent de s'exécuter | Chemin de finalité ou problème de quorum |
| Aucun pair actif | `vexo_active_peer_count == 0` pendant 1 minute sur un nœud non isolé | Panne P2P, incompatibilité d'authentification ou problème d'adresse |
| Nombre de pairs trop faible | pairs actifs en dessous de l'objectif de connectivité du quorum | Problème de partition ou de bootstrap |
| Pic de délai d'attente rond | le compteur de délai d'attente augmente plus rapidement que la ligne de base normale | Latence, échec du proposant ou partition réseau |
| Latence de validation élevée | p95/p99 s'approche du budget de délai d'attente consensuel | Surcharge du magasin/d'exécution |
| Pression du pool de mémoire | la taille du pool de mémoire augmente pendant plusieurs minutes | Problème de politique tarifaire, de spam ou de capacité de blocage |
| Instantané malsain | `vexo_snapshot_healthy == 0` | Risque de synchronisation/récupération d'état |
| Replay malsain | `vexo_replay_healthy == 0` | Déterminisme ou risque de cohérence d'état |
| Échecs des signataires | `vexo_validator_signing_failures > 0` | KMS/signataire distant/échec de la stratégie |
| Échecs de la réconciliation | `vexo_post_commit_reconciliation_failures > 0` | Preuve durable ou réparation nécessaire |
| Pic de pairs interdit | pairs interdits augmente soudainement | Attaque, pairs mal configurés ou problème de seuil de score |

## Seuils de démarrage suggérés

Utilisez-les comme valeurs d’alerte initiales, puis ajustez-les en fonction d’une véritable référence à long terme :

| Signalisation | Avertissement | Critique | Première action |
|---|---:|---:|---|
| Taux de taille | en dessous de 50% de la valeur attendue pour 2 fenêtres | croissance nulle pour des intervalles de 2-3 blocs | comparer tous les validateurs, vérifier les journaux des proposants/signatures/pairs |
| Décalage de hauteur finalisé | grandit pendant 5 minutes | grandit tandis que la hauteur exécutée continue d'augmenter pendant 10 minutes | inspecter les journaux de preuve de contrôle de qualité/finalité et le hachage défini par le validateur |
| Pairs actifs | en dessous de l'objectif de connectivité du quorum | zéro pairs actifs | vérifier l'adresse annoncée, TLS/auth, incompatibilité d'ID de genèse/chaîne |
| Temps morts de ronde | 3x ligne de base normale | boucle de délai d'attente continue | augmenter le budget de délai d'attente ou enquêter sur la latence/partition |
| Latence de proposition p95 | supérieur à 50 % de `timeout_propose` | supérieur à 80 % de `timeout_propose` | proposant de profil, mempool, engagement DA, disque |
| Latence des votes p95 | supérieur à 50 % du budget préalable au vote/préengagement | au-dessus de 80% du budget | inspecter le CPU, le signataire, le transport, la contre-pression des potins |
| Latence de validation p95 | au-dessus de 50 % de l'intervalle de bloc | au-dessus de 80 % de l'intervalle de bloc | inspecter LevelDB, les racines d'état, l'exécution EVM, les instantanés |
| Taille de la mémoire | augmentant pendant 5 minutes | proche de `max_txs` ou taux de désabonnement de remplacement soutenu | inspecter les frais de base, les frais minimum, la validité d'émission, le spam |
| Échecs des signataires | toute valeur non nulle | échecs répétés dans une fenêtre de hauteur | arrêter le validateur si une protection à double signe ou une incompatibilité de clé apparaît |
| État de santé instantané | un chèque échoué | échec répété d'exportation/vérification/restauration | suspendre le service de synchronisation d'état et exécuter le rapport de récupération |
| Rejouer la santé | un échec de relecture strict | rejouer l'inadéquation à la dernière hauteur de sécurité | préserver le répertoire des données et arrêter la mise à niveau/version non sécurisée |
| Pairs bannis | pic soudain | de nombreux pairs bannis après le déploiement de la configuration | vérifier les plafonds de score, TLS CA, l'identité des pairs, la preuve d'authentification facultative et le décalage d'horloge |

La règle la plus importante : alerte sur **changement dans le temps**. Un seul chiffre peut être trompeur ; le taux de taille, le décalage de finalité, le taux de désabonnement des pairs, la croissance du pool de mémoire et les échecs des signataires racontent ensemble la véritable histoire.

## Matrice de tri des incidents

| Situation | Couche probable | Que conserver | Prochaine étape sûre |
|---|---|---|---|
| Hauteur arrêtée, pairs en bonne santé | consensus/signataire/exécution | journaux de consensus, journaux de signataires, exemple de pool de mémoire | vérifier la clé du proposant et arrondir les journaux de délai d'attente |
| Pairs supprimés après le déploiement | mise en réseau/configuration | configuration réseau, certificats TLS, carnet d'adresses, journaux des pairs | annuler l'adresse annoncée/TLS/changement d'authentification |
| Les hachages d'application diffèrent à la même hauteur | exécution/stockage | répertoires de données, enregistrements de bloc, journaux d'applications, sortie de relecture | arrêter les nœuds affectés et exécuter une relecture stricte |
| Preuve de finalité rejetée | ensemble de finalité/validateur | preuve JSON, validateur réglé à la hauteur de la preuve | vérifier le hachage défini par le validateur et signer le domaine des octets |
| La restauration d'instantané échoue | synchronisation/stockage de l'état | fichier d'instantané, somme de contrôle, racines d'état, journaux de restauration | ne réessayez pas avec des données en direct ; restaurer dans un répertoire propre |
| Le signataire distant rejette les demandes | garde des clés | journal d'audit des signataires, fichier de garde, fichier occasionnel, journaux de nœuds | distinguer le rejet politique de la panne des transports |
| Pic de pairs bannis | P2P/sécurité | instantanés du score des pairs et raisons d'interdiction | inspecter les potins mal formés ou la mauvaise configuration partagée |

Lors d’incidents, préférez conserver les données plutôt que « nettoyer ». La suppression des WAL, des addrbooks, des signataires de garde ou des répertoires LevelDB peut détruire les preuves nécessaires pour distinguer un bug d'une erreur de l'opérateur.

## Enregistrez les événements à conserver

Les journaux structurés doivent être conservés avec l'ID de nœud, l'ID de validateur, l'ID de chaîne, la hauteur, le tour, le hachage de bloc et l'ID d'homologue, le cas échéant.

Événements importants :

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

Pour les versions candidates, archivez les journaux avec des exemples de métriques, des exemples pprof, des fichiers de configuration, la genèse, des sommes de contrôle binaires et des manifestes de preuves.

## Manuel de première réponse

Lorsqu'un opérateur constate un problème :

1. Vérifiez `/v1/status` sur au moins deux validateurs.
2. Comparez `latest_height`, `latest_finalized_height`, `latest_app_hash` et le nombre de pairs.
3. Vérifiez `/v1/diagnostics` pour les fonctionnalités manquantes ou les contrôles de stockage/relecture/instantanés défectueux.
4. Inspectez les journaux d'événements homologues pour détecter les erreurs d'authentification, TLS, Genesis, ID de chaîne ou d'attente.
5. Inspectez les métriques du pool de mémoire et des frais de base si les taxes ne sont pas incluses.
6. Vérifiez les journaux des signataires et des signataires distants si les signatures du validateur échouent.
7. Exportez le rapport de récupération avant de supprimer ou de modifier des données.
8. Si un conflit de finalité est suspecté, arrêtez l'automatisation, conservez les journaux/preuves et exécutez la détection des conflits de finalité.

## Disposition du tableau de bord

Un tableau de bord utile comporte généralement cinq lignes :

1. **Vivacité** : nœud en cours d'exécution, dernière hauteur, hauteur finalisée, taux de hauteur.
2. **Latence de consensus** : délais d'attente des tours, proposition/vote/commit p95 et p99.
3. **Réseau** : pairs actifs/configurés/notés, pairs bannis, messages de la fenêtre des pairs.
4. **Exécution** : taille du pool de mémoire, frais de gaz/de base, nombre de transmissions, latence de validation.
5. **Récupération et sécurité** : état de santé des instantanés, état de relecture, échecs de signataires, échecs de réconciliation.

Gardez les tableaux de bord ennuyeux. Le but n’est pas d’afficher tous les compteurs internes ; il s'agit de rendre évidents les états dangereux avant que les validateurs ne divergent ou que les utilisateurs ne remarquent des transactions bloquées.

## Libérer les preuves de l'observabilité

Pour une version candidate, l’observabilité n’est pas seulement une surveillance en direct. Cela devient une preuve :

1. Collectez les références `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest` et `/v1/recovery/report` auprès de chaque validateur.
2. Exécutez la charge pendant la durée et le taux choisis.
3. Injectez au moins un redémarrage, une interruption homologue et un exercice d'exportation/vérification/restauration d'instantané.
4. Collectez les métriques finales de chaque validateur.
5. Stockez les échantillons avant/après, les journaux, les échantillons pprof, les journaux d'audit des signataires et le manifeste de preuves dans `dist/`.

Un bon ensemble de preuves permet à un évaluateur de répondre : la taille a-t-elle augmenté, la finalité a-t-elle progressé, les pairs ont-ils récupéré, les txs ont-ils été validés, les instantanés ont-ils été vérifiés, la relecture est-elle restée saine, les signataires ont-ils évité la double signature et le binaire de version exact a-t-il produit les résultats ?

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Core Endpoints — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Reading `/v1/status` — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Prometheus Metrics — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Suggested Alert Rules — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Suggested Starting Thresholds — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Incident Triage Matrix — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Log Events to Keep — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: First Response Playbook — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Dashboard Layout — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Release Evidence From Observability — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `/v1/status` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/metrics` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/metrics/text` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/diagnostics` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/finality/latest` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/state/latest` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/recovery/report` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/snapshot` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `latest_height` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `latest_finalized_height` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `latest_app_hash` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `active_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `configured_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `scored_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `banned_peers` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `banned_peers=0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_node_running` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_latest_height` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_active_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_configured_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_scored_peer_count` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_banned_peers` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_height_rate_per_minute` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_round_timeouts` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_proposal_latency_p95_nanos` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_vote_latency_p95_nanos` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_commit_latency_p95_nanos` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_mempool_size` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_snapshot_healthy` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_replay_healthy` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_validator_signing_failures` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_post_commit_reconciliation_failures` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_node_running == 0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_active_peer_count == 0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_snapshot_healthy == 0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_replay_healthy == 0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_validator_signing_failures > 0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_post_commit_reconciliation_failures > 0` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_propose` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `max_txs` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `node_running` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_listening` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_listening` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `peer_configured` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `peer_connected` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `peer_disconnected` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `peer_dial_failed` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `peer_banned` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `consensus_loop_running` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `block_committed` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `round_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator_signing_failure` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evidence_received` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evidence_applied` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `snapshot_exported` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `replay_checked` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `upgrade_halt` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `upgrade_applied` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `dist/` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
