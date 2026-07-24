> Locale: de · Deutsch

# Dokumentation

Dieses Verzeichnis ist das praktische Handbuch für `vexo-consensus`. Es richtet sich an Entwickler, Betreiber, Release-Verantwortliche und Prüfer, die das Netzwerk verstehen müssen, ohne sein Verhalten allein aus dem Quellcode abzuleiten.

Jede Seite soll Verantwortung, implementierende Dateien, Befehle, Konfigurationsschlüssel und APIs, Sicherheitsbedingungen sowie erforderliche Nachweise erklären. Englisch bleibt die normative Quelle für Protokoll, Sicherheit, Release, SDK, Befehle, Konfiguration und RPC; diese Übersetzung unterstützt das Lesen, ersetzt aber bei Audit- und Release-Entscheidungen nicht die englische Quelle.

Für den Einstieg führen Sie die folgenden Befehle aus und lesen anschließend `Node Initialization`, `Docker Deployment`, `Observability Guide` und `RPC API Versioning`.

| Aufgabe | Befehlspfad |
|---|---|
| Lokale Binärdatei erstellen | `make build` |
| Eine Unterkunft für Validierer erstellen | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| Eine Unterkunft validieren | `vexod validate --home .vexo-validator-1` und `vexod config audit --home .vexo-validator-1 --strict` |
| Einen Knoten ausführen | `vexod start --home .vexo-validator-1` |
| Einen Knoten abfragen | `curl -s http://127.0.0.1:26657/v1/status` |
| Docker-Vier-Validator-Netzwerk ausführen | `docker compose -f deployments/docker/compose.single-host-init.yml up` gefolgt von `docker compose -f deployments/docker/compose.single-host.yml up` |
| Connect Remix | Verwenden Sie die Web3-URL des Docker-Validators 1 `http://127.0.0.1:28657/web3` |
| Web3-Ketten-ID prüfen | `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` |

## Schnellstart

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Beginnen Sie hier

| Dokument | Zweck |
|---|---|
| [Production Readiness Guide](./production-readiness.md) | Single map of protocol, runtime, operations, evidence, and release readiness |

## Protokollspezifikationen

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md) und [Validator Lifecycle](./specs/validator-lifecycle.md) beschreiben Sicherheit, Finalität und Änderungen des validator set.
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md) und [Transaction Format](./specs/tx-format.md) behandeln Transport, dauerhafte Wiederherstellung und Transaktionsannahme.
- [EVM and Native Accounting](./specs/evm-native-accounting.md) definiert die Grenze zwischen nativer und EVM-Abrechnung.

## SDK und Erweiterungen

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md) und `RPC API Versioning` zeigen, wie die Runtime erweitert wird, ohne Konsens- oder RPC-Verträge zu brechen.

## Betrieb, Release und Sicherheit

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [Launch-Betriebshandbuch](./release/launch-runbook.md), `Release Pipeline` und [Version Compatibility Matrix](./release/version-compatibility.md) bilden den Betreiberpfad. [Security Audit Readiness](./security/audit-readiness.md) dokumentiert Bedrohungsmodell und Pflichtnachweise.

## Reifegradregel

Vorhandener Code allein belegt keine Produktionsreife. Erforderlich sind Unit-, adversariale und E2E-Tests, Betriebsartefakte, Annahmen und Fehlermodi sowie Ergebnisse des Release Gates. Befehle, RPC-Methoden und Konfigurationsschlüssel bleiben in allen Übersetzungen identisch.

## Forschung und Veröffentlichung

Für die Vorbereitung einer Veröffentlichung beginnen Sie mit [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). Das Dokument grenzt die tatsächlich implementierten Mechanismen, insbesondere adaptive Runden-Timeouts, das Recovery-Finalitäts-Gate und deterministische Transaktionsreihenfolge, von früheren Arbeiten ab. Es beschreibt Forschungsfragen, Hypothesen, Versuchsablauf, reproduzierbare Artefakte und Forschungsethik. Nicht gemessene Leistung wird nicht als Ergebnis dargestellt; PoS, BFT und HotStuff selbst werden nicht als neue Beiträge beansprucht.

Für die sprachübergreifende Navigation bleiben die normativen Namen `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline` und `Adaptive Recovery-Gated HotStuff Research Draft` unverändert.

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: How to Read This Set — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Start Here — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Protocol Specs — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: SDK and Extension Guides — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Operations and Release — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Security — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Localized Documentation — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Writing New Docs — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Production Claim Rule — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Documentation Review Checklist — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `vexo-consensus` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/*` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `make docs-check` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod status --json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `feature_assurance` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `network_config.json:p2p.auth_replay_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `network_config.json:p2p.node_key_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `module_config.json:governance.RequireDeposit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `module_config.json:governance.MinDeposit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `consensus_config.json:consensus.execution_commit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `mempool_config.json:mempool.WALPath` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
