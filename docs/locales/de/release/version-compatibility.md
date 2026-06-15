# Version Compatibility Matrix

> Locale: de · Deutsch
> Dieses Dokument ist ein deutsches Begleitdokument zur englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.


## Reihenfolge zum Einstieg

Dieses Dokument erklärt den Release- und Betriebsablauf von Version Compatibility. Wenn du neu bist, lies in dieser Reihenfolge.

1. Current Matrix
2. Upgrade Compatibility Checklist
3. Rollback Drill

Diese Reihenfolge entspricht der praktischen Nutzung: zuerst Ziele und Gates, dann Artefakte und Evidence-Anforderungen, und zuletzt die Ausführungsschritte.

## Überblick

Dieses Dokument hilft dabei, Version-Kompatibilitätsmatrix und Upgrade-Entscheidungskriterien zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/release/version-compatibility.md`
- Locale path: `docs/locales/de/release/version-compatibility.md`

## Warum dieses Dokument lesen

- Version-Kompatibilitätsmatrix und Upgrade-Entscheidungskriterien
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

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `/v1/*`
- `vexod upgrade plan --json`
- `vexod upgrade apply`
- `rollback_required`
- `make release-candidate`

## Struktur der englischen Quelle

- Version Compatibility Matrix
- Current Matrix
- Upgrade Compatibility Checklist
- Rollback Drill

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/release/version-compatibility.md)

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Current Matrix — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Upgrade Compatibility Checklist — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Rollback Drill — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `module_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `network_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `consensus_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `mempool_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `log_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/*` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod upgrade plan --json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod upgrade apply` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rollback_required` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `make release-candidate` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
