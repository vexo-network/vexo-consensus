> Locale: de · Deutsch

# Hinzufügen eines Validators

In dieser Anleitung wird der Bedienerablauf zum Hinzufügen eines Validators zu einem Vexo-Netzwerk beschrieben.

Der genaue Zulassungspfad hängt von der Einsatz- und Governance-Richtlinie der Kette ab. Der Validator muss mindestens im Kettenstatus dargestellt werden, über gültige Anmeldeinformationen verfügen und Teil einer Aktualisierung des Validatorsatzes mit Höhenversion sein.

## 1. Validator Home initialisieren
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
Für einen BLS-Validierungsschlüssel:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
Legen Sie `VEXO_KEY_PASSPHRASE` fest, bevor Sie diese Befehle ausführen, oder übergeben Sie `--passphrase` für eine einmalige lokale Einrichtung.

Wenn Sie einen BLS-Validator in eine bestehende Kette aufnehmen, schließen Sie die generierten `bls_pop`-Metadaten in den Validator-Aktualisierungsvorschlag ein.
Der Standard-BLS-Schlüsselpfad verwendet `blst-bls12381-minpk-v1`; Verwenden Sie `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` nur für Referenz-/Kompatibilitätstests.

Archivieren Sie den generierten öffentlichen Schlüssel:
```bash
vexod keys show --home .vexo-validator-new --json
```
Behalten Sie auch den generierten `node.key.json` bei. Es signiert P2P-Handshakes für `network_config.json:p2p.node_id`; Es handelt sich nicht um einen Validator-Konsensschlüssel und sollte nicht als Kontoschlüssel wiederverwendet werden.

## 2. Netzwerkadressen und Peers konfigurieren

Bearbeiten Sie `.vexo-validator-new/network_config.json` und legen Sie lokale Abhöradressen sowie persistente Peers fest:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
Verlassen Sie sich bei Produktionsvalidatoren nicht auf langlebige Befehlszeilen-Netzwerküberschreibungen. Behalten Sie persistente Peer-Adressen in `network_config.json`.

Verwenden Sie separate Adressrollen:

- `p2p.listen_address` und `rpc.address` sind lokale Bindungsadressen für diesen Computer oder Container.
– `p2p.node_id` ist die Peer-Identität dieses Knotens. Halten Sie es stabil, nachdem Ihre Kollegen es gelernt haben.
– `p2p.node_key_path` verweist auf den lokalen Handshake-Signaturschlüssel für diese Peer-Identität.
– `p2p.peers` enthält Wählziele, die dieser Knoten verwendet, um andere Peers zu erreichen; Kartenschlüssel sollten die `p2p.node_id`-Werte der Remote-Knoten sein.
– Die Validator-Metadaten `p2p_address` und `rpc_address` sollten öffentlich angekündigte Adressen enthalten, keine Nur-Docker-Dienstnamen, es sei denn, das Netzwerk ist absichtlich privat.

## 3. Validator-Zulassung einreichen

Erstellen Sie zum Beispiel Absteckflüsse eine Abstecktransaktion:
```bash
vexod staking --help
```
Die Zulassungstransaktion des Validators sollte Folgendes umfassen:

- Validator-ID
- Validator-Adresse
- Konsens öffentlicher Schlüssel
- Stimmrechts- oder Anteilsreferenz
- Validator-Provisionsbasispunkte, wenn die Kette Self-Service-Provisionsaktualisierungen zulässt
- P2P-`node_id`-Metadaten, wenn die Kette Genesis-/Validator-Metadaten verwendet, um Peer-Maps vorab zu erstellen
- Metadaten öffentlicher P2P-Adressen
– öffentliche RPC-Adressmetadaten, falls öffentlich
– BLS-Besitznachweis-Metadaten, wenn BLS aktiviert ist

Das Validator-Update muss bei einer bestimmten Höhe wirksam werden und einen neuen vom Validator festgelegten Hash erzeugen.

Nachdem der Validator aktiv ist, können Betreiber den Belohnungsstatus über das Absteckmodul offenlegen:
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. Überprüfen Sie die Aktualisierung des Validator-Sets

Nach der Aktualisierung der Höhe:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
Überprüfen Sie:

- Validator erscheint im höhenspezifischen Satz
- Das Stimmrecht ist korrekt
- Der vom Validator festgelegte Hash wurde wie erwartet geändert
- Endgültigkeitsnachweise beziehen sich auf die korrekte Höhe des Validatorsatzes

## 5. Planen Sie die Validator-Schlüsselrotation

Validatorschlüssel können rotiert werden, indem ein nächstes Schlüsseldokument mit nicht überlappenden Metadaten `active_from` und `active_until` vorbereitet und dann der Knoten mit dem zusätzlichen Rotationsschlüssel gestartet wird:
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
Zum Zeitpunkt der Signatur verwendet der Knoten den Schlüssel, dessen aktives Fenster die Konsenshöhe enthält. Für Remote-Signer-Schlüsseldokumente gelten dieselben Richtlinien-, Authentifizierungstoken- und Doppelsignaturschutzanforderungen.

## 6. Validator starten
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
Beim Start gibt es keinen Netzwerkmodusschalter. Verwenden Sie `config audit --strict` vor dem Start, wenn erwartet wird, dass das Netzwerk die Sicherheitsannahmen des öffentlichen Netzwerks erfüllt.

## 7. Überwachen

Anschauen:

- Vorschlags-/Abstimmungslatenz
- Runden-Timeouts
- Validator-Signaturfehler
- Peer-Verbote
- Mempool-Größe
- Latenz begehen
- Snapshot-/Replay-Zustand

Verwendung:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## Sicherheitshinweise

- Verwenden Sie Prüfschlüssel niemals über unabhängige Ketten hinweg wieder.
– Lassen Sie die Remote-Signer-Richtlinie für Produktionsvalidatoren aktiviert.
- Lassen Sie einen BLS-Validator nicht ohne einen Besitznachweis oder eine gleichwertige Abwehr gegen betrügerische Schlüssel zu.
- Zerschneiden oder sperren Sie einen Validator nicht ein, ohne dass verifizierte Beweise mit dem korrekten Validatorsatz für die Beweishöhe verknüpft sind.

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: 1. Initialize Validator Home — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: 2. Configure Network Addresses and Peers — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: 3. Submit Validator Admission — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: 4. Verify Validator Set Update — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: 5. Plan Validator Key Rotation — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: 6. Start Validator — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: 7. Monitor — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Safety Notes — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `VEXO_KEY_PASSPHRASE` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `--passphrase` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bls_pop` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blst-bls12381-minpk-v1` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `node.key.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `network_config.json:p2p.node_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `.vexo-validator-new/network_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `network_config.json` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.listen_address` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc.address` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.node_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.node_key_path` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p.peers` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p_address` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc_address` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `node_id` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `active_from` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `active_until` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `config audit --strict` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
