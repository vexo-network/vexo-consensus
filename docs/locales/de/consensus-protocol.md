> Locale: de · Deutsch

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- weniger als ein Drittel der byzantinischen Stimmrechte
- Domain-getrennte Unterschriften für Vorschlag, Abstimmung, Timeout-Abstimmung und Finalität
- Validator-Set-Hash-Bindung in der jeweiligen Proofhöhe
- eindeutige bekannte Unterzeichner in QCs und Finalitätsnachweisen
- rechenschaftspflichtige Beweise für die Äquivokation des Validators
- Ablehnung widersprüchlicher Commit-Entscheidungen in der gleichen endgültigen Höhe

## Krypto-Grenze

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- strenges Konfigurations-Audit für jede Unterkunft des Validierers
- Release-Gate-Nachweis
- externe Sicherheitsüberprüfung
- Langfristige Beweise für mehrere Hosts und Chaos
- Unterzeichner/KMS-Richtliniennachweis
- kettenspezifische Überprüfung der Wirtschafts- und Governance-Politik

Sehen Sie sich [Security Audit Readiness](./security/audit-readiness.md) und [Release Pipeline](./release/release-pipeline.md) an, bevor Sie eine Version als produktionsbereit behandeln.
<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang hält die technischen Begriffe und Schnittstellen fest, die zwischen kanonischer Version und Übersetzung unverändert bleiben müssen.

### Abschnitts-Tracking
- section: Model - HotStuff, Drei-Ketten-Finalität, QC, Timeout-Certificate und Locked-QC-Sicherheit müssen zusammen gelesen werden.
- section: Execution Terms - der Unterschied zwischen qc certified, finalized, executed und state committed bleibt entscheidend.
- section: Safety Boundary - der Byzantine-Anteil unter einem Drittel, Domain Separation, Validator-Set-Hash-Bindung und accountable evidence müssen geprüft werden.
- section: Crypto Boundary - die Bezeichner `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` und `ecvrf-p256-sha256-tai-v1` bleiben stabil.
- section: Operational Boundary - `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` sowie Snapshot-/Replay-Signale gemeinsam auswerten.
- `require_network_safety` und `block_committed` müssen in der Übersetzung sichtbar bleiben.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### Zu erhaltende Schnittstellen
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
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## Betriebsnotizen

Beim Anlegen eines neuen Validator-Home sollten `config.json` zusammen mit `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` und `log_config.json` geprüft werden.
Im Betrieb sollten `vexo_quorum_health_ratio` und `adaptive_round_timeout_enabled` immer gemeinsam beobachtet werden.

- `execution_commit=finalized` hat Vorrang.
- `qc` sollte nur in kontrollierten Testnetzen aktiviert werden.
- `recovery_finality_gate_enabled` sollte mit Snapshot- und Replay-Nachweisen geprüft werden.
