# Adaptives, wiederherstellungsgeschütztes HotStuff für modulare Proof-of-Stake-Netze

> Locale: de · Deutsch  
> Dokumenttyp: Forschungsmanuskript und Reproduktionsprotokoll  
> Status: implementierungsnaher Entwurf; Leistungsbehauptungen benötigen gemessene Artefakte.

## Zusammenfassung

Dieses Manuskript untersucht eine HotStuff-artige byzantinisch fehlertolerante Zustandsmaschinenreplikation für modulare Proof-of-Stake-Netze. Die vorliegende Implementierung verbindet eine Three-Chain-Finalisierung und höhenversionierte Validatorensätze mit drei betrieblichen Mechanismen. Ein begrenzter adaptiver Round-Timeout-Controller verwendet die p95-Verarbeitungszeiten von Vorschlägen, Stimmen und Commits sowie den Zustand aktiver Peers. Eine Recovery-Finality-Schranke hält finalisierte Anwendungs-Commits zurück, wenn die dauerhaft gespeicherte Blockhistorie und der Anwendungszustand oberhalb einer gemeinsamen sicheren Höhe voneinander abweichen. Eine deterministische Transaktionsordnung entfernt den lokalen Mempool-Eingangszeitpunkt aus der Reihenfolge eines identischen Kandidatensatzes, bewahrt aber die Nonce-Abhängigkeiten jedes Signierers.

Die Arbeit behauptet nicht, Proof of Stake, BFT, HotStuff, adaptive View-Synchronisation oder faire Transaktionsordnung erfunden zu haben. Untersucht wird die engere Frage, ob genau diese begrenzte Steuerungs- und Recovery-Komposition vermeidbare Runden-Timeouts und Wiederherstellungsinkonsistenzen reduziert, ohne die zugrunde liegende HotStuff-Sicherheitsregel zu verändern. Implementierte Tatsachen, Hypothesen und noch zu messende Aussagen werden getrennt. Zahlen zu Durchsatz oder Latenz dürfen erst nach wiederholten Experimenten mit festgehaltenem Binary, Konfiguration, Topologie und Datensatz als Ergebnis erscheinen.

## Forschungsfragen und Hypothesen

RQ1 fragt, ob die adaptive Steuerung bei wechselnder Netzverzögerung weniger Round-Timeouts und eine niedrigere p95-Commit-Latenz als dieselbe Implementierung mit festem Timeout erzeugt. RQ2 prüft bei injizierten Speicher- und Neustartfehlern, ob die Recovery-Schranke verhindert, dass der Anwendungszustand die gemeinsam konsistente Block-/Zustandshöhe überschreitet. RQ3 prüft alle Eingabepermutationen derselben Transaktionsmenge auf identische Vorschlagsordnung und monotone Nonces pro Signierer. RQ4 misst CPU-, Speicher-, Netzwerk- und Latenzkosten unter stabilen, fehlerfreien Bedingungen.

H1 bis H4 sind gerichtete, widerlegbare Hypothesen. Das Vorhandensein eines Codepfads beweist keine Verbesserung. Falls ein Vorteil statistisch oder betrieblich nicht relevant ist, wird dies als negatives Ergebnis oder als Grenze des Verfahrens berichtet und nicht durch eine stärkere Formulierung verdeckt.

## Abgrenzung zum Stand der Forschung

HotStuff beschreibt leaderbasierte BFT-Replikation unter partieller Synchronität, Quorum-Zertifikate, verkettete Commit-Regeln, lineare Kommunikation im günstigen Fall und Reaktionsfähigkeit. LibraBFT beziehungsweise DiemBFT und AptosBFT zeigen bereits die Verbindung HotStuff-abgeleiteter BFT mit stakegewichteter Validatorverwaltung. Jolteon und Ditto behandeln geringere Latenz, Netzanpassung und asynchronen Fallback; Fever untersucht responsive View-Synchronisation. Tendermint bildet eine andere rundenbasierte PoS-BFT-Linie. Narwhal/Tusk trennt zuverlässige Transaktionsverteilung von der Ordnung. Aequitas, Wendy und Themis definieren stärkere Ordnungsgerechtigkeit als die hier eingesetzte hashbasierte Deterministik.

Deshalb sind Aussagen wie „erste PoS+BFT-Blockchain“, „erstes HotStuff-PoS-Netz“, „identisch mit AptosBFT“, „asynchrone Lebendigkeit ohne Beweis“, „optimale Kommunikationskomplexität“, „vollständiger MEV-Schutz“ oder „produktionsreif nach einem Single-Host-Test“ unzulässig. Der mögliche Systembeitrag ist enger: ein begrenzter Feedback-Controller, eine lokale Durable-History-Commit-Schranke und noncebewusste deterministische Ordnung werden in einen modularen Go-PoS-Knoten integriert und gegen feste sowie Gate-deaktivierte Varianten reproduzierbar ausgewertet.

## Systemmodell und Protokoll

Für Höhe h sei Vh der aktive Validatorensatz und Ph seine gesamte Voting Power. Ein QC ist gültig, wenn eindeutige bekannte Signierer mindestens zwei Drittel von Ph beitragen. Validatorensatz und Hash sind höhenversioniert. Die Aufnahme kann bei Mindeststake erlaubnisfrei, zahlenmäßig begrenzt oder durch Konfiguration eingeschränkt sein. Diese Schicht dient Sybil-Abwehr und Governance; sie ändert den BFT-Fehlerschwellenwert nicht.

Das Netz wird als partiell synchron angenommen. Sicherheit wird bei weniger als einem Drittel byzantinischer Voting Power sowie gültigen Signaturen, korrekter Validatorensatzbindung und zuverlässiger Persistenz erwartet. Lebendigkeit setzt zusätzlich voraus, dass die Verzögerung schließlich begrenzt ist, ein ehrliches Quorum erreichbar bleibt, Signierer verfügbar sind und genügend Peers verbunden sind. Für ein dauerhaft asynchrones Netz wird kein Fortschritt garantiert.

EVM-Ausführung ist eine Anwendungsarbeitslast unter Vexo-Consensus. Ethereum-Bytecode und `/web3`-Werkzeugkompatibilität bedeuten nicht, dass Ethereum-Fork-Choice oder Ethereum-devp2p-Consensus implementiert werden.

Die Basissicherheitsregel verfolgt `locked_qc` und `high_qc`. Ein Vorschlag ist nur sicher, wenn er die Sperre erweitert oder ein mindestens gleich neues Justify-QC trägt. Ein Validator darf in derselben Höhe und Runde nicht für verschiedene Blöcke stimmen. Drei aufeinanderfolgende, an Höhe und Hash gebundene zertifizierte Links finalisieren den Großelternblock. Der adaptive Controller verändert weder dieses Prädikat noch Quorumschwelle, QC-Prüfung oder Three-Chain-Regel.

Der adaptive Timeout verwendet ein Basisbudget T0, das aktuelle Budget Tt, die Summe der p95-Verarbeitungszeiten und ein Peer-Defizit. Nach einem Timeout wächst der aktuelle Wert in Richtung 1,5×Tt; nach Fortschritt fällt er in Richtung 0,8×Tt. Das Dreifache der beobachteten Latenzsumme bildet eine Untergrenze. Der endgültige Wert wird auf T0 bis 8×T0 begrenzt. Ohne aktive Peers beträgt die Peer-Untergrenze 2×T0. Leerlauf ohne anstehende Arbeit sowie lokale Ausführungs- oder Speicherfehler verbrauchen keine Runde. Diese Regel ist ein begrenzter Betriebscontroller, kein Beweis eines theoretisch optimalen Pacemakers.

Die Recovery-Schranke berechnet bei vorhandener dauerhafter Zustandshöhe Hs und Blockindexhöhe Hb den Wert Hsafe=min(Hs,Hb). Solange beide Historien abweichen, werden finalisierte Anwendungs-Commits oberhalb Hsafe zurückgestellt. Dies ist eine lokale Persistenzbeschränkung, keine zusätzliche Netzwerkabstimmung und kein neues Zertifikat.

Die deterministische Ordnung erzeugt aus Chain-ID und Höhe einen Salt. Transaktionen mit Signierer-/Nonce-Metadaten werden in Signiererketten gruppiert und innerhalb jeder Kette nach aufsteigender Nonce sortiert. Die Köpfe der Ketten werden anhand eines gesalzenen Transaktionshashes deterministisch zusammengeführt. Dadurch wird die Ankunftsreihenfolge für einen identischen Kandidatensatz entfernt. First-seen Fairness, Zensurresistenz, Vertraulichkeit oder starke Order-Fairness folgen daraus nicht, denn ein Proposer kann die Aufnahme in den Kandidatensatz beeinflussen.

Der aktive Consensus-Stimmpfad verwendet derzeit den vollständigen höhenversionierten Validatorensatz und deterministische Proposer-Auswahl. Ein ECVRF-Committee-Selector existiert als Komponente und Abfrageoberfläche, ist aber nicht mit Quorumsbildung oder Vorschlagsberechtigung verbunden. VRF-Committee-Consensus ist daher zukünftige Arbeit.

## Versuchsaufbau und Auswertung

Alle Treatments verwenden dasselbe Binary und dieselbe Anwendungskonfiguration. Verglichen werden eine feste Variante mit adaptiver Steuerung aus und Recovery-Schranke an, die adaptive Variante mit beiden Funktionen an und eine ausschließlich im isolierten Forschungsnetz zulässige Ablation mit deaktivierter Recovery-Schranke. Wenn Ressourcen vorhanden sind, werden 4, 7, 16 und 31 Validatoren untersucht; Single-Host-Läufe dienen nur als Smoke-Test.

Netzbedingungen umfassen 10, 50, 100 und 250 ms Latenz, schrittweise Verzögerungsänderungen, Jitter, 0/1/5/10 Prozent Verlust, Neustart eines normalen Validators, Neustart des aktuellen Proposers, Ausfall von knapp weniger als einem Drittel Voting Power, kurze Minderheitspartition mit Heilung, Signiererverzögerung und eine injizierte Inkonsistenz dauerhafter Block-/Zustandshistorien. Arbeitslasten enthalten native Transfers, EVM-Transfers, Contract Creation, Event-Logs, Proxy-Deployment und UUPS-Upgrade.

Gemessen werden committed und finalized height, p50/p95/p99 für Proposal, Vote und Commit, Ende-zu-Ende-Finalitätslatenz, Timeoutanzahl, Rundenverteilung, aktueller adaptiver Timeout, Peerzahl, Recovery-Deferrals, Durchsatz, Gas, CPU, RSS, Disk- und Netzwerkbytes sowie Ablehnungen, Double-Sign- und Invalid-Nonce-Ereignisse. Ein Lauf gilt nur, wenn alle Validatoren auf der verglichenen Höhe denselben App-Hash und finalisierten Block-Hash besitzen, Receipts und Blockpositionen übereinstimmen, Contract-Code vorhanden ist und Proxy-State das UUPS-Upgrade korrekt übersteht.

Nach einer Aufwärmphase werden pro Bedingung grundsätzlich mindestens 30 unabhängige Wiederholungen ausgeführt oder eine kleinere Zahl durch Power-Analyse begründet. Treatment-Reihenfolge und Seeds werden aufgezeichnet. Berichtete Statistiken umfassen Median, Interquartilsabstand, p95, Konfidenzintervalle und Effektgröße. Es dürfen nicht nur die besten Läufe ausgewählt werden; Ausschlussregeln werden vor Einsicht in Ergebnisse festgelegt.

## Korrektheit, Reproduzierbarkeit und Forschungsethik

Die adaptive Regel verändert nur den Zeitpunkt eines Timeout-Versuchs, nicht die Definition einer sicheren Stimme oder eines gültigen QC. Die Recovery-Schranke kann Commits nur weiter einschränken und keinen vom Basissystem abgelehnten Commit erlauben. Deterministische Ordnung unterstützt identische Ausführungseingaben, ersetzt jedoch keinen Sicherheitsbeweis gegen widersprüchliche Finalisierung.

Eine veröffentlichungsfähige formale Argumentation muss stakegewichtete Quorumschnittmenge, Lock-Monotonie, Eindeutigkeit finalisierter Blöcke je Höhe, Übergänge zwischen Validatorensätzen, Crash-Recovery des Vote-WAL und die sicherheitsneutrale Wirkung von Controller und Gate abdecken. Unit Tests und adversariale Simulationen sind Evidenz, aber kein Ersatz für formalen Beweis oder unabhängiges Audit.

Für jedes Experiment werden Commit, Dirty-Tree-Status, Go-/OS-/CPU-/Speicher-/Containerdaten, Topologie, Genesis, Split-Konfigurationen, Binary-SHA-256, Workload-Seed, rohe JSON/JSONL/CSV-Daten, Validator-Logs, finale App-Hashes, Analyseskripte und ein Verzeichnis fehlgeschlagener Läufe archiviert. Bekannte Mechanismen dürfen nicht bloß umbenannt und als Erfindung dargestellt werden. Durchsatz, Latenz und Validatorenzahl werden nicht erfunden; Hypothese, Beobachtung und Interpretation bleiben getrennt.

KI-Unterstützung wird nach den Regeln des Zielvenues offengelegt; die Autorinnen und Autoren bleiben für jede Behauptung, Zitation, Messung und jeden Beweis verantwortlich. Fehlerinjektion findet nur auf eigenen oder ausdrücklich autorisierten isolierten Systemen statt. Private Schlüssel, Operator-Tokens, Teilnehmerdaten und Produktionsendpunkte gehören nicht in Artefakte. Sicherheitsfunde werden koordiniert offengelegt.

Vor Einreichung müssen Manuskript und gepinnte Source-Revision übereinstimmen, die Prior-Art-Suche dokumentiert, Baselines reproduzierbar, Multi-Host-Fehlermessungen abgeschlossen und alle Tabellen und Abbildungen aus Rohdaten und Skripten wiederherstellbar sein. Negative Resultate, Grenzen, angemessene Beweisformulierungen und externe methodische Prüfung bleiben Bestandteil der Einreichung. Bis dahin lautet die korrekte Bezeichnung „implementierungsnaher Forschungsentwurf“, nicht „neuer bewiesener Consensus“.

<!-- vexo-docs:technical-parity -->

## Anhang zur technischen Parität

Die folgenden Implementierungs-, Konfigurations- und Prüfnamen bleiben unverändert:

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
