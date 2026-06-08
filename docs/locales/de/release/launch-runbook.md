# Launch Runbook

> Locale: de · Deutsch
> Dieses Dokument ist ein übersetzter Leitfaden auf Basis der kanonischen englischen Dokumentation. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Zweck

Dieses Dokument behandelt Operator-Checkliste und Ablauf vor dem Netzwerkstart. Befehle, JSON-Felder, RPC-Namen, config key und Code-Bezeichner, die in Implementierung und Betrieb verwendet werden, bleiben aus Kompatibilitätsgründen auf Englisch.

## Kernbereich

- Beim Lesen müssen die folgenden Punkte geprüft werden. Befehle, JSON-Felder, RPC-Methoden, Konfigurationsschlüssel und Code-Bezeichner bleiben aus Kompatibilitätsgründen unverändert.
- Für detaillierte normative Formulierungen gilt der englische Originaltext.
- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/de/release/launch-runbook.md`

## Beizubehaltende Bezeichner

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `chain_id`

## Englische Abschnitte

- Launch Runbook
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Betriebshinweis

- `MUST`, `SHOULD`, `MAY`, Befehlsbeispiele, JSON-Beispiele und RPC-Namen behalten die englische Schreibweise.
- Führe nach Änderungen an dieser Übersetzung `make docs-check` aus.
- Wenn diese Seite der englischen Quelle widerspricht, gilt die englische Quelle; aktualisiere diese Locale-Datei im selben Change.

## Kanonische Quelle

- [English canonical document](../../en/release/launch-runbook.md)
