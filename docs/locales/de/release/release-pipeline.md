# Release Pipeline

> Locale: de · Deutsch
> Dieses Dokument ist ein deutsches Begleitdokument zur englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Überblick

Dieses Dokument hilft dabei, die Release-Pipeline mit signierten Binaries, Checksums und SBOM zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/de/release/release-pipeline.md`

## Warum dieses Dokument lesen

- die Release-Pipeline mit signierten Binaries, Checksums und SBOM
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
- Nach Änderungen `make docs-check` ausführen, um locale tree und Übersetzungs-Guards zu prüfen.

## Worauf zu achten ist

- Dieses lokalisierte Dokument unterstützt das Verständnis; Audit-, Release- und Sicherheitsentscheidungen erfolgen anhand der englischen Quelle.
- Bei Implementierungsänderungen müssen englische Quelle und alle lokalisierten Dokumente im selben Change aktualisiert werden.

## Unverändert zu behaltende Schnittstellen

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## Struktur der englischen Quelle

- Release Pipeline
- Ziels
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Start-Runbook

## EVM/Web3-Konformitätsnachweis

`--sdk-conformance-evidence` und `--evm-web3-conformance-evidence` bleiben getrennte Nachweise. Eine reine Zusammenfassung wie “EVM passed” reicht nicht aus; der EVM/Web3-Nachweis muss die maschinenlesbaren Abschnitte `evm_fixtures`, `evm_execution`, `web3_rpc` und `evm_corpus` enthalten und vor öffentlichen Kompatibilitätsaussagen per SHA-256 an `evidence-manifest.json` gebunden sein.

## Richtlinie für Release Candidates

Öffentliche Release Candidates verwenden standardmäßig `make release-candidate`. Dieses Target ist das echte Gate, führt zu `release-candidate-real` und verlangt `RELEASE_CGO_ENABLED=1`, damit das Artifact den cgo-basierten BLS-Adapter `supranational/blst` wirklich enthält. `make release-candidate-plan` ist nur für PR-Smoke-Tests und Betriebsplanung gedacht; es nutzt eingebaute Fixtures und Dry-run-Pläne und darf nicht als finale Release-Evidence gelten. Ein no-cgo Artifact ist nur mit `make release-portable RELEASE_REQUIRE_BLS=0` zulässig und darf nicht als BLS-fähige Release veröffentlicht werden. Wenn `RELEASE_CGO_ENABLED=1` gesetzt ist und `RELEASE_TARGETS` fehlt, baut das Makefile nur das aktuelle Host-Target. Für mehrere OS/Architektur-Artefakte muss `RELEASE_TARGETS` explizit auf Runnern mit passenden cgo-Cross-Compilern gesetzt werden.

## VRF audit evidence SHA-256

`release gate` pinnt nicht nur BLS audit evidence; auch VRF audit evidence muss per SHA-256 gebunden werden. Die Datei `--vrf-audit` muss in `evidence-manifest.json` stehen, und `--vrf-audit-sha256` muss exakt zum Dateiinhalt passen. Bei config-Nutzung dient `vrf.audit_evidence_sha256` als Standard-Digest-Pin. Diese Regel prüft, dass VRF service, KMS/HSM custody, TLS/mTLS oder pinned CA, auth token und nonce replay defense an Release-Evidence gebunden sind.

## Kanonische Quelle

- [Englisches kanonisches Dokument](../../en/release/release-pipeline.md)

## Begriffe für Release-Evidence-Attestation

Für öffentliche Releases muss jeder Eintrag in `evidence-manifest.json` mit einer Ed25519-Signatur verifiziert werden. Die folgenden CLI-Flags und JSON-Felder bleiben unverändert und werden nicht übersetzt.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
<!-- vexo-docs-ops-update-2026-06 -->

## Netzwerk-E2E verstehen

`make network-e2e` ist mehr als ein Build-Test: Es startet 4 validators mit dem echten Binary und prüft signed-shape smoke transaction, peer Verbindung, height Fortschritt und clean stop. `NETWORK_E2E_GO_TIMEOUT` ist die äußere Go-test Grenze und muss größer sein als der interne network timeout.
