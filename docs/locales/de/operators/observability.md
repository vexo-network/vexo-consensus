> Locale: de · Deutsch

# Leitfaden zur Beobachtbarkeit

In diesem Leitfaden wird erläutert, wie Sie anhand von RPC, Metriken, Protokollen und Release-Beweisen feststellen können, ob ein Vexo-Knoten fehlerfrei ist.

Es richtet sich an Bediener, die praktische Signale benötigen: Was ist zu beachten, was jede Zahl bedeutet und wann ein Wert als gefährlich eingestuft werden sollte.

## Auf einen Blick

Wenn ein Knoten falsch aussieht, überprüfen Sie diese der Reihe nach:

1. `running` und `latest_height` in `/v1/status`
2. `latest_finalized_height` und Peer-Anzahl
3. `round_timeout`, Vorschlags-/Abstimmungslatenz, Mempool-Größe und Commit-Latenzmetriken
4. Unterzeichnerfehler, Snapshot-Zustand und Wiedergabezustand
5. Peer-Verbote und Peer-Dial-Fehler

Diese Reihenfolge ist wichtig, weil sie „der Prozess läuft“ von „die Kette macht tatsächlich sichere Fortschritte“ trennt.

## Kernendpunkte

| Endpunkt | Verwenden Sie |
|---|---|
| `/v1/status` | Schneller Prozess, Höhe, App-Hash, Endgültigkeit und Peer-Zusammenfassung |
| `/v1/metrics` | JSON-Metriken für Dashboards und Automatisierung |
| `/metrics/text` | Prometheus-kompatible Textmetriken |
| `/v1/diagnostics` | Kombinierte Bereitschafts-, Leistungs-, Status-, Peers-, Speicher- und Metrikprüfungen |
| `/v1/finality/latest` | Neueste Endgültigkeitsnachweise für Licht-Client- und Sicherheitsprüfungen |
| `/v1/state/latest` | Aktuelle Status-Root- und Validator-Set-Bindung |
| `/v1/recovery/report` | Konsistenzdiagnose nach Absturz/Neustart |
| `/v1/snapshot` | Snapshot-Zustand und Metadaten exportieren |

Admin-Endpunkte wie Prune, Replay und Consensus Control sollten normalerweise nur über Loopback, ein Betreibernetzwerk, mTLS oder ein authentifiziertes Gateway erreichbar sein. Bereichsbezogene Admin-Tokens bleiben optional und werden bei der Konfiguration erzwungen.

## Lesen `/v1/status`

Wichtige Felder:

| Feld | Bedeutung | Betreiberhinweis |
|---|---|---|
| `running` | Der Knotenprozess wurde gestartet und besitzt den Laufzeitstatus | `true` allein beweist nicht die Lebendigkeit des Konsenses |
| `latest_height` | Neueste lokal festgeschriebene App-Höhe | Muss im Laufe der Zeit in einem Live-Validator-Netzwerk zunehmen |
| `latest_finalized_height` | Neueste HotStuff dreikettige endgültige Höhe | Sollte nicht unbegrenzt hinter der ausgeführten/festgeschriebenen Höhe | zurückbleiben
| `latest_app_hash` | App-Commit-Hash | Sollte mit Gleichgesinnten auf gleicher Höhe übereinstimmen |
| `peer_count` | Abwärtskompatible verbundene/bewertete Peer-Zusammenfassung | Bevorzugen Sie die spezifischeren Peer-Felder unten |
| `active_peer_count` | Aktive Transportsitzungen, wenn der Transport sie melden kann | Bestes schnelles Signal für Live-P2P-Konnektivität |
| `configured_peer_count` | Konfigurierte oder gelernte Peer-Adressen | Die Erreichbarkeit ist nicht garantiert |
| `scored_peer_count` | In der Punktetabelle bekannte Gleichaltrige | Nützlich für den Verlauf von Sperren/Ratenbegrenzungen, nicht für den Nachweis von Live-Sitzungen |
| `banned_peers` | Gleichaltrige sind derzeit durch die Punkterichtlinie gesperrt | Spitzen deuten auf einen Angriff, eine schlechte Peer-Konfiguration oder zu strenge Grenzwerte hin |

Gesundes Beispiel für ein 4-Validator-Single-Host-Netzwerk: `running=true`, `latest_height` steigend, `latest_finalized_height` vorhanden, `active_peer_count` nahe `3` und `banned_peers=0`.

## Prometheus-Metriken

Der Textendpunkt macht Messgeräte verfügbar wie:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` wird für ältere Dashboards beibehalten. Neue Dashboards sollten `vexo_active_peer_count`, `vexo_configured_peer_count` und `vexo_scored_peer_count` separat darstellen.

## Empfohlene Alarmregeln

Passen Sie die Zahlen für die tatsächliche Validatoranzahl, das Blockintervall, die Latenz und die Hardware an. Dies sind Ausgangspunkte, keine universellen Konstanten.

| Warnung | Startbedingung | Warum |
|---|---|---|
| Knoten unten | `vexo_node_running == 0` für 1 Minute | Prozess/Laufzeit gestoppt |
| Höhe ins Stocken geraten | `latest_height` unverändert für 2-3 erwartete Blockintervalle | Konsens oder Umsetzung ins Stocken geraten |
| Endgültigkeit ins Stocken geraten | `latest_finalized_height` unverändert, während Blöcke weiterhin ausgeführt werden | Finalitätspfad oder Quorumproblem |
| Keine aktiven Peers | `vexo_active_peer_count == 0` für 1 Minute auf einem nicht isolierten Knoten | P2P-Ausfall, Authentifizierungskonflikt oder Adressproblem |
| Peer-Anzahl zu niedrig | aktive Peers unter Quorum-Konnektivitätsziel | Partitions- oder Bootstrap-Problem |
| Runde Timeout-Spitze | Timeout-Zähler wächst schneller als normale Basislinie | Latenz, Antragstellerfehler oder Netzwerkpartition |
| Commit-Latenz hoch | p95/p99 nähert sich dem Konsens-Timeout-Budget | Speicher-/Laufzeitüberlastung |
| Mempool-Druck | Mempool-Größe wächst mehrere Minuten lang | Gebührenpolitik, Spam oder Blockkapazitätsproblem |
| Snapshot ungesund | `vexo_snapshot_healthy == 0` | Statussynchronisierungs-/Wiederherstellungsrisiko |
| Wiederholung ungesund | `vexo_replay_healthy == 0` | Determinismus- oder Zustandskonsistenzrisiko |
| Unterzeichnerfehler | `vexo_validator_signing_failures > 0` | KMS/Remote-Unterzeichner/Richtlinienfehler |
| Abstimmungsfehler | `vexo_post_commit_reconciliation_failures > 0` | Dauerhafter Nachweis oder Verpflichtung zur Reparatur erforderlich |
| Verbotener Peer-Spike | verbotene Peers steigen plötzlich | Angriff, falsch konfigurierte Peers oder Problem mit dem Bewertungsschwellenwert |

## Empfohlene Startschwellenwerte

Verwenden Sie diese als anfängliche Alarmwerte und optimieren Sie sie dann nach einer echten langfristigen Basislinie:

| Signal | Warnung | Kritisch | Erste Aktion |
|---|---:|---:|---|
| Höhenrate | unter 50 % des für 2 Fenster erwarteten Werts | Nullwachstum für 2-3 Blockintervalle | Alle Validatoren vergleichen, Antragsteller-/Signatur-/Peer-Protokolle prüfen |
| Endgültige Höhenverzögerung | wächst für 5 Minuten | wächst, während die ausgeführte Höhe 10 Minuten lang weiter zunimmt | Überprüfen Sie die QC-/Finalitätsnachweisprotokolle und den vom Validator festgelegten Hash |
| Aktive Kollegen | unter Quorum-Konnektivitätsziel | null aktive Peers | Überprüfen Sie die beworbene Adresse, TLS/Authentifizierung, Genesis/Chain-ID-Nichtübereinstimmung |
| Runden-Timeouts | 3x normale Grundlinie | kontinuierliche Timeout-Schleife | Timeout-Budget erhöhen oder Latenz/Partition untersuchen |
| Vorschlagslatenz p95 | über 50 % von `timeout_propose` | über 80 % von `timeout_propose` | Profilvorschlager, Mempool, DA-Verpflichtung, Festplatte |
| Abstimmungslatenz p95 | über 50 % des Prevote-/Precommit-Budgets | über 80 % des Budgets | Überprüfen Sie CPU, Unterzeichner, Transport, Klatsch-Gegendruck |
| Commit-Latenz p95 | über 50 % des Blockintervalls | über 80 % des Blockintervalls | Überprüfen Sie LevelDB, Statuswurzeln, EVM-Ausführung, Snapshots |
| Mempool-Größe | 5 Minuten lang steigern | nahe `max_txs` oder anhaltende Ersatzabwanderung | Überprüfen Sie die Grundgebühr, die Mindestgebühr, die Sendegültigkeit und den Spam |
| Unterzeichnerfehler | jeder Wert ungleich Null | wiederholte Ausfälle in einem Höhenfenster | Stoppen Sie den Validator, wenn ein Doppelzeichenschutz oder eine Schlüsselinkongruenz auftritt |
| Snapshot-Gesundheit | eine fehlgeschlagene Prüfung | wiederholter fehlgeschlagener Export/Verifizierung/Wiederherstellung | Statussynchronisierungsbereitstellung anhalten und Wiederherstellungsbericht ausführen |
| Gesundheit wiedergeben | ein strikter Wiederholungsfehler | Nichtübereinstimmung der Wiedergabe auf der letzten sicheren Höhe | Datenverzeichnis beibehalten und unsicheres Upgrade/Release stoppen |
| Gesperrte Peers | plötzlicher Anstieg | Viele Peers nach Konfigurations-Rollout gesperrt | Überprüfen Sie die Score-Obergrenzen, die TLS-CA, die Peer-Identität, den optionalen Authentifizierungsnachweis und den Zeitversatz |

Die wichtigste Regel: Warnung vor **Änderungen im Laufe der Zeit**. Eine einzelne Zahl kann irreführend sein; Höhenrate, Finalitätsverzögerung, Peer-Abwanderung, Mempool-Wachstum und Unterzeichnerfehler zusammen erzählen die wahre Geschichte.

## Incident-Triage-Matrix

| Situation | Wahrscheinliche Schicht | Was zu bewahren ist | Sicherer nächster Schritt |
|---|---|---|---|
| Höhe gestoppt, Artgenossen gesund | Konsens/Unterzeichner/Laufzeit | Konsensprotokolle, Unterzeichnerprotokolle, Mempool-Beispiel | Schlüssel des Antragstellers überprüfen und Zeitüberschreitungsprotokolle abrunden |
| Peers wurden nach der Bereitstellung gelöscht | Netzwerk/Konfiguration | Netzwerkkonfiguration, TLS-Zertifikate, Addrbook, Peer-Protokolle | Rollback der angekündigten Adress-/TLS-/Authentifizierungsänderung |
| App-Hashes unterscheiden sich auf gleicher Höhe | Ausführung/Speicherung | Datenverzeichnisse, Blockaufzeichnungen, App-Protokolle, Wiedergabeausgabe | Betroffene Knoten anhalten und strikte Wiederholung ausführen |
| Endgültigkeitsnachweis abgelehnt | Endgültigkeits-/Validatorsatz | Proof-JSON, Validator auf Proof-Höhe eingestellt | Überprüfen Sie den vom Validator festgelegten Hash und signieren Sie die Byte-Domäne |
| Snapshot-Wiederherstellung schlägt fehl | Zustandssynchronisierung/-speicherung | Snapshot-Datei, Prüfsumme, Statuswurzeln, Wiederherstellungsprotokolle | Versuchen Sie es nicht erneut mit Live-Daten. Wiederherstellung im sauberen Verzeichnis |
| Remote-Unterzeichner lehnt Anfragen ab | Schlüsselverwahrung | Unterzeichner-Überwachungsprotokoll, Schutzdatei, Nonce-Datei, Knotenprotokolle | zwischen politischer Ablehnung und Transportausfall unterscheiden |
| Verbotene Peers-Spitze | P2P/Sicherheit | Peer-Score-Schnappschüsse und Sperrgründe | Untersuchen Sie fehlerhafte Gerüchte oder geteilte falsche Konfigurationen |

Ziehen Sie bei Vorfällen die Datensicherung der „Bereinigung“ vor. Das Löschen von WALs, Addrbooks, Signer Guards oder LevelDB-Verzeichnissen kann die Beweise zerstören, die zur Unterscheidung eines Fehlers von einem Bedienerfehler erforderlich sind.

## Protokollieren Sie Ereignisse, die Sie behalten möchten

Strukturierte Protokolle sollten mit Knoten-ID, Validator-ID, Ketten-ID, Höhe, Runde, Block-Hash und Peer-ID (sofern relevant) aufbewahrt werden.

Wichtige Ereignisse:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

Archivieren Sie für Release-Kandidaten Protokolle zusammen mit Metrikbeispielen, Pprof-Beispielen, Konfigurationsdateien, Genesis, binären Prüfsummen und Beweismanifesten.

## First Response Playbook

Wenn ein Bediener ein Problem sieht:

1. Überprüfen Sie `/v1/status` auf mindestens zwei Validatoren.
2. Vergleichen Sie `latest_height`, `latest_finalized_height`, `latest_app_hash` und die Peer-Anzahl.
3. Überprüfen Sie `/v1/diagnostics` auf fehlende Funktionen oder fehlerhafte Speicher-/Wiedergabe-/Snapshot-Prüfungen.
4. Überprüfen Sie Peer-Ereignisprotokolle auf Authentifizierungs-, TLS-, Genesis-, Ketten-ID- oder Backoff-Fehler.
5. Überprüfen Sie die Mempool- und Grundgebührenmetriken, wenn Txs nicht enthalten sind.
6. Überprüfen Sie die Protokolle des Unterzeichners und des Remote-Unterzeichners, wenn Validatorsignaturen fehlschlagen.
7. Exportieren Sie den Wiederherstellungsbericht, bevor Sie Daten löschen oder ändern.
8. Wenn ein Endgültigkeitskonflikt vermutet wird, stoppen Sie die Automatisierung, bewahren Sie Protokolle/Beweise auf und führen Sie die Endgültigkeitskonflikterkennung durch.

## Dashboard-Layout

Ein nützliches Dashboard besteht normalerweise aus fünf Zeilen:

1. **Lebendigkeit**: laufender Knoten, letzte Höhe, endgültige Höhe, Höhenrate.
2. **Konsenslatenz**: Runden-Timeouts, Vorschlag/Abstimmung/Commit p95 und p99.
3. **Netzwerk**: aktive/konfigurierte/bewertete Peers, gesperrte Peers, Peer-Fenstermeldungen.
4. **Ausführung**: Mempool-Größe, Gas-/Basisgebühr, Tx-Anzahl, Commit-Latenz.
5. **Wiederherstellung und Sicherheit**: Snapshot-Zustand, Wiedergabezustand, Unterzeichnerfehler, Abstimmungsfehler.

Halten Sie Dashboards langweilig. Das Ziel besteht nicht darin, jeden internen Zähler anzuzeigen. Es geht darum, gefährliche Zustände deutlich zu machen, bevor Prüfer voneinander abweichen oder Benutzer blockierte Transaktionen bemerken.

## Beweise aus Observability freigeben

Für einen Release Candidate bedeutet Observability nicht nur Live-Überwachung. Es wird zum Beweis:

1. Erfassen Sie die Basiswerte `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest` und `/v1/recovery/report` von jedem Validator.
2. Führen Sie die Last für die gewählte Dauer und Geschwindigkeit aus.
3. Fügen Sie mindestens einen Neustart, eine Peer-Unterbrechung und eine Snapshot-Export-/Überprüfungs-/Wiederherstellungsübung ein.
4. Sammeln Sie endgültige Messwerte von jedem Validator.
5. Speichern Sie die Vorher/Nachher-Beispiele, Protokolle, Pprof-Beispiele, Prüfprotokolle des Unterzeichners und das Beweismanifest in `dist/`.

Ein gutes Beweisbündel lässt einen Prüfer antworten: Ist die Größe gewachsen, hat sich die Endgültigkeit weiterentwickelt, haben sich die Peers erholt, wurden TXS-Commits durchgeführt, wurden Snapshots überprüft, blieb die Wiedergabe fehlerfrei, haben die Unterzeichner Doppelsignierungen vermieden und hat die genaue Release-Binärdatei die Ergebnisse hervorgebracht?

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Core Endpoints — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Reading `/v1/status` — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Prometheus Metrics — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Suggested Alert Rules — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Suggested Starting Thresholds — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Incident Triage Matrix — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Log Events to Keep — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: First Response Playbook — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Dashboard Layout — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Release Evidence From Observability — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `/v1/status` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/metrics` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/metrics/text` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/diagnostics` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/finality/latest` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/state/latest` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/recovery/report` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `/v1/snapshot` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `latest_height` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `latest_finalized_height` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `latest_app_hash` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `active_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `configured_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `scored_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `banned_peers` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `banned_peers=0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_node_running` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_latest_height` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_active_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_configured_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_scored_peer_count` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_banned_peers` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_height_rate_per_minute` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_round_timeouts` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_proposal_latency_p95_nanos` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_vote_latency_p95_nanos` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_commit_latency_p95_nanos` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_mempool_size` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_snapshot_healthy` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_replay_healthy` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_validator_signing_failures` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_post_commit_reconciliation_failures` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_node_running == 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_active_peer_count == 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_snapshot_healthy == 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_replay_healthy == 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_validator_signing_failures > 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `vexo_post_commit_reconciliation_failures > 0` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `timeout_propose` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `max_txs` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `node_running` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `rpc_listening` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `p2p_listening` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `peer_configured` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `peer_connected` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `peer_disconnected` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `peer_dial_failed` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `peer_banned` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `consensus_loop_running` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `block_committed` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `round_timeout` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `validator_signing_failure` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evidence_received` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evidence_applied` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `snapshot_exported` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `replay_checked` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `upgrade_halt` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `upgrade_applied` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `dist/` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
