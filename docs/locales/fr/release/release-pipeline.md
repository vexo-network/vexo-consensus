# Release Pipeline

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.

## Vue d’ensemble

Ce document aide à comprendre le pipeline de release avec binaires signés, checksums et SBOM et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/fr/release/release-pipeline.md`

## Pourquoi lire ce document

- le pipeline de release avec binaires signés, checksums et SBOM
- Vérifiez d’abord les phrases MUST/SHOULD/MAY dans la source anglaise.
- Ce document localisé aide à la compréhension ; l’audit, le release et la sécurité se décident sur la source anglaise.

## Ce que vous devez savoir faire

- Expliquer quelle décision d’implémentation ou d’exploitation ce document soutient.
- Relier les exigences normatives de la source anglaise à la configuration réseau actuelle.
- Vérifier chain ID, validator ID, fee/gas et adresses peer avant de copier les exemples.

## Checklist d’utilisation sûre

- Vérifiez d’abord les phrases MUST/SHOULD/MAY dans la source anglaise.
- Ne traduisez pas les commandes, config key, noms RPC, champs JSON ni identifiants de code.
- Avant de copier des exemples, adaptez chain ID, validator ID, fee/gas et adresses peer à votre réseau.
- Après modification, exécutez `make docs-check` pour vérifier le locale tree et les garde-fous de traduction.

## Points d’attention

- Ce document localisé aide à la compréhension ; l’audit, le release et la sécurité se décident sur la source anglaise.
- Quand l’implémentation change, mettez à jour la source anglaise et tous les documents localisés dans le même changement.

## Interfaces à conserver telles quelles

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
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## Structure de la source anglaise

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Runbook de lancement

## Preuve de conformité EVM/Web3

`--sdk-conformance-evidence` et `--evm-web3-conformance-evidence` restent deux preuves séparées. Un simple résumé indiquant que “EVM passed” ne suffit pas ; la preuve EVM/Web3 doit inclure les sections lisibles par machine `evm_fixtures`, `evm_execution`, `web3_rpc` et `evm_corpus`, puis être liée à `evidence-manifest.json` par SHA-256 avant toute annonce publique de compatibilité.

## VRF audit evidence SHA-256

`release gate` ne fixe pas seulement les preuves d’audit BLS ; les preuves d’audit VRF doivent aussi être épinglées par SHA-256. Le fichier `--vrf-audit` doit être présent dans `evidence-manifest.json`, et `--vrf-audit-sha256` doit correspondre exactement au contenu du fichier. Avec une config, `vrf.audit_evidence_sha256` sert de digest pin par défaut. Cette règle vérifie que VRF service, KMS/HSM custody, TLS/mTLS ou pinned CA, auth token et défense contre nonce replay sont liés aux preuves de release.

## Source canonique

- [Document canonique anglais](../../en/release/release-pipeline.md)

## Termes d’attestation des preuves de release

Pour une publication publique, chaque entrée de `evidence-manifest.json` doit être vérifiée par une signature Ed25519. Conservez les indicateurs CLI et champs JSON suivants sans traduction.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
