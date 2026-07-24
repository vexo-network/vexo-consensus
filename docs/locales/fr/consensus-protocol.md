> Locale: fr · Français

# Présentation du protocole de consensus

Cette page est l'entrée générale de la documentation du consensus Vexo. Les détails normatifs se trouvent dans [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md) et [Transaction Format](./specs/tx-format.md).

## Modèle

Vexo utilise un coeur BFT de style HotStuff avec proposal, vote, quorum certificate(QC), timeout certificate, règle locked-QC et finalité à trois chaînes. Un bloc n'est sûr pour un vote que s'il prolonge le locked QC ou porte un justify QC au moins aussi récent. Les chaînes QC synthétiques ou sautant une hauteur, sans liaison explicite des hauteurs et hachages du bloc, du parent et du grand-parent, sont rejetées avant toute décision de finalité.

## Identité du protocole et limite de recherche

Vexo n'est ni un nouveau nom pour HotStuff non modifié, ni le même protocole ou la même implémentation qu'AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint ou CometBFT. Son runtime Go distinct réutilise les concepts de sûreté de la famille HotStuff et y associe un délai de ronde adaptatif, une récupération durable, un ordre déterministe des transactions, une exécution modulaire et des validator sets versionnés par hauteur.

Le chemin de vote actif emploie le validator set complet de la hauteur et un proposer déterministe. Le sélecteur VRF committee est accessible comme composant et requête, mais ne contrôle pas encore l'éligibilité des proposals ni la formation du quorum. Il doit donc être présenté comme travail futur. Voir [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) pour les contributions et le protocole expérimental.

## Limites d'exécution et de récupération

La certification QC, la finalisation HotStuff, l'exécution applicative et le commit d'état sont des événements distincts. Par défaut, `execution_commit=finalized` n'exécute que l'ancêtre choisi par la règle à trois chaînes. Le pacemaker adaptatif et `recovery_finality_gate_enabled` pilotent le délai et la récupération, sans modifier le proposer, le quorum power, la règle safe-vote ni la finalité.

## Limite de sûreté

- moins d'un tiers du pouvoir de vote byzantin
- signatures de proposition, de vote, de timeout-vote et de finalité séparées par domaine
- liaison de hachage définie par le validateur à la hauteur de preuve pertinente
- signataires connus uniques dans les QC et preuves de finalité
- preuve responsable de l'équivoque du validateur
- rejet des décisions d'engagement contradictoires à la même hauteur finalisée

## Limite cryptographique

- Le backend `deterministic` est réservé aux tests et échoue à la validation network safety.
- `ed25519` convient aux essais de réseau public et à la préparation du lancement.
- `bls` utilise par défaut `blst-bls12381-minpk-v1` et requiert proof-of-possession, contrôles de sous-groupe, validation des clés, audit des dépendances et preuves release-gate.
- Les métadonnées de l'adaptateur VRF sont requises par la validation, sans signifier que VRF committee participe au consensus actif.

- audit de configuration strict pour chaque maison de validateur
- preuve de release-gate
- examen externe de la sécurité
- preuves multi-hôtes à long terme et du chaos
- signataire/KMS preuve de politique
- examen de la politique économique et de gouvernance spécifique à la chaîne

Voir [Security Audit Readiness](./security/audit-readiness.md) et [Release Pipeline](./release/release-pipeline.md) avant de traiter une version comme prête pour la production.
<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garde les termes techniques et les interfaces qui ne doivent pas changer entre la version canonique et la traduction.

### Suivi des sections
- section: Model - HotStuff, finalité en trois chaînes, QC, timeout certificate et locked-QC safety doivent être lus ensemble.
- section: Execution Terms - la différence entre qc certified, finalized, executed et state committed doit rester claire.
- section: Safety Boundary - vérifier le seuil byzantin inférieur à un tiers, la séparation des domaines, le hachage du validator set et les preuves responsables.
- section: Crypto Boundary - conserver les identifiants `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` et `ecvrf-p256-sha256-tai-v1`.
- section: Operational Boundary - lire ensemble `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled`, ainsi que les signaux snapshot/replay.
- `require_network_safety` et `block_committed` doivent rester visibles tels quels dans la traduction.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### Interfaces à conserver
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-pv1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## Notes d'exploitation

Quand vous créez un nouveau domicile de validateur, vérifiez `config.json` avec `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` et `log_config.json`.
En production, `vexo_quorum_health_ratio` et `adaptive_round_timeout_enabled` doivent être observés ensemble, pas séparément.

- `execution_commit=finalized` reste la priorité.
- `qc` ne doit être activé que dans des réseaux de test contrôlés.
- `recovery_finality_gate_enabled` doit être vérifié avec les preuves de snapshot et de replay.
