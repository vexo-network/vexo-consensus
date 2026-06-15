# Speicherschema

> Locale: de · Deutsch
> Dieses Dokument ist ein deutsches Begleitdokument zur englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.


## Reihenfolge zum Einstieg

Dieses Dokument erklärt die normative Spezifikation von Storage Schema. Wenn du neu bist, lies in dieser Reihenfolge.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Diese Reihenfolge passt zur Art, wie man das Dokument liest: zuerst Umfang und Zustand, dann Nachrichten-, Safety- und Liveness-Regeln, und am Ende die Beweise.

## Überblick

Dieses Dokument hilft dabei, durable storage namespaces, key schema und recovery marker zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/de/specs/storage-schema.md`

## Warum dieses Dokument lesen

- durable storage namespaces, key schema und recovery marker
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

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Struktur der englischen Quelle

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Scope — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Backend — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Records — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Indexes — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: EVM Records — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Recovery Rules — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Snapshot Validation — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Schema Migration — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `store.Store` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_ethstate` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_getBalance` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_getProof` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bank/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `auth/nonce/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm/code/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm/storage/{0x_address}/{slot}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_ethstate/{height}/meta` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_ethstate/{height}/accounts/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_ethstate/{height}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `seen_ttl` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `code/{address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `storage/{address}/{slot}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `receipts/{tx_hash}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `logs/{address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
