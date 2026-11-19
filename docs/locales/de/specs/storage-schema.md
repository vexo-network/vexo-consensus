# Speicherschema

> Locale: de · Deutsch
> Dieses Dokument ist eine direkte deutsche Übersetzung der englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.


## Reihenfolge zum Einstieg

Dieses Dokument erklärt die normative Spezifikation von Storage Schema. Wenn du neu bist, lies in dieser Reihenfolge.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Diese Reihenfolge passt zur Art, wie man das Dokument liest: zuerst Umfang und Zustand, dann Nachrichten-, Safety- und Liveness-Regeln, und am Ende die Beweise.

## Überblick

Dieses Dokument hilft dabei, durable storage namespaces, key schema und recovery marker zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/de/specs/storage-schema.md`

## Warum dieses Dokument lesen

- durable storage namespaces, key schema und recovery marker
- Prüfe zuerst MUST/SHOULD/MAY-Sätze in der englischen Quelle.
- Dieses lokalisierte Dokument unterstützt das Verständnis; Audit-, Release- und Sicherheitsentscheidungen erfolgen anhand der englischen Quelle.

## Was danach möglich sein sollte

- Erklären, welche Implementierungs- oder Betriebsentscheidung dieses Dokument unterstützt.
- Normative Anforderungen der englischen Quelle mit der aktuellen Netzwerkkonfiguration verbinden.
- Vor dem Kopieren von Beispielen chain ID, validator ID, fee/gas und Peer-Adressen prüfen.

## Checkliste für sichere Nutzung

- Prüfe zuerst MUST/SHOULD/MAY-Sätze in der englischen Quelle.
- Übersetze keine Befehle, config key, RPC-Namen, JSON-Felder oder Code-Bezeichner.
- Passe Beispielwerte vor dem Kopieren an chain ID, validator ID, fee/gas und Peer-Adressen deines Netzwerks an.
- Nach Änderungen `make docs-check` ausführen, um den lokalen Dokumentbaum und die Übersetzungs-Guards zu prüfen.

## Worauf zu achten ist

- Dieses lokalisierte Dokument unterstützt das Verständnis; Audit-, Release- und Sicherheitsentscheidungen erfolgen anhand der englischen Quelle.
- Bei Implementierungsänderungen müssen englische Quelle und alle lokalisierten Dokumente im selben Change aktualisiert werden.

## Unverändert zu behaltende Schnittstellen

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Struktur der englischen Quelle

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/specs/storage-schema.md)

## Pebble-Backend und binäre Datensätze

Das Standard-Backend ist Pebble (`github.com/cockroachdb/pebble/v2`), geöffnet mit 256 MiB Block-Cache und 64 MiB Memtables. Datensätze werden in der kanonischen Binärkodierung des Pakets `types` gespeichert: minimale Uvarints, längenpräfixierte Bytefolgen und Listen, Hashes als ihre 32 Bytes, ein optionaler Wert hinter einem Präsenzbyte `0` oder `1` und Felder in der Reihenfolge der deklarierenden Struktur. Ein Decoder weist alles andere zurück, auch überzählige Bytes, sodass ein beschädigter Datensatz gemeldet und nicht als anderer Wert gelesen wird; jeder Datensatz hat genau eine Kodierung. Ein von einem früheren Build (LevelDB, JSON-Datensätze) geschriebenes Datenverzeichnis ist nicht lesbar und wird nicht konvertiert: Pebble verweigert das Öffnen mit einem Fehler zu `format major version 1`. Einen solchen Knoten aus einem Snapshot wiederherstellen oder in ein leeres Datenverzeichnis synchronisieren.

Zugehörige Bezeichner:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Zustandshistorie (Undo-Log) und Tree-Cache

Die Zustandshistorie ist ein Undo-Log von Versionen je Schlüssel. Ändert sich ein Schlüssel bei einer Höhe (die Schreibvorgänge eines Blocks bei seiner Höhe, ein direkter Schreibvorgang (`Set`, `Delete`, `SetBatch`) nach dem ersten committeten Zustand bei der nächsten committeten Höhe), speichert der Store den Wert, den der Schlüssel davor hatte, Abwesenheit eingeschlossen, als Version des Schlüssels bei dieser Höhe, einmal je Schlüssel und Höhe; direkte Schreibvorgänge vor dem Block, der ihre Höhe committet, behalten die Version, die der erste von ihnen gespeichert hat (`hdirect` nennt diese Höhe). Der Wert eines Schlüssels bei einer committeten Höhe ist der Wert, den seine niedrigste Version oberhalb dieser Höhe speichert, sonst sein aktueller Wert. Das Speichern einer Änderung liest nur den aktuellen Wert, nie die Historie, und ein historischer Lesezugriff ist ein Seek in die Versionen des Schlüssels. `PruneBelow` löscht jede Version bei oder unter der Höhe, ab der es behält. Baumknoten werden ebenso versioniert. Zwei Strukturen im Speicher verkleinern die Baumarbeit eines Blocks; keine wird persistiert, keine ändert ein Ergebnis: der Tree-Node-Cache mit bis zu 2^18 kodierten Knoten der aktuellen Bäume (jeder Batch des Stores aktualisiert ihn, und eine Generation, die ungerade ist, während ein solcher Batch committet, lässt einen Leser den Cache nur beim Zustand seines Snapshots nutzen), und je Namespace Wurzel und Knotenänderungen der letzten Wurzelberechnung, geschlüsselt nach dieser Generation und einem SHA-256-Digest der Schreibvorgänge des Namespace, sodass App-Hash, State-Roots und Commit dieselben Schreibvorgänge nur einmal berechnen.

## Mempool-WAL-Kompaktierung und Genesis-Batch

Ein Commit kompaktiert das Mempool-WAL nur, wenn es mindestens 4 MiB und das Doppelte dessen hält, was es nach der letzten Kompaktierung oder dem Öffnen hielt. So wird jedes angehängte Byte im Mittel etwa einmal neu geschrieben, und das Log, das ein Neustart wiederholt, bleibt innerhalb des doppelten Live-Inhalts oder 4 MiB. Eine ausdrückliche Kompaktierung (`Node.Compact` oder ein Start, der wiederholte Transaktionen verwirft, die die Anwendung ablehnt) schreibt es immer neu. `InitChain` schreibt den Genesis-Zustand aller Module als einen Batch, sodass eine Genesis mit vielen Konten einen dauerhaften Schreibvorgang kostet und eine fehlgeschlagene Genesis nichts im Store hinterlässt.

## Finality Proofs im Commit-Batch und verzögertes Entfernen aus dem Mempool

Der Commit-Pfad des Nodes schreibt die Finality Proofs, die das QC des Blocks vervollständigt, in denselben block/state/app-write-Batch (`CommitBlockStateWithWrites` erhält sie als letztes Argument). Ein Commit kostet so einen durable Write statt eines weiteren pro Proof. Ein Proof, den der Store ablehnen würde, bleibt außerhalb des Batches und wird vom Schritt nach dem Commit gespeichert, der den Fehler meldet. Ein Commit entfernt seine Transaktionen aus dem Pool und reiht den Entfernungs-Record ein, ohne auf dessen Sync zu warten. Der nächste Commit wartet, bis dieser Record durable ist, bevor er seinen Block ausführt, denn ein Neustart wiederholt die Entfernung nur für den zuletzt committeten Block. Meist teilt sich der Record den Sync einer späteren Admission.

## Verzögertes Hashen des State Tree

Eine Berechnung des State Tree hasht verzögert: Ein Write markiert die Hälften, die er geändert hat, und jeder geänderte Node wird einmal, von unten nach oben, gehasht, wenn Root, Proof oder die Änderungen verlangt werden. Die Nodes nahe der Root, die jeder Write des Blocks neu schreibt, werden so einmal pro Block statt einmal pro Write gehasht. Hashes, Nodes und Roots sind dieselben wie beim Hashen nach jedem Write.

## Periodischer Sync des Mempool-WAL

Das Mempool-WAL schreibt jede Änderung in das Log und synct es alle 100 ms (`walSyncInterval`). Eine Admission wartet auf den Write, nicht auf einen Sync. Ein Absturz des Prozesses behält jeden Record; ein Absturz der Maschine verliert höchstens die Admissions des letzten Intervalls, die ihre Clients erneut einreichen. Der Entfernungs-Record eines committeten Blocks wird vor dem nächsten Commit gesynct (`WaitDurable`).

## Wiederverwendete Reads beim History-Eintrag

Ein Commit zeichnet den vorherigen Wert eines geänderten Keys als Version auf, meist ohne ihn erneut zu lesen: Der Store merkt sich die Werte, die `Get` seit dem letzten Batch-Commit gelesen hat, bis zu 2^16, und die Ausführung eines Blocks liest die meisten Keys, die sie ändert. Ein Wert wird nur verwendet, solange seit dem Lesen kein Batch committet wurde, er ist also der aktuelle Wert; ein fehlender Key wird nicht gemerkt und gelesen.

## Versionierte State-Tree-Nodes

Tree-Nodes werden versioniert gespeichert und von keiner späteren Höhe überschrieben. Ein Batch schreibt jeden geänderten Node unter `tree: | namespace | height | node key`, und ein interner Node speichert die Versionen seiner beiden Hälften, sodass jeder Node von seinem Parent aus gefunden wird. Ein Root-Record (`troot:`) markiert jede Höhe, an der sich der Tree änderte; der Tree jeder Höhe wird ab ihrem letzten Root-Record darunter gelesen. Ein ersetzter Node bleibt für die Höhen darunter und steht in der Orphan-Liste der ersetzenden Höhe (`torph:`, eine pro Batch), die `PruneBelow` löscht. Da Versionen nur wachsen, verteilen sich die Writes eines Blocks nicht über bestehende Keys, was Pebbles Flushes und Compactions klein hält. Der Version-Index der Keys (`hidx:`) hat ebenfalls einen Eintrag pro Batch.

Zugehörige Bezeichner:

- `troot: | namespace | height`
- `torph: | height | first entry`
- `hidx: | height | first group`

## Dekodierte Nodes im Tree-Node-Cache

Der Tree-Node-Cache hält bis zu 2^18 Nodes der aktuellen Trees dekodiert, mit den Versionen ihrer Hälften, sodass ein Read weder dekodiert noch alloziert.

<!-- vexo-docs:technical-parity -->
## Anhang zur technischen Parität

Dieser Anhang stellt sicher, dass die Übersetzung die ausführbaren Schnittstellen und Kernabschnitte des englischen Referenzdokuments nicht verliert. Befehle, Konfigurationsschlüssel, RPC-Methoden und Paketnamen bleiben in allen Sprachen unverändert.

### Abschnittsabgleich
- section: Scope — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Backend — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Records — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Indexes — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: EVM Records — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Recovery Rules — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Snapshot Validation — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.
- section: Schema Migration — Dieser Abschnitt ist zusammen mit Konfigurationswerten, Prüfnachweisen, Fehlerbedingungen und Betreiberaktionen zu prüfen.

### Unverändert beibehaltene Schnittstellen
- `store.Store` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_getBalance` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `eth_getProof` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `bank/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `auth/nonce/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm/code/{0x_address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm/storage/{0x_address}/{slot}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `seen_ttl` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `code/{address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `storage/{address}/{slot}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `receipts/{tx_hash}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `logs/{address}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `evm_receipts` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `receipt_index/{tx_hash}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blob_sidecars/by_tx/{tx_hash}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `blob_sidecars/by_blob/{blob_hash}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `tree:` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `receipt_height/{height}/{tx_hash}` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
- `receipt_pruned_below` — Dieser Name wird in ausführbaren Beispielen und Konfigurationsprüfungen unverändert verwendet und darf nicht übersetzt werden.
