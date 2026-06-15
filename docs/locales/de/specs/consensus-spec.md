# Konsensspezifikation

> Locale: de · Deutsch
> Dieses Dokument ist ein deutsches Begleitdokument zur englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.


## Reihenfolge zum Einstieg

Dieses Dokument erklärt die normative Spezifikation von Consensus Spec. Wenn du neu bist, lies in dieser Reihenfolge.

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

Diese Reihenfolge passt zur Art, wie man das Dokument liest: zuerst Umfang und Zustand, dann Nachrichten-, Safety- und Liveness-Regeln, und am Ende die Beweise.

## Überblick

Dieses Dokument hilft dabei, die normative Spezifikation der Konsens-State-Machine zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/de/specs/consensus-spec.md`

## Warum dieses Dokument lesen

- die normative Spezifikation der Konsens-State-Machine
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
- Nach Änderungen `make docs-check` ausführen, um locale tree und Übersetzungs-Guards zu prüfen.

## Worauf zu achten ist

- Dieses lokalisierte Dokument unterstützt das Verständnis; Audit-, Release- und Sicherheitsentscheidungen erfolgen anhand der englischen Quelle.
- Bei Implementierungsänderungen müssen englische Quelle und alle lokalisierten Dokumente im selben Change aktualisiert werden.

## Unverändert zu behaltende Schnittstellen

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## Struktur der englischen Quelle

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Leere Blöcke und Round-Recovery

Mit `create_empty_blocks=false` ist eine stabile height bei leerem mempool ein normaler idle Zustand. Sobald eine Transaktion ankommt, kann der Node zur nächsten local proposer round wechseln und einen Transaktionsblock bauen; QC/finality Regeln bleiben unverändert.

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Scope — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Roles — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: State — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Message Types — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Safety Rules — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Finality Rule — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Execution Commit Policy — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Liveness Assumptions — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Empty Blocks and Round Recovery — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Evidence — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `chain_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator_set_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `locked_qc` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `high_qc` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `last_timeout_cert` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `last_finalized` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `>= 2/3` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `B3.height = B2.height + 1` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `B2.height = B1.height + 1` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `execution_commit = "qc"` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `execution_commit = "finalized"` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `block_committed` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `create_empty_blocks = false` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `latest_height = 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `latest_height` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `actual_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `actual_time_unix_nano` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `parity_shards` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
