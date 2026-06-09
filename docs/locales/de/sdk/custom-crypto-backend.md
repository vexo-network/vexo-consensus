# Custom Crypto Backend Guide

> Locale: de · Deutsch
> Dieses Dokument ist ein deutsches Begleitdokument zur englischen Quelle. Protokoll-, Sicherheits- und Release-Entscheidungen bleiben im Englischen normativ.

## Überblick

Dieses Dokument hilft dabei, die Integration von custom crypto backends wie BLS, VRF und Signer zu verstehen und mit Implementierungs- sowie Betriebsentscheidungen zu verbinden.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/de/sdk/custom-crypto-backend.md`

## Warum dieses Dokument lesen

- die Integration von custom crypto backends wie BLS, VRF und Signer
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

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

## Struktur der englischen Quelle

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## Kanonische Quelle

- [English canonical document](../../en/sdk/custom-crypto-backend.md)
