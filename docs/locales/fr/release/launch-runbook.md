# Runbook de lancement

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


## Ordre de lecture

Ce document explique le processus de release et d'exploitation de Launch Runbook. Si c'est votre première lecture, suivez cet ordre.

1. At a Glance
2. Prelaunch Gate
3. Release Candidate Gate
4. Genesis Gate
5. Launch Window
6. Postlaunch Archive

Cet ordre correspond à l'usage réel : d'abord les objectifs et les gates, puis les artefacts et les exigences de preuve, et enfin les étapes d'exécution.

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

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Prelaunch Gate — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Release Candidate Gate — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Genesis Gate — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Launch Window — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Postlaunch Archive — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `release docs-quality` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `checksums.txt` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `sbom-go-modules.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `sbom-go-version.txt` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `release-manifest.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `release-audit-pack.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `release collect-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network analyze-longrun` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `longrun-evidence.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-default-fixtures` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-tx-fixtures` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-tx-fixtures-dir` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-execution-fixtures` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-execution-fixtures-dir` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-tx-fixtures-sha256` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-execution-fixtures-sha256` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-web3-conformance-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_fixtures` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_execution` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `web3_rpc` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_corpus` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod ops conformance` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `relayer soak-plan` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `chain_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evidence-manifest.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
