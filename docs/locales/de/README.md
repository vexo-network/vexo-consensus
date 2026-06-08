# Vexo-Dokumentation

Dieses Verzeichnis ist der deutsche Einstiegspunkt für die Vexo-Dokumentation. Die englische Dokumentation (`en`) ist die normative Quelle; die deutsche Struktur spiegelt dieselben Pfade wider.

## Einstieg

1. [Konsensprotokoll Überblick](./consensus-protocol.md)
2. [Konsensspezifikation](./specs/consensus-spec.md)
3. [Transaktionsformat](./specs/tx-format.md)
4. [Validator-Lebenszyklus](./specs/validator-lifecycle.md)
5. [Vorbereitung auf Sicherheitsaudits](./security/audit-readiness.md)

## Dokumentgruppen

| Bereich | Pfad | Zweck |
|---|---|---|
| Betrieb | `operators/` | Node-Initialisierung, Validator hinzufügen, Konfiguration |
| Release | `release/` | Release-Pipeline, Runbook, Kompatibilität, Gates |
| SDK | `sdk/` | App-Module, eigene crypto/storage/transport Backends, RPC-Versionierung |
| Sicherheit | `security/` | Threat Model, Annahmen, Audit-Vorbereitung |
| Spezifikationen | `specs/` | Konsens, Netzwerk, Storage, Transaktionen, Finality Proof |

Befehle, JSON-Felder, RPC-Methoden und Code-Bezeichner bleiben unverändert auf Englisch.
