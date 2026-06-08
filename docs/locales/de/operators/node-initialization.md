# Node Initialization

> Locale: de · Deutsch
> Dieses Dokument ist ein übersetzter Leitfaden auf Basis der kanonischen englischen Dokumentation. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Zweck

Dieses Dokument behandelt Initialisierung von Archive- und Validator-Nodes sowie getrennte Konfigurationsdateien. Befehle, JSON-Felder, RPC-Namen, config key und Code-Bezeichner, die in Implementierung und Betrieb verwendet werden, bleiben aus Kompatibilitätsgründen auf Englisch.

## Kernbereich

- Beim Lesen müssen die folgenden Punkte geprüft werden. Befehle, JSON-Felder, RPC-Methoden, Konfigurationsschlüssel und Code-Bezeichner bleiben aus Kompatibilitätsgründen unverändert.
- Für detaillierte normative Formulierungen gilt der englische Originaltext.
- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/de/operators/node-initialization.md`

## Beizubehaltende Bezeichner

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`

## Englische Abschnitte

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## Betriebshinweis

- `MUST`, `SHOULD`, `MAY`, Befehlsbeispiele, JSON-Beispiele und RPC-Namen behalten die englische Schreibweise.
- Führe nach Änderungen an dieser Übersetzung `make docs-check` aus.
- Wenn diese Seite der englischen Quelle widerspricht, gilt die englische Quelle; aktualisiere diese Locale-Datei im selben Change.

## Kanonische Quelle

- [English canonical document](../../en/operators/node-initialization.md)
