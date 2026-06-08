# Networking Spec

> Locale: de · Deutsch
> Dieses Dokument ist ein übersetzter Leitfaden auf Basis der kanonischen englischen Dokumentation. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Zweck

Dieses Dokument behandelt P2P handshake, gossip, peer scoring und ban policy. Befehle, JSON-Felder, RPC-Namen, config key und Code-Bezeichner, die in Implementierung und Betrieb verwendet werden, bleiben aus Kompatibilitätsgründen auf Englisch.

## Kernbereich

- Beim Lesen müssen die folgenden Punkte geprüft werden. Befehle, JSON-Felder, RPC-Methoden, Konfigurationsschlüssel und Code-Bezeichner bleiben aus Kompatibilitätsgründen unverändert.
- Für detaillierte normative Formulierungen gilt der englische Originaltext.
- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/de/specs/networking-spec.md`

## Beizubehaltende Bezeichner

- `consensus`
- `tx`
- `commit`
- `evidence`
- `network_config.json`
- `rpc.address`
- `p2p.listen_address`
- `p2p.peers`
- `p2p.seeds`
- `p2p_address`
- `rpc_address`
- `host:port`
- `0.0.0.0:26656`
- `[::]:26656`
- `0`
- `p2p.tls_cert_path`
- `p2p.tls_key_path`
- `p2p.tls_ca_path`

## Englische Abschnitte

- Networking Spec
- Scope
- Transport
- Topics
- Handshake
- Address Roles
- Transport TLS
- Peer Scoring
- Reconnect and Backoff
- DoS/DDOS Defenses
- Operational Signals

## Betriebshinweis

- `MUST`, `SHOULD`, `MAY`, Befehlsbeispiele, JSON-Beispiele und RPC-Namen behalten die englische Schreibweise.
- Führe nach Änderungen an dieser Übersetzung `make docs-check` aus.
- Wenn diese Seite der englischen Quelle widerspricht, gilt die englische Quelle; aktualisiere diese Locale-Datei im selben Change.

## Kanonische Quelle

- [English canonical document](../../en/specs/networking-spec.md)
