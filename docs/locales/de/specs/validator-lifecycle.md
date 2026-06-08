# Validator Lifecycle

> Locale: de · Deutsch
> Dieses Dokument ist ein übersetzter Leitfaden auf Basis der kanonischen englischen Dokumentation. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Zweck

Dieses Dokument behandelt validator join, rotation, jail, slashing und leave lifecycle. Befehle, JSON-Felder, RPC-Namen, config key und Code-Bezeichner, die in Implementierung und Betrieb verwendet werden, bleiben aus Kompatibilitätsgründen auf Englisch.

## Kernbereich

- Beim Lesen müssen die folgenden Punkte geprüft werden. Befehle, JSON-Felder, RPC-Methoden, Konfigurationsschlüssel und Code-Bezeichner bleiben aus Kompatibilitätsgründen unverändert.
- Für detaillierte normative Formulierungen gilt der englische Originaltext.
- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/de/specs/validator-lifecycle.md`

## Beizubehaltende Bezeichner

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Englische Abschnitte

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Betriebshinweis

- `MUST`, `SHOULD`, `MAY`, Befehlsbeispiele, JSON-Beispiele und RPC-Namen behalten die englische Schreibweise.
- Führe nach Änderungen an dieser Übersetzung `make docs-check` aus.
- Wenn diese Seite der englischen Quelle widerspricht, gilt die englische Quelle; aktualisiere diese Locale-Datei im selben Change.

## Kanonische Quelle

- [English canonical document](../../en/specs/validator-lifecycle.md)
