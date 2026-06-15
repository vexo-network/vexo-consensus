> Locale: de · Deutsch

# Knoteninitialisierung

In dieser Anleitung wird erläutert, wie Sie Validator- und Archivknoten-Homes initialisieren, starten, überprüfen, ob sie fehlerfrei sind, und Clients verbinden.

Die Peer-Konnektivität sollte in `network_config.json` konfiguriert werden und nicht wiederholt in der Befehlszeile `start` übergeben werden.

Laufzeitverhalten, das sich auf Konsens, RPC, P2P, Protokollierung oder verwaltete Web3-Konten auswirkt, betrifft nur die Konfigurationsdatei. `vexod start` lehnt Flags wie `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env` und `--evm-account-key` ab; Bearbeiten Sie stattdessen die geteilten Konfigurationsdateien, damit jeder Bediener das gleiche deterministische Knotenverhalten überprüft.

Es gibt keinen Knotenmodusschalter. Ein Node-Home wird durch seine Konfigurationsdateien, Genesis, Schlüsselmaterial und ob `validator_id` plus ein Unterzeichner vorhanden sind, definiert.

## Was Sie bauen

Ein Vexo-Knoten-Home ist ein Verzeichnis, das alles enthält, was ein Knoten zum Starten benötigt:
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
Die wichtige Regel ist einfach: Einmal initialisieren, Konfigurationsdateien bearbeiten und dann starten. Verstecken Sie das Netzwerkverhalten nicht in Shell-Flags.

## Fünfminütiger lokaler Lauf

Verwenden Sie diesen Ablauf, wenn Sie beweisen möchten, dass die Binärdatei funktioniert, bevor Sie über die Bereitstellung auf mehreren Hosts nachdenken.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
In einem anderen Terminal:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
Erwartete Statusform:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
Die letzte Höhe kann bei einem Einzelknoten- oder Leer-Mempool-Lauf bei Null bleiben, wenn die Erstellung leerer Blöcke deaktiviert ist. Das bedeutet nicht, dass der Prozess unterbrochen ist. Das bedeutet, dass der Knoten keine leeren Blöcke produziert. Fügen Sie Transaktionen hinzu oder führen Sie ein Multi-Validator-Testnetzwerk aus, um kontinuierliche Commits zu beobachten.

## Lokales Netzwerk mit vier Validatoren

Verwenden Sie diesen Ablauf, wenn Sie Peer-Konnektivität, Antragstellerrotation, Block-Commit-Protokolle und Höhenwachstum wünschen.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
Nützliche Kontrollen:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
Wenn die Block-Commit-Protokollierung in `log_config.json` aktiviert ist, enthalten Validator-Protokolle Ereignisse wie:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
Stoppen Sie das generierte lokale Netzwerk mit:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 und Remix

JSON-RPC im Ethereum-Stil befindet sich am Web3-Endpunkt und nicht unter dem versionierten Vexo-Betriebs-API-Namespace.

Für den Docker-Single-Host-Validator 1 lautet die URL des benutzerdefinierten Remix-Anbieters:
```text
http://127.0.0.1:28657/web3
```
Für einen direkten lokalen Knoten mit dem Standard-RPC-Port:
```text
http://127.0.0.1:26657/web3
```
Testen Sie den gleichen Aufruf von Remix:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
Wenn ein Browser meldet, dass der Ketten-ID-Abruf fehlgeschlagen ist, überprüfen Sie diese der Reihe nach:

1. Die URL endet mit dem Web3-Endpunktpfad.
2. Der Browser kann den Host-Port erreichen. Docker-Beispiele machen `28657`, `28667`, `28677` und `28687` verfügbar; Innerhalb des Containers ist der RPC-Port immer noch `26657`.
3. Der RPC-Server läuft; Fragen Sie den Statusendpunkt auf demselben Host und Port ab.
4. CORS ist in der `network_config.json`/RPC-Konfiguration zulässig. Der Standardhandler ermöglicht den Browser-Preflight, wenn keine benutzerdefinierte CORS-Liste festgelegt ist.
5. Die Kette hat eine EVM-Ketten-ID ungleich Null in `module_config.json`.

## Validatorknoten

Verwenden Sie `init validator`, wenn der Knoten Konsensnachrichten vorschlägt, abstimmt, unterzeichnet und an der Validatorrotation teilnimmt.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
Legen Sie `VEXO_KEY_PASSPHRASE` fest, bevor Sie diesen Befehl ausführen, oder übergeben Sie `--passphrase` für eine einmalige lokale Einrichtung. `--encrypt-keys` verschlüsselt `validator.key.json`, `node.key.json` und `validator.vrf.key.json`.

Faustregel für die Schlüsselverwahrung:

- `validator.key.json` signiert Konsensvorschläge, Abstimmungen, Timeout-Abstimmungen und Nachrichten zur Endgültigkeit.
- `node.key.json` signiert nur P2P-Handshakes; Er darf niemals als Validator-Konsensschlüssel wiederverwendet werden.
- `validator.vrf.key.json` beweist die Zufälligkeit des Ausschusses und sollte wie Material aus der Verwahrung des Prüfers behandelt werden.
– Öffentliche Listener müssen verschlüsselte lokale Schlüsseldokumente oder Schlüsseldokumente im Remote-Signer-/KMS-Stil verwenden. Wenn ein Knoten während `require_network_safety=true` öffentliches RPC oder authentifiziertes öffentliches P2P verfügbar macht, lehnt der Start lokale Validierungsschlüssel im Klartext ab.
- Generierte Schlüssel werden mit dem Dateisystemmodus `0600` geschrieben; bevorzugen immer noch einen Remote-Signer/KMS für langlebige Validatoren.

Für einen BLS-Konsensschlüssel:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` schreibt ein `blst-bls12381-minpk-v1` BLS-Schlüsseldokument und kopiert den Besitznachweis in `genesis.json` Validator-Metadaten als `bls_pop`.

Dadurch entsteht:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` ist der Konsensunterzeichner. `node.key.json` ist der P2P-Handshake-Unterzeichner, auf den `network_config.json:p2p.node_key_path` verweist. Sie sind bewusst getrennt, sodass Archivknoten und Validatoren denselben Transport verwenden können, ohne jedem Peer einen Validator-Signaturschlüssel zu geben.

Beginnen Sie mit konfigurationsgesteuertem Netzwerk:
```bash
vexod start --home .vexo-validator-1
```
Lesen Sie nach dem Start die Protokolle. Ein fehlerfreier Validator sollte Node-Running-, RPC-Listening-, P2P-Listening-Ereignisse und, sobald Blöcke festgeschrieben sind, Block-Committed-Ereignisse aussenden. Wenn die Erstellung leerer Blöcke deaktiviert ist, können fehlende Block-Commit-Protokolle einfach bedeuten, dass keine Transaktionen vorhanden sind.

## Archivknoten

Verwenden Sie `init archive`, wenn der Knoten Kettendaten behalten, RPC verfügbar machen, von Peers synchronisieren und Validatorsignaturen vermeiden soll.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
Dadurch entsteht:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

Es erstellt **nicht** `validator.key.json`.

Beginnen Sie mit:
```bash
vexod start --home .vexo-archive-1
```
Archivknoten unterzeichnen keine Konsensabstimmungen. Sie sind nützlich für RPC, Indizierung, Statussynchronisierung, Bereitstellung historischer Beweise und die Aufbewahrung eines umfassenderen Abfrageverlaufs als Pruning-Validatoren.

## Konfigurationsdateien aufteilen

Knotenhäuser verwenden separate Konfigurationsdateien, sodass Bediener ein Subsystem bearbeiten können, ohne nicht verwandte Einstellungen zu vermischen:

- `config.json` enthält Knotenidentität, Ketten-ID, Datenpfad und Zeiger auf die geteilten Konfigurationsdateien.
- `module_config.json` enthält die Auswahl des Anwendungsmoduls, die Ausführungs-/Ante-Richtlinie und die Governance-Richtlinie auf Modulebene.
- `network_config.json` enthält RPC, P2P-Knotenidentität, Listen-/Peer-/Seed-Einstellungen, TLS-/Authentifizierungseinstellungen und Peer-Scoring-Richtlinie.
- `consensus_config.json` enthält Konsensschleifen-Timing, Leerblock-Richtlinie, Krypto-Backend, VRF, Validator-Zulassung und Ausschussrichtlinie.
– `mempool_config.json` enthält Mempool-Größe, Gebühr, Priorität, WAL, Duplikat und TTL-Richtlinie.
- `log_config.json` enthält Protokollformat, Ebene, Block-Commit-Ereignisprotokollierung und Peer-Ereignisprotokollierung.
– `genesis.json` enthält unveränderliche Genesis-Validatoren, Validator-Metadaten und den Status des Genesis-Moduls.

Zu den `network_config.json` RPC-Einstellungen gehören auch `shutdown_timeout`, `web3_max_subscriptions_per_connection` und `web3_idle_timeout`. `shutdown_timeout` begrenzt das ordnungsgemäße Herunterfahren der Konsensschleife, des RPC-Servers und des Knotentransports, sodass Bediener nicht ewig auf einem festgefahrenen Stopppfad warten müssen. Der generierte Standardwert ist `10s`; Web3-Abonnements sind standardmäßig auf 256 pro Verbindung mit einem Leerlaufzeitlimit von `2m` festgelegt, sodass öffentliche RPC-Endpunkte keine unbegrenzten Leerlaufabonnements ansammeln können.

Zu den `network_config.json` P2P-Einstellungen gehören `auth_replay_path`, `require_auth_replay_store` und `dial_timeout`. Der generierte Standardwert schreibt Nonce-Replay-Beweise in `data/p2p_auth_replay.jsonl` und verwendet ein `10s`-Timeout für ausgehende Anrufe. Für private Loopback-Tests ist der Replay Store meist eine harmlose Buchhaltung; Für öffentlich authentifiziertes P2P ist dies eine Sicherheitsanforderung, da dadurch verhindert wird, dass ein erfasster signierter Handshake-Nonce nach dem Neustart wiedergegeben wird. `dial_timeout` sollte lang genug für TLS, signierte Handshake-Überprüfung und regionsübergreifende Latenz sein; Wenn Sie den Wert zu niedrig einstellen, sehen gesunde Artgenossen schuppig aus und können die Lebendigkeit nach Neustarts verlangsamen.

`network_config.json` besitzt auch die Startstatussynchronisierung. Dies ist nützlich für Archivknoten, Ersatzvalidatoren oder auf einer sauberen Maschine wiederhergestellte Knoten. Wenn `state_sync.enabled` wahr ist, lädt `vexod start` den ersten gültigen Snapshot von `state_sync.snapshot_urls` herunter, überprüft Ketten-ID, Prüfsumme, Statuswurzeln und KV-Namespaces, stellt ihn in LevelDB wieder her, erstellt Indizes neu und startet erst dann den Knoten. Wenn der lokale Status bereits `state_sync.min_height` erfüllt und `state_sync.trust_local_higher` wahr ist, protokolliert der Start `state_sync_skipped` und behält den lokalen Speicher.

Beispiel `state_sync` Block:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
Beim Start wird `state_sync_candidate_failed` für einen Abruffehler, `state_sync_candidate_rejected` für einen ungültigen oder veralteten Snapshot und `state_sync_applied` nach einer überprüften Wiederherstellung protokolliert. Halten Sie `max_snapshot_bytes` unter dem größten Snapshot, den Ihre Infrastruktur absichtlich bereitstellt, aber hoch genug für ein normales Zustandswachstum. Richten Sie öffentliche Knoten nicht auf eine nicht authentifizierte Snapshot-Quelle eines Drittanbieters aus, es sei denn, der Betreiber verfügt über eine Out-of-Band-Vertrauensrichtlinie und Endgültigkeits-/Light-Client-Nachweise für diese Quelle.

Wenn ein Feld das Netzwerkverhalten ändert, bearbeiten Sie die geteilte Konfigurationsdatei und übertragen oder verteilen Sie die überprüfte Datei. Verlassen Sie sich für das Laufzeitverhalten nicht auf lange `vexod start`-Flags. Der Startbefehl lehnt absichtlich Konsens-Timing, leere Blöcke, P2P-Authentifizierung, RPC-Administration und verwaltete Web3-Schlüsselflags ab, damit Bediener nicht versehentlich ein anderes Verhalten als die überprüfte Konfiguration ausführen.

## Welche Datei bearbeite ich?

| Ziel | Datei | Feld |
|---|---|---|
| RPC-Bind-Port ändern | `network_config.json` | `rpc.address` |
| P2P-Bind-Port ändern | `network_config.json` | `p2p.listen_address` |
| Persistente Peers hinzufügen | `network_config.json` | `p2p.peers` |
| Seed-Peers hinzufügen | `network_config.json` | `p2p.seeds` |
| Leere Blöcke aktivieren/deaktivieren | `consensus_config.json` | Konsens-Leerblock-Feld |
| Konsens-Timeouts optimieren | `consensus_config.json` | Vorschlags-, Prevote-, Precommit- und Commit-Timeout-Felder |
| Erfordern endgültige Ausführung | `consensus_config.json` | Konsens-Ausführungs-Commit-Feld |
| Module aktivieren/deaktivieren | `module_config.json` | Anwendungsmodulliste |
| EVM-Ketten-ID ändern | `module_config.json` | Ausführungs-EVM-Ketten-ID-Feld |
| Grundgebühr/Gas anpassen | `module_config.json` | Ausführungsfelder „Grundgebühr“, „Dynamische Gebühr“, „Zielgas“ und „Maximalgas“ |
| Mempool WAL | konfigurieren `mempool_config.json` | mempool WAL-Pfad |
| Kontrollblock-Commit-Protokolle | `log_config.json` | Feld „Commit-Events“ protokollieren |
| Peer-Protokolle steuern | `log_config.json` | Peer-Events-Feld protokollieren |

Führen Sie im Zweifelsfall Folgendes aus:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## Schlüsseltypen

Die Validator-Initialisierung ist standardmäßig auf `--key-type bls` eingestellt, da die Validierung der Netzwerksicherheit eine geprüfte Endgültigkeit des BLS-Aggregats erfordert. `--key-type ed25519` bleibt für private Experimente und benutzerdefinierte Bereitstellungen außerhalb der Netzwerksicherheitstür verfügbar. `--encrypt-keys` sollte für jedes Nicht-Wegwerf-Knotenhaus verwendet werden. Die eigenständige Schlüsselgenerierung unterstützt auch VRF-Schlüssel:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
VRF-Schlüssel sind keine Konsensunterzeichner. Sie werden für die Auswahl von VRF-gestützten Komitees verwendet und sollten von `consensus_config.json` bis `vrf_key_paths` plus Validator-Metadatenschlüssel `vrf_public_key` referenziert werden, wenn dieses Backend aktiviert ist.

`config.json` verweist auf die geteilten Konfigurationsdateien:
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
Jeder Pfad kann absolut oder relativ zum Knotenheim sein. Wenn es weggelassen wird, verwendet `vexod` die Standarddatei `<home>/<name>_config.json`.

Beispiel `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
Die Governance-Richtlinie lebt auch in `module_config.json`. Für generierte netzwerksichere Konfigurationen ist eine Angebotseinzahlung erforderlich:
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
Die Anzahlung wird vom Einreicher des Angebots treuhänderisch hinterlegt. Bei bestandenen Vorschlägen wird die Anzahlung zurückerstattet; Abgelehnte Vorschläge verschieben es nach `RejectedDeposits`. Verwenden Sie eine von Ihrem Treasury-/Community-Pool-Modul gesteuerte Adresse, wenn abgelehnte Einzahlungen auf ein Treasury-Konto statt auf das Standardmodulkonto eingezahlt werden sollen.

Beispiel `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` und `rpc.evm_account_private_keys` sind optional und unterstützen Web3-Methoden für verwaltete Konten wie `eth_accounts`, `eth_sign`, `eth_signTransaction` und `eth_sendTransaction`. Bevorzugen Sie `evm_account_key_envs`, damit der private Schlüssel von der Prozessumgebung oder dem Secret Manager eingefügt und nicht in JSON gespeichert wird. Lassen Sie beide Listen für den normalen Validatorbetrieb leer, es sei denn, dieser Knoten fungiert absichtlich als lokaler Web3-Hot-Wallet-Endpunkt. Die Startsicherheit lehnt verwaltete EVM-Hotkeys auf öffentlichen RPC-Listenern ab.

Beispiel `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` werden relativ zu dem Verzeichnis aufgelöst, das `consensus_config.json` enthält. Verwenden Sie verschlüsselte Schlüsseldokumente und stellen Sie dem Knotenprozess `VEXO_KEY_PASSPHRASE` zur Verfügung, wenn die lokale VRF-Schlüsselverwahrung unvermeidbar ist. Fügen Sie rohe private VRF-Skalare nicht direkt in `consensus_config.json` für vom Betreiber betriebene Netzwerke ein.

Verwenden Sie `vexod config paths --home <home>`, um alle aufgelösten Pfade zu überprüfen.

Archivkonfiguration hat:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
Archiv `consensus_config.json` deaktiviert die lokale Konsensschleife:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
Generierte Validator-Homes setzen standardmäßig `"require_network_safety": true` in `config.json`. Dies ist kein Modus; Es handelt sich um eine Sicherheitsschleuse für Startups, die deterministische Krypto, nicht signierte/nicht signierte Transaktionen, fehlende Gebühren-/Gasuntergrenzen, fehlende dauerhafte Mempool-WAL, fehlende Ersatzrichtlinien für Transaktionen mit demselben Unterzeichner/Nonce, unsichere Ausschusszufälligkeit und andere `execution_commit`-Werte als `finalized` ablehnt.

Wenn `require_network_safety` aktiviert ist, führen Sie Folgendes aus:
```bash
vexod config audit --home <home> --strict
```
bevor Sie den Knoten starten. Die Prüfung sollte für jeden Validator und jedes Archivhaus bestanden werden, die am selben Netzwerk teilnehmen.

## Konfigurationsbasierte Peers

Peer- und Listen-Adressen live in `network_config.json`:
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` lädt diese Peers automatisch:
```bash
vexod start --home .vexo-archive-1
```
Persistente Peers und Seeds werden in `network_config.json` konfiguriert; `vexod start` akzeptiert keine Peer- oder Seed-Host-Überschreibungen.

Geben Sie keine langlebigen Host- oder `host:port`-Einstellungen in die `vexod start`-Befehlszeile ein. Bearbeiten Sie stattdessen `rpc.address`, `p2p.listen_address`, `p2p.peers` und `p2p.seeds` in `network_config.json`.

Halten Sie `p2p.node_id` für die gesamte Lebensdauer des Node-Homes stabil. `p2p.node_key_path` sollte auf `node.key.json` oder ein anderes lokales/verwaltetes Schlüsseldokument verweisen, das nur für die Peer-Handshake-Signierung verwendet wird. Peer-Maps sollten Peer-Knoten-IDs verwenden, keine Kontoadressen oder Validator-Operator-Namen, es sei denn, diese sind absichtlich identisch.

Für den verschlüsselten und authentifizierten gRPC-Peer-Transport legen Sie außerdem `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path` und optional `p2p.tls_server_name` in `network_config.json` fest. Relative TLS-Pfade werden aus dem Node-Home-Verzeichnis aufgelöst. Behalten Sie `p2p.dial_timeout` in derselben Datei, damit jeder Operator dasselbe Verhalten bei der Wiederverbindung verwendet. Verstecken Sie das Peer-Timing nicht in Shell-Skripten.

## Konsens-Timing

Das Timing der Konsensschleife lebt in `consensus_config.json`:
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` steuert, wie lange eine Runde auf einen Vorschlag wartet.
- `timeout_prevote` steuert das Fenster zur Stimmensammlung.
– `timeout_precommit` steuert das Commit-Zertifikat-Sammelfenster.
- `timeout_commit` steuert die minimale Verzögerung nach einem festgeschriebenen Block.
- `create_empty_blocks: false` bedeutet, dass der Knoten nur dann Vorschläge macht, wenn Transaktionen verfügbar sind.
– `execution_commit: "finalized"` wartet auf die Drei-Ketten-Finalitätsentscheidung von HotStuff, bevor der finalisierte Vorfahre ausgeführt wird, und ist der generierte Validator-Standard. `execution_commit: "qc"` führt QC-zertifizierte Blöcke sofort aus und behält sie bei, wird jedoch vom Sicherheitstor abgelehnt.

`round_timeout` wird nur als Kompatibilitätsaggregat aufbewahrt. Bevorzugen Sie die Timeout-Felder im Tendermint-Stil oben.

Wenn `create_empty_blocks` falsch ist, kann die Höhe unverändert bleiben, während der Mempool leer ist. Das wird erwartet: Die Kette wartet auf nützliche Arbeit, anstatt leere Blöcke zu übergeben. Wenn eine Transaktion erscheint und der Status der lokalen Konsensrunde an einem anderen Antragsteller vorbeigegangen ist, geht der Knoten zur nächsten Runde über, in der sein Validator der Antragsteller ist und aus dem Mempool erstellt. Dieser Wiederherstellungspfad sorgt dafür, dass die durch Transaktionen ausgelöste Lebendigkeit erhalten bleibt, ohne Spam mit leeren Blöcken erneut zu aktivieren.

## Multi-Validator-Netzwerk

Für ein generiertes Netzwerk:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
Jedes generierte Validator-Haus erhält:

- sein eigener `validator.key.json`
- eigene geteilte Konfigurationsdateien: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` und `log_config.json`
- ein gemeinsamer `genesis.json`
- `network_config.json` Peer-Einträge für die anderen Validatoren

`vexod network up` und `make network-e2e` verwenden ein Timeout auf Prozessebene, während sie darauf warten, dass alle Validatoren starten, die Smoke-Transaktion übermitteln und das Höhenwachstum beobachten. Das standardmäßige Befehlszeitlimit ist absichtlich länger als das Konsensintervall, da es den Prozessstart, das Öffnen von LevelDB, P2P-signierte Handshakes, TLS-/Authentifizierungsprüfungen, Transaktionszulassung und Endgültigkeit umfasst. Wenn Sie die Konsens-Timeouts aggressiv senken, halten Sie das Netzwerk-Up-Timeout groß genug, um Startfehler zu diagnostizieren, anstatt den Harness zu früh zu beenden.

Geben Sie für Container- oder Multi-Host-Netzwerke Topologiewerte in eine JSON-Datei ein:
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` und `rpc_host_template` sind Wählziele, die in die Peer-Liste `network_config.json` jedes Knotens geschrieben werden. In Docker können dies Dienstnamen wie `validator-%d` sein.
- `p2p_advertise_host_template` und `rpc_advertise_host_template` sind öffentliche Adressen, die in Validator-Metadaten in `genesis.json` geschrieben werden. Verwenden Sie hier DNS-Namen oder öffentliche IPs für öffentliche Netzwerke.
- `p2p_listen_host` und `rpc_listen_host` sind lokale Bind-Hosts. Verwenden Sie `0.0.0.0` für Container oder Server, die alle Schnittstellen überwachen sollen.
– Verwenden Sie keine reinen Docker-Dienstnamen als angekündigte öffentliche Adressen, es sei denn, das Netzwerk ist absichtlich privat.

Generieren Sie dann Knotenhäuser aus dieser Datei:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## Fehlerbehebung

| Symptom | Höchstwahrscheinlich Ursache | Was ist zu überprüfen |
|---|---|---|
| `latest_height` erhöht sich nicht | Leere Blöcke deaktiviert und keine Sendungen, nicht genügend Validatoren online oder Unterzeichner nicht verfügbar | `consensus_config.json`, Validatorprotokolle, `/v1/diagnostics` |
| `peer_count` ist `0` | Peer-Adressen sind nicht erreichbar oder `network_config.json` wurde für die falschen Hostnamen generiert | `p2p.peers`, Container-Host-Ports, DNS, Firewall |
| `p2p auth replay store` Fehler | Öffentliches/authentifiziertes P2P erfordert dauerhaften Wiedergabespeicher | `p2p.auth_replay_path` und Schreibberechtigung unter der Startseite |
| `eth_chainId` schlägt in Remix | fehl Falsche URL, falscher Host-Port oder Browser-CORS/Preflight durch benutzerdefinierte Konfiguration blockiert | Verwenden Sie die Web3-Endpunkt-URL und rufen Sie dann denselben Endpunkt direkt auf |
| `config audit --strict` schlägt fehl | Safety Gate hat eine unsichere Konfigurationseigenschaft gefunden | Lesen Sie die fehlgeschlagene Prüfung und bearbeiten Sie dann die geteilte Konfigurationsdatei mit dem Namen |
| `no block_committed logs` | Protokollierung deaktiviert oder es werden keine Blöcke erstellt | `log_config.json`, `create_empty_blocks`, Mempool-Inhalte |
| `managed EVM key rejected` | Heiße private Schlüssel werden auf einem öffentlichen RPC-Listener | konfiguriert Entfernen Sie `evm_account_private_keys` oder halten Sie RPC privat |

## Minimale Bediener-Checkliste

Bevor Sie einen Knoten an eine andere Maschine oder einen anderen Bediener übergeben:

- `vexod validate --home <home>` besteht.
- `vexod config audit --home <home> --strict` gilt für genau dieses Zuhause.
- `config.json`, geteilte Konfigurationsdateien, `genesis.json` und öffentliche Validator-Metadaten werden überprüft.
- `validator.key.json`, `node.key.json` und `validator.vrf.key.json` werden verschlüsselt oder durch Remote-Signer-/KMS-Schlüsseldokumente ersetzt.
- `network_config.json:p2p.peers` enthält Adressen, die vom Zielcomputer aus wählbar sind, keine Nur-Docker-Namen, es sei denn, der Knoten läuft tatsächlich innerhalb dieses Docker-Netzwerks.
– `network_config.json` öffentliche RPC/P2P-Listener verfügen über TLS-Material, wenn `require_network_safety` aktiviert ist.
- `module_config.json:execution.EVMChainID` wird vor der Verbindung von Web3-Wallets oder Remix festgelegt.
– `mempool_config.json` verfügt über einen WAL-Pfad, wenn der Knoten ausstehende Übertragungen nach dem Neustart wiederherstellen soll.
– `log_config.json` ermöglicht Block-Commit- und Peer-Protokolle, während das Netzwerk hochgefahren wird.

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Validator Node — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Archive Node — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Split Configuration Files — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Key Types — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Config-Based Peers — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Consensus Timing — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Multi-Validator Network — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `network_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod start` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--timeout-propose` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--create-empty-blocks` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--p2p-auth-token` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--rpc-admin-token` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--evm-account-key-env` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--evm-account-key` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `VEXO_KEY_PASSPHRASE` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--passphrase` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--encrypt-keys` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator.key.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `node.key.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator.vrf.key.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `require_network_safety=true` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--key-type bls` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blst-bls12381-minpk-v1` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `genesis.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bls_pop` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `module_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `consensus_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `mempool_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `log_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `data/` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `network_config.json:p2p.node_key_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `shutdown_timeout` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `web3_max_subscriptions_per_connection` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `web3_idle_timeout` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `auth_replay_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `require_auth_replay_store` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `dial_timeout` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `data/p2p_auth_replay.jsonl` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--key-type ed25519` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vrf_key_paths` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vrf_public_key` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `<home>/<name>_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc.evm_account_key_envs` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc.evm_account_private_keys` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_accounts` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_sign` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_signTransaction` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_sendTransaction` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_account_key_envs` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod config paths --home <home>` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `"require_network_safety": true` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `execution_commit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `require_network_safety` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `host:port` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc.address` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.listen_address` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.peers` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.seeds` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.node_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.node_key_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.tls_cert_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.tls_key_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.tls_ca_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.tls_server_name` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.dial_timeout` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `timeout_propose` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `timeout_prevote` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `timeout_precommit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `timeout_commit` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `create_empty_blocks: false` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `execution_commit: "finalized"` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `execution_commit: "qc"` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `round_timeout` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `create_empty_blocks` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod network up` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `make network-e2e` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p_host_template` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc_host_template` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator-%d` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p_advertise_host_template` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc_advertise_host_template` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p_listen_host` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc_listen_host` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
