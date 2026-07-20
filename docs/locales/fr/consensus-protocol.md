> Locale: fr · Français

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- moins d'un tiers du pouvoir de vote byzantin
- signatures de proposition, de vote, de timeout-vote et de finalité séparées par domaine
- liaison de hachage définie par le validateur à la hauteur de preuve pertinente
- signataires connus uniques dans les QC et preuves de finalité
- preuve responsable de l'équivoque du validateur
- rejet des décisions d'engagement contradictoires à la même hauteur finalisée

## Limite cryptographique

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

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
