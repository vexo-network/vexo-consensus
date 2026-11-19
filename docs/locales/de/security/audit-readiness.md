# Vorbereitung auf das Sicherheitsaudit

> Locale: de · Deutsch
> Dieses Dokument ist eine direkte deutsche Übersetzung der englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Überblick

Dieses Dokument hilft dabei, Threat Model, Sicherheitsannahmen und Audit-Nachweise zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/de/security/audit-readiness.md`

## Warum dieses Dokument lesen

- Threat Model, Sicherheitsannahmen und Audit-Nachweise
- Prüfe zuerst MUST/SHOULD/MAY-Sätze in der englischen Quelle.
- Dieses lokalisierte Dokument unterstützt das Verständnis; Audit-, Release- und Sicherheitsentscheidungen erfolgen anhand der englischen Quelle.

## Was danach möglich sein sollte

- Erklären, welche Implementierungs- oder Betriebsentscheidung dieses Dokument unterstützt.
- Normative Anforderungen der englischen Quelle mit der aktuellen Netzwerkkonfiguration verbinden.
- Vor dem Kopieren von Beispielen chain ID, validator ID, fee/gas und Peer-Adressen prüfen.

## Checkliste für sichere Nutzung

- Prüfe zuerst MUST/SHOULD/MAY-Sätze in der englischen Quelle.
- Übersetze keine Befehle, config key, RPC-Namen, JSON-Felder oder Code-Bezeichner.
- Passe Beispielwerte vor dem Kopieren an chain ID, validator ID, fee/gas und Peer-Adressen deines Netzwerks an.
- Nach Änderungen `make docs-check` ausführen, um den lokalen Dokumentbaum und die Übersetzungs-Guards zu prüfen.

## Worauf zu achten ist

- Dieses lokalisierte Dokument unterstützt das Verständnis; Audit-, Release- und Sicherheitsentscheidungen erfolgen anhand der englischen Quelle.
- Bei Implementierungsänderungen müssen englische Quelle und alle lokalisierten Dokumente im selben Change aktualisiert werden.

## Unverändert zu behaltende Schnittstellen

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## Struktur der englischen Quelle

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Sicherheitsziele
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

Audit-Unterlagen müssen neben BLS auch VRF adapter audit evidence enthalten. Pinne den SHA-256 einer Datei wie `docs/security/ecvrf-audit-evidence.json` in `vrf.audit_evidence_sha256` oder `--vrf-audit-sha256` und prüfe dependency audit, key custody, TLS/mTLS oder pinned CA, auth, replay defense und service availability gemeinsam.

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/security/audit-readiness.md)

## Mempool-WAL und Konsens-WAL

Jedes angenommene Tx- oder Batch-Ereignis ist dauerhaft gespeichert, bevor seine Einreichung zurückkehrt. Datensätze sind `uvarint(length) | op | payload | CRC-32C`; gleichzeitige Aufnahmen teilen sich ein fsync (Group Commit), und der Knoten wartet darauf, ohne die Aufnahmesperre zu halten. Das Replay schneidet einen abgerissenen oder beschädigten letzten Datensatz ab und lehnt ein vor seinem Ende beschädigtes Log ab (`ErrWALCorrupt`). Nach einem I/O-Fehler stoppen Anhänge, bis das Log aus dem Pool neu geschrieben ist. Das Konsens-WAL nutzt dasselbe Framing (4-Byte-Länge big-endian, Payload, 4-Byte-CRC-32C): Ein abgerissenes Ende wird abgeschnitten, Beschädigung in der Mitte abgelehnt.

## Group-Records im Consensus WAL und Dauerhaftigkeit der Mempool-Entfernung

Ein Vote und der Safety State, in den er die Maschine versetzt, bilden einen Group-Record, der mit einem fsync geschrieben und bei der Wiederherstellung ganz oder gar nicht übernommen wird. Ein Proposal-Record kostet keinen eigenen fsync und wird mit dem Vote des Proposers geschrieben, bevor das Proposal den Node verlässt. Der Mempool-Entfernungs-Record eines committeten Blocks ist durable, bevor der nächste Block committet wird; ein Neustart wiederholt die Entfernung für den zuletzt committeten Block.

## Periodischer Sync des Mempool-WAL

Das Mempool-WAL schreibt jede Änderung in das Log und synct es alle 100 ms (`walSyncInterval`). Eine Admission wartet auf den Write, nicht auf einen Sync. Ein Absturz des Prozesses behält jeden Record; ein Absturz der Maschine verliert höchstens die Admissions des letzten Intervalls, die ihre Clients erneut einreichen. Der Entfernungs-Record eines committeten Blocks wird vor dem nächsten Commit gesynct (`WaitDurable`).

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Scope — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Threat Model — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Security Assumptions — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Known Limitations — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Formal-ish Safety Argument — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Required Evidence for Audit — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Auditor Focus Areas — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Practical Audit Walkthrough — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Remote Signer Audit Notes — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: EVM/Web3 Audit Notes — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Snapshot and WAL Audit Notes — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `docs/security/blst-audit-evidence.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `remote-vrf-http-v1` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod keys serve-vrf` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `release collect-evidence` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/*` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `chain_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `go.mod` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/recovery/report` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `da_mismatch` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `missing_data` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator_set_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `app_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `finality.LightClient` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod light verify` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `next_validator_set_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `parent_linkage` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `consensus.MaxBlockBytes`
