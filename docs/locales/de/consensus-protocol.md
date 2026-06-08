# Consensus Protocol Overview

> Locale: de · Deutsch
> Dieses Dokument ist ein übersetzter Leitfaden auf Basis der kanonischen englischen Dokumentation. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Zweck

Dieses Dokument behandelt das Konsensmodell, Ausführungs-/Finality-Begriffe und die Sicherheitsgrenze. Befehle, JSON-Felder, RPC-Namen, config key und Code-Bezeichner, die in Implementierung und Betrieb verwendet werden, bleiben aus Kompatibilitätsgründen auf Englisch.

## Kernbereich

- Beim Lesen müssen die folgenden Punkte geprüft werden. Befehle, JSON-Felder, RPC-Methoden, Konfigurationsschlüssel und Code-Bezeichner bleiben aus Kompatibilitätsgründen unverändert.
- Für detaillierte normative Formulierungen gilt der englische Originaltext.
- Canonical path: `docs/consensus-protocol.md`
- Locale path: `docs/locales/de/consensus-protocol.md`

## Beizubehaltende Bezeichner

- `FinalizeBlock`
- `consensus_config.json`
- `execution_commit`
- `finalized`
- `qc`
- `require_network_safety`
- `block_committed`
- `deterministic`
- `ed25519`
- `bls`

## Englische Abschnitte

- Consensus Protocol Overview
- Model
- Execution Terms
- Safety Boundary
- Crypto Boundary
- Operational Boundary

## Betriebshinweis

- `MUST`, `SHOULD`, `MAY`, Befehlsbeispiele, JSON-Beispiele und RPC-Namen behalten die englische Schreibweise.
- Führe nach Änderungen an dieser Übersetzung `make docs-check` aus.
- Wenn diese Seite der englischen Quelle widerspricht, gilt die englische Quelle; aktualisiere diese Locale-Datei im selben Change.

## Kanonische Quelle

- [English canonical document](../en/consensus-protocol.md)
