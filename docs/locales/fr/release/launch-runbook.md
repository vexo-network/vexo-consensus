# Runbook de lancement

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.

## Vue d’ensemble

Ce document aide à comprendre la checklist opérateur et la procédure d’exécution avant lancement réseau et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/fr/release/launch-runbook.md`

## Pourquoi lire ce document

- la checklist opérateur et la procédure d’exécution avant lancement réseau
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

- `MaxScore`
- `release gate`
- `checksums.txt`
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
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## Structure de la source anglaise

- Runbook de lancement
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Preuve de conformité EVM/Web3

Avant une publication publique, archivez `--evm-web3-conformance-evidence` séparément de `--sdk-conformance-evidence`. Le fichier doit contenir `evm_fixtures`, `evm_execution`, `web3_rpc` et `evm_corpus` afin que `release gate` puisse rejeter les résumés non vérifiables.

## VRF audit evidence SHA-256

Pour valider un release candidate, passez à `release gate` les digest d’audit BLS et VRF. Utilisez au minimum `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` et `--evidence-manifest`, puis vérifiez que chaque fichier evidence correspond au SHA-256 du manifest.

## Source canonique

- [Document canonique anglais](../../en/release/launch-runbook.md)
