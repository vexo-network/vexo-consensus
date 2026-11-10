# Vexo Lanes: zertifizierte Batch-Verbreitung für ein stake-gewichtetes Two-Chain-HotStuff

> Locale: de · Deutsch  
> Dokumenttyp: Forschungsmanuskript und Reproduzierbarkeitsprotokoll  
> Status: implementierungsnaher Entwurf; die Ergebnisse stammen von einem Rechner mit emulierten Links und müssen vor einer Veröffentlichung in einem echten Netz wiederholt werden

## Zusammenfassung

Ein leiterbasiertes BFT-Protokoll der HotStuff-Familie ist durch den Uplink seines Leiters begrenzt: Jede Transaktion eines Blocks verlässt eine Maschine einmal pro Validator, der Durchsatz fällt wie `1/(n−1)`, während die Links der anderen Validatoren brachliegen. Vexo Lanes nimmt dem Leiter die Verbreitung ab, ohne das Ordnungsprotokoll zu ersetzen. Jeder Validator versiegelt die Transaktionen, die ihm gehören, zu signierten Batches auf seiner eigenen Lane; Validatoren mit zwei Dritteln des Stakes bestätigen die Speicherung, wodurch der Batch als verfügbar zertifiziert ist; der Leiter schlägt einen Block vor, indem er zertifizierte Batches benennt. Blockformat, Hash, Transaktionswurzel, signierte Bytes, Abstimmungsregel und Two-Chain-Commit-Regel bleiben unverändert.

Auf einem Rechner mit emulierten Links von 100 Mbit/s und 25 ms pro Validator committet Lanes 60.080, 45.323 und 37.099 tx/s bei 4, 7 und 10 Validatoren, gegenüber 21.073, 10.909 und 8.297 tx/s, wenn derselbe Knoten ganze Blöcke rundsendet: 2,9×, 4,2× und 4,5×, mit `n` wachsend. Unterhalb der Kapazität (4.000 tx/s) kostet Lanes bei 4 Validatoren 35 ms Medianlatenz und spart bei 7 und 10 Validatoren 25 ms und 130 ms. Ohne Bandbreitengrenze verliert Lanes 4 % Durchsatz. Die Auswertung deckte neun Fehler auf, darunter einen Leiterplan, unter dem bei gerader Validatorzahl nur Validatoren mit geradem Index Blöcke vorschlugen.

## 1. Forschungsfrage

Kann zertifizierte Batch-Verbreitung unter ein bestehendes stake-gewichtetes HotStuff-Protokoll gelegt werden, ohne zu ändern, worüber Validatoren abstimmen und wie Blöcke committet werden, sodass der Durchsatz mit dem gesamten Uplink aller Validatoren statt mit dem des Leiters wächst? Die Teilfragen betreffen den Leiter-Engpass (RQ1), die Skalierung mit der Validatorzahl (RQ2), die Erhaltung von Sicherheit, Nonce-Reihenfolge und Zensurresistenz (RQ3) und die Kosten bei reichlich Bandbreite (RQ4). Die Hypothesen: H1, der Upload des Baselines pro Transaktion wächst linear mit `n`, der von Lanes bleibt nahe der Transaktionsgröße; H2, der Vorsprung wächst mit `n`; H3, ohne Bandbreitengrenze ist Lanes nicht schneller und kostet höchstens einen Roundtrip.

## 2. Stand der Forschung und Neuheitsgrenze

Die Trennung von Verbreitung und Ordnung ist etabliert: Narwhal und Tusk, Bullshark und Shoal, Stratus, Mir-BFT und ISS, Autobahn sowie Aptos' Quorum Store. Dieses Manuskript beansprucht weder diese Trennung, Verfügbarkeitszertifikate, Lanes pro Validator noch WAN-Ergebnisse als neu. Der Beitrag ist ingenieurmäßig: ein Umbau, der Block, Header, Signatur und Commit-Regel bitgenau erhält; Lane-Eigentum nach Absender mit einem Fallback gegen Zensur; Authentifizierung des Vorschlags vor jedem Abruf; und ein reproduzierbares Messgerüst samt aufgedeckter Fehler.

## 3. Systemmodell

`n` Validatoren mit stake-gewichteter Stimmkraft; byzantinische Validatoren halten weniger als ein Drittel. Quoren umfassen `>= 2/3` der Stimmkraft. Partielle Synchronität, authentifizierte Kanäle und domänengetrennte Signaturen. Validatoren dürfen abstürzen, doppelt signieren, Batches zurückhalten, ihre Lane fluten oder gefälschte Vorschläge senden.

## 4. Protokoll

**Lanes und Zertifikate.** Jeder Validator versiegelt seine Transaktionen zu einem `lanes.Batch` bei 512 KiB oder 5 ms nach der ersten Transaktion, begrenzt durch `lanes.MaxBatchTxs` und `lanes.MaxBatchBytes`. Eine schwach belastete Lane, deren vorheriger Batch unter 16 KiB lag, wartet zusätzlich auf dessen Zertifikat, höchstens 50 ms. Validatoren bestätigen gespeicherte Batches in der Domäne `DomainLaneBatchAck`; Bestätigungen von `>= 2/3` der Stimmkraft bilden ein `BatchCert`, sodass mindestens ein ehrlicher Validator den Batch hält. Erneute Übertragungen verwenden exponentielles Backoff.

**Eigentum und Fallback.** Eine Transaktion gehört zur Lane `lanes.Owner(members, sender)`. Alle Transaktionen eines Absenders laufen über eine Lane in Aufnahmereihenfolge, daher vertauschen Lanes keine Nonce-Folge. Trägt nach `lanes.FallbackDelay` (3 s) kein gespeicherter Batch eine fremde Transaktion, batcht der Halter sie selbst; ein fehlerhafter Eigentümer kann verzögern, aber nicht zensieren.

**Lane-Vorschläge.** Der Leiter nimmt nur zertifizierte Batches, pro Lane in Folge bis zum ersten unzertifizierten, abwechselnd bis zur Blockgrenze, signiert den Block wie bisher und sendet ein `lane_proposal` mit Batch-IDs, abgelehnten Positionen (`Excluded`) und zusätzlichen Transaktionen (`Extra`).

**Authentifizierung vor dem Abruf.** Der Empfänger prüft Leiter und Signatur, bevor er irgendetwas abruft. Hängt die Validatormenge von einem noch nicht ausgeführten Block ab, wird der Vorschlag behalten (höchstens 16) und nach der nächsten Ausführung erneut beurteilt. Der Block wird aus gespeicherten Batches rekonstruiert und nur beurteilt, wenn Hash und Transaktionswurzel dem signierten Header entsprechen; sonst fordert der Validator mit `proposal_request` den ganzen `proposal` an.

**Stake-proportionaler Leiterplan.** Der Leiter von View `v` ist `validator.SelectProposer(validators, v)`: Validatoren besitzen `VotingPower` aufeinanderfolgende Slots aus `[0, W)`, View `v` fällt auf Slot `v·S mod W`, mit einem zu `W` teilerfremden Schritt ab `ceil(W·(φ−1))`. Jedes Fenster von `W` Views gibt jedem Validator genau seinen Anteil. Der Plan hängt nur von der View ab.

**Ressourcengrenzen.** 64 MiB pro Autor in einem Speicher von 256 MiB; Ablauf nach 30 s; höchstens zwei Auslieferungen pro Batch und Peer; höchstens `consensus.MaxLaneRefs` Batches pro Vorschlag; eine versionierte Binärkodierung.

## 5. Korrektheitsargument

Sicherheit: Abstimmungs- und Commit-Regel bleiben unverändert, und ein Validator stimmt nur für einen Block, dessen rekonstruierter Hash dem signierten Header entspricht; Lanes können eine Stimme verzögern, aber ihren Gegenstand nicht ändern. Verfügbarkeit: Jeder benannte Batch wird von einem ehrlichen Validator gehalten, notfalls wird der ganze Vorschlag abgerufen. Lebendigkeit: Nach der Stabilisierung werden ehrliche Batches zertifiziert und vorgeschlagen; ein zurückliegender Validator beurteilt Vorschläge direkt nach der Ausführung. Dies ist ein Argument, kein maschinell geprüfter Beweis.

## 6. Experimentelle Methode

Behandlungen: Lanes (Standardknoten) gegen die Baseline mit `DisseminateByProposal`, bei der der Leiter ganze Blöcke rundsendet; alles andere ist geteilt. Alle Validatoren laufen in einem Prozess auf einem Apple M4; jeder Upload läuft über einen virtuellen Link mit fester Rate und Latenz, Konsensnachrichten vor Massenverkehr, ohne Verlust und Jitter. Clients senden 89-Byte-Transaktionen; Durchsatzläufe übergeben 150.000 Transaktionen auf einmal, Latenzläufe 20.000 offen mit 4.000 tx/s (`VEXO_RATE`). Die 10-Validator-Durchsatzzellen nutzen für beide Behandlungen 5 s Timeout, weil die Baseline mit dem Standard nach Höhe 6 stehen bleibt. Jede Zelle wurde dreimal gemessen.

## 7. Ergebnisse

| Validatoren | Lanes tx/s | Baseline tx/s | Faktor | Lanes B/tx | Baseline B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Alle Läufe committeten alle 150.000 Transaktionen. Die Baseline zahlt allein für Vorschläge `s·(n−1)` Bytes pro Transaktion, Lanes 0,2–0,4 B/tx. Das stützt H1 und H2.

| Validatoren | Lanes p50 / p99 | Baseline p50 / p99 | Lanes Vorschlag B/tx | Baseline Vorschlag B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

Bei 4 Validatoren kostet die Zertifizierung etwa einen Roundtrip; ab 7 Validatoren verzögert der Leiter-Uplink der Baseline schon bei dieser Last, und bei 10 Validatoren hielt nur Lanes mit der angebotenen Last Schritt. H3 gilt nur für kleine Mengen.

| Behandlung | tx/s | Upload B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Baseline | 114,522 (113,095–116,408) | 147.4–148.3 |

Ohne Bandbreitengrenze ist die Baseline 4 % schneller; Lanes lädt dennoch weniger als die Hälfte der Bytes hoch. Die Baseline ist am Leiter-Link begrenzt, Lanes in dieser Matrix am gemeinsamen Rechner.

## 8. Von der Auswertung aufgedeckte Fehler

1. **Leiterplan nach Parität.** `(height−1+round) mod n` ließ bei geradem `n` nur gerade Indizes vorschlagen; ersetzt durch den stake-proportionalen Plan (`node/leader_schedule_test.go`).
2. **Wiederholung committeter Transaktionen.** Späte Gossip-Nachrichten wurden erneut aufgenommen (152.629 Commits für 150.000 Transaktionen).
3. **Quadratischer Ablauf des Seen-Caches.** Jede Aufnahme durchsuchte den ganzen Cache; jetzt älteste zuerst und begrenzt.
4. **Neuübertragungssturm.** Feste 200 ms auf einem überlasteten Link; jetzt exponentielles Backoff.
5. **Unbegrenzt duplizierte Evidenz.** Jetzt einmal pro Verstoß in einem Fenster von 1.024.
6. **Anfragen nach Commit-Bodies.** Jetzt eine Anfrage, Wiederholung nach einer Sekunde.
7. **Vorschläge nur bei Neuübertragung beurteilt.** Ein 10-Validator-Lauf von vier brach auf 2.280 tx/s ein; jetzt aufbewahrt und nach der Ausführung beurteilt (`consensus/transport_reactor_test.go`).
8. **Kosten pro Batch bei geringer Last.** Batches mit zwei Transaktionen überlasteten den Rechner; behoben durch Lane-Pacing.
9. **Falsch benannte Latenzmetrik.** Jetzt Latenz von der Einreichung bis zum Commit pro Transaktion.

## 9. Gefährdungen der Validität

- Emuliertes Netz auf einem Rechner ohne Verlust und Jitter.
- Geteilte CPU begrenzt Lanes bei großem `n`.
- Die Baseline braucht bei 10 Validatoren 5 s Timeout; eine kompaktere Kodierung würde den Abstand verringern.
- Die Wartezeit von 300 ms vor dem Abruf ganzer Vorschläge ist auf überlasteten Links zu kurz.
- Nur kleine Überweisungen; nur drei Wiederholungen (34,9k–39,4k tx/s bei 10 Validatoren).
- Ed25519-Zertifikate verketten Signaturen; BLS würde sie verkleinern.

## 10. Reproduktion

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` ändert den Timeout; ohne `VEXO_UPLINK_MBIT` und `VEXO_LATENCY_MS` läuft der unbegrenzte Link. Jede veröffentlichte Zahl braucht Revision, Rechner, Go-Version und vollständige Logzeilen.

<!-- vexo-docs:technical-parity -->

## Anhang zur technischen Parität

Diese Implementierungs- und Experimentnamen bleiben unübersetzt:

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
