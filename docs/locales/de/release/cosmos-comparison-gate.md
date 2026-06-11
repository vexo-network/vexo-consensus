# Cosmos/Tendermint Comparison Gate

> Locale: de · Deutsch
> Dieses Dokument ist ein deutsches Begleitdokument zur englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Überblick

Dieses Dokument hilft dabei, das Release-Gate gegenüber Cosmos/Tendermint-ähnlichen Erwartungen zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/de/release/cosmos-comparison-gate.md`

## Warum dieses Dokument lesen

- das Release-Gate gegenüber Cosmos/Tendermint-ähnlichen Erwartungen
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

- `release gate`
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## Struktur der englischen Quelle

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/release/cosmos-comparison-gate.md)
