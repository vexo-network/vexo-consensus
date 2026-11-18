# Transaktionsformat

> Locale: de · Deutsch
> Dieses Dokument ist eine direkte deutsche Übersetzung der englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.


## Reihenfolge zum Einstieg

Dieses Dokument erklärt die normative Spezifikation von Tx Format. Wenn du neu bist, lies in dieser Reihenfolge.

1. Scope
2. Canonical Payload
3. Address Format
4. Signed Envelope
5. Required Ante Metadata
6. CheckTx Requirements
7. Fee and Gas
8. Load Test Payloads
9. CLI Examples

Diese Reihenfolge passt zur Art, wie man das Dokument liest: zuerst Umfang und Zustand, dann Nachrichten-, Safety- und Liveness-Regeln, und am Ende die Beweise.

## Überblick

Dieses Dokument hilft dabei, transaction format, signing, fee und gas-Regeln zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/de/specs/tx-format.md`

## Warum dieses Dokument lesen

- transaction format, signing, fee und gas-Regeln
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

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## Struktur der englischen Quelle

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/specs/tx-format.md)

## Binärer signierter Umschlag

Eine signierte Transaktion ist binär (`txcodec.EncodeEnvelope`): das Zwei-Byte-Präfix `0x89 'V'` (`txcodec.SignedPrefix`), das Versionsbyte `1` und vier Felder mit vorangestellter uvarint-Länge: Chain-ID (höchstens `txcodec.MaxChainIDLength`, 128 Byte), öffentlicher Schlüssel (höchstens `txcodec.MaxPublicKeyLength`), Signatur (höchstens `txcodec.MaxSignatureLength`) und Payload. Der Decoder lehnt nicht minimale Längen, zu große Felder, unbekannte Versionen und überzählige Bytes ab, sodass eine signierte Transaktion genau eine Kodierung hat. `0x89` ist kein gültiges UTF-8, ein Text-Payload wird daher nie für einen Umschlag gehalten. Ein Knoten lehnt beim Start eine Chain-ID ab, die länger als `txcodec.MaxChainIDLength` ist (`config.ErrChainIDTooLong`).

Textschnittstellen transportieren signierte Transaktionen in Base64. `vexod keys sign-tx` gibt `tx: <base64>` und `encoding: base64` aus; `POST /v1/tx` liest `tx` als Base64, sofern die Anfrage nicht `"encoding": "plain"` setzt; `vexod tx parse --encoding base64` liest eine signierte Transaktion, `--encoding plain` (Standard) einen Text-Payload. Alle drei dekodieren über `txcodec.DecodeTxEncoding`.

Der Ante-Handler merkt sich nur erfolgreiche Authentifizierungen (`app.VerifyCache`, höchstens `app.VerifyCacheCapacity`), geschlüsselt über einen Digest der Transaktionsbytes und alles, wovon das Urteil abhängt, etwa die Chain-ID; ein Fehlschlag wird nie gemerkt. Die Signaturen eines Blocks werden parallel geprüft, bevor der Block der Reihe nach verarbeitet wird.

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Scope — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Canonical Payload — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Address Format — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Signed Envelope — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Required Ante Metadata — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: CheckTx Requirements — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Fee and Gas — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Load Test Payloads — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: CLI Examples — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `gas_limit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_chain_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `chain_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `base_fee` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `max(min_fee, base_fee * gas)` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blob_base_fee` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blob_gas` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blob_gas_fee_cap` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_sendRawBlobTransaction` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blob_hashes` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_getBlobSidecarByTxHash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_getBlobSidecarByBlobHash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_chainId` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `net_version` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_sendRawTransaction` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `dynamic_base_fee` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `target_gas` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `dynamic_blob_base_fee` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `target_blob_gas` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bank:send` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bank:send:<from>:<to>:<amount>` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bank:mint:<to>:<amount>` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_raw` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `max_txs` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `max_total_bytes` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `max_txs_per_sender` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
