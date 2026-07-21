# EVM-Aktualisierungsleitfaden

> Locale: de · Deutsch
> Dieses Dokument ist die deutsche Übersetzung der englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen folgen der englischen Quelle.

Dieser Leitfaden erklärt, wie man den eingebauten EVM-Stack aktualisiert, ohne Chain-ID-Behandlung, Web3-Kompatibilität oder Release-Nachweise zu beschädigen. Er richtet sich an Betreiber und Maintainer, die go-ethereum aktualisieren, Fork-Presets anpassen oder EVM-Verhalten kontrolliert ändern müssen.

## Was als EVM-Update gilt

Behandeln Sie jede Änderung als release-sensitives Feature-Update, wenn sie Ethereum-artige Ausführung oder Web3-Sichtbarkeit beeinflussen kann:

- Versionssprung von `go-ethereum` in `modules/evm/backend/geth`
- Änderungen an `modules/evm/ethcompat`
- Änderungen an `modules/evm`
- Änderungen an `execution.evm_fork_preset`
- Änderungen an `execution.evm_chain_config_json`
- Änderungen an Raw-Transaction-Zulassung, Gas Accounting, Receipts, Traces, Proofs oder Block-Response-Feldern
- Änderungen an verwalteten Web3-Accounts wie `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction` oder `eth_sendTransaction`

## Sichere Update-Reihenfolge

Nutzen Sie diese Reihenfolge, damit Code, Konfiguration und Dokumentation zusammenbleiben:

1. Zuerst den isolierten geth-backed Adapter aktualisieren.
2. Danach Fixture-Corpus und Conformance-Tests aktualisieren.
3. Bei Semantikänderungen `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md` und `docs/sdk/rpc-api-versioning.md` aktualisieren.
4. Wenn sich die Release-Evidence-Form ändert, `docs/release/release-pipeline.md` aktualisieren.
5. Wenn sich Operator-Schalter ändern, die Node-Konfigurationsdokumentation anpassen.
6. Vor dem Merge die Validierungsmatrix erneut ausführen.

Erhöhen Sie die EVM-Runtime-Version nicht und liefern Sie sie nicht gleichzeitig aus, außer wenn Conformance-Suiten, RPC-Smoke-Checks und Docker-Deployment-Checks alle bestanden haben.

## Update-Workflow

### 1. Änderung eingrenzen

Dokumentieren Sie die genaue Absicht des Updates:

- nur fork behavior
- nur transaction admission
- nur execution semantics
- nur RPC compatibility
- nur blob / receipt / trace handling
- nur managed account oder wallet behavior

Diese Aufteilung hält die Review fokussiert und verhindert unnötige Kopplungen.

### 2. Die Änderung in der engsten Schicht machen

Bevorzugen Sie diese Grenzen:

- `modules/evm/backend/geth` für Änderungen an der Integration mit upstream go-ethereum
- `modules/evm/ethcompat` für raw transaction decoding, hash preservation und Fixture-Verarbeitung
- `modules/evm` für state transition, receipts, logs, storage und snapshot-Verhalten
- `rpc` für Änderungen an der Web3 request/response-Oberfläche
- `cmd/vexod` nur dann, wenn CLI oder Release-Workflow das neue Verhalten sichtbar machen müssen

Erreicht die Änderung application modules, halten Sie die Modulgrenze explizit und bewahren Sie deterministische State Writes.

### 3. Standardkonfiguration aktualisieren

Wenn sich die Semantik ändert, aktualisieren Sie im gleichen Patch die Default-Konfiguration:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- bei Bedarf die RPC-Felder für verwaltete Accounts in `network_config.json`
- die EVM Chain ID in `module_config.json`

Verlassen Sie sich nie auf einen versteckten CLI-Flag, um Runtime-Verhalten zu erklären. Die Konfiguration muss das Verhalten aus den Dateien selbst sichtbar machen.

### 4. Conformance-Stack ausführen

Führen Sie mindestens Folgendes aus:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Prüfen Sie danach die für Anwender sichtbaren Pfade, die zuerst kaputtgehen:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Bei Docker-Single-Host-Deployments prüfen Sie zusätzlich:

```text
http://127.0.0.1:28657/web3
```

Kontrollieren Sie mindestens diese Verhaltensweisen:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

Testen Sie anschließend einen einfachen Contract-Deploy, einen Proxy-Contract-Deploy und den UUPS-Upgrade-Pfad mit demselben RPC-Endpunkt, den Wallet oder Tool in Produktion verwenden.

### 5. Proxy und Upgrade verifizieren

Das EVM-Update ist erst fertig, wenn all dies wahr ist:

- ein normaler Contract-Deploy gelingt
- ein Proxy-Deploy gelingt
- ein UUPS-Upgrade-Aufruf gelingt
- nach dem Upgrade liefern Storage- und Code-Lesungen die erwarteten Werte
- das Nonce-Tracking bleibt monoton
- der Block Producer akzeptiert die resultierenden Transaktionen ohne unsafe proposal-Fehler

Wenn der Proxy-Deploy klappt, das Upgrade aber fehlschlägt, ist die Änderung noch nicht auslieferbar. Behandeln Sie das als Release-Blocker, nicht als Warnung.

### 6. Evidence aktualisieren

Wenn sich die EVM-Oberfläche ändert, aktualisieren Sie auch das Release-Evidence-Bundle:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- alle gepinnten SHA-256-Fixture-Referenzen

Release-Evidence muss sagen, was geändert wurde, was getestet wurde und welches Commit oder welche Version verifiziert wurde. Beschreiben Sie ein EVM-Update nie als abgeschlossen, wenn die Evidence nicht zum tatsächlich ausgeführten Code passt.

## Validierungsmatrix

Nutzen Sie diese Tabelle als Merge-Gate.

| Check | Warum wichtig |
| --- | --- |
| `make evm-conformance` | fängt Fork-Regel- und Ausführungsregressionen ab |
| `go test ./modules/evm -count=1` | prüft Receipts, Logs, Storage, Balances und Snapshots |
| `go test ./rpc -count=1` | prüft Web3 Request/Response-Kompatibilität |
| `make network-e2e` | bestätigt, dass der Knoten weiterhin startet, Peers hat und committet |
| Docker single-host smoke | bestätigt den Pfad, den Remix und Browser-Tools verwenden |
| Contract deploy | bestätigt Transaction Admission und Receipt-Generierung |
| Proxy deploy | bestätigt ABI- und Storage-Layout-Annahmen |
| UUPS upgrade | bestätigt Upgrade-Semantik und Reads nach dem Upgrade |

Ist ein Check rot, ist das Update noch nicht fertig.

## Rollback-Kriterien

Rollen Sie das EVM-Update zurück, wenn eines der folgenden Dinge passiert:

- `eth_chainId` ändert sich unerwartet
- `eth_sendRawTransaction` lehnt gültige Transaktionen ab
- `eth_call` oder `eth_estimateGas` weichen von den erwarteten Fork-Regeln ab
- Receipts, Logs oder Proofs passen nicht mehr zum committed State
- Proxy- oder Upgrade-Transaktionen schlagen fehl
- die Release-Evidence passt nicht mehr zum aktuellen Codepfad

Ein Rollback muss die zuletzt bekannte gute Adapter-Version, die Default-Konfiguration und den Fixture-Satz gemeinsam wiederherstellen.

## Technischer Paritätsanhang

Dieser Anhang hält den Leitfaden mit dem Rest der Dokumentation in Einklang.

- Behalten Sie `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc` und `cmd/vexod` als stabile Implementierungsgrenzen bei.
- Behalten Sie die Schreibweise von `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction` und `eth_sendTransaction` unverändert.
- Behalten Sie auch `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures` und `--evm-web3-conformance-evidence` unverändert.
- Die operative Frage bleibt einfach: Bewahrt dieses Update Ethereum-artige Ausführung und bleibt es zugleich mit Vexo-Konsens und Release-Sicherheit vereinbar?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
