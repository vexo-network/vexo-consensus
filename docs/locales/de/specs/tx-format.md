# Transaction Format

> Locale: de · Deutsch
> Dieses Dokument ist ein übersetzter Leitfaden auf Basis der kanonischen englischen Dokumentation. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Zweck

Dieses Dokument behandelt transaction format, signing, fee und gas-Regeln. Befehle, JSON-Felder, RPC-Namen, config key und Code-Bezeichner, die in Implementierung und Betrieb verwendet werden, bleiben aus Kompatibilitätsgründen auf Englisch.

## Kernbereich

- Beim Lesen müssen die folgenden Punkte geprüft werden. Befehle, JSON-Felder, RPC-Methoden, Konfigurationsschlüssel und Code-Bezeichner bleiben aus Kompatibilitätsgründen unverändert.
- Für detaillierte normative Formulierungen gilt der englische Originaltext.
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/de/specs/tx-format.md`

## Beizubehaltende Bezeichner

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

## Englische Abschnitte

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

## Betriebshinweis

- `MUST`, `SHOULD`, `MAY`, Befehlsbeispiele, JSON-Beispiele und RPC-Namen behalten die englische Schreibweise.
- Führe nach Änderungen an dieser Übersetzung `make docs-check` aus.
- Wenn diese Seite der englischen Quelle widerspricht, gilt die englische Quelle; aktualisiere diese Locale-Datei im selben Change.

## Kanonische Quelle

- [English canonical document](../../en/specs/tx-format.md)
