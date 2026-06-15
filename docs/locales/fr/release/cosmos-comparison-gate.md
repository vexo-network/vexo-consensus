# Cosmos/Tendermint Comparison Gate

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


## Ordre de lecture

Ce document explique le processus de release et d'exploitation de Cosmos Comparison Gate. Si c'est votre première lecture, suivez cet ordre.

1. Required Evidence Properties
2. Release Rule

Cet ordre correspond à l'usage réel : d'abord les objectifs et les gates, puis les artefacts et les exigences de preuve, et enfin les étapes d'exécution.

## Vue d’ensemble

Ce document aide à comprendre la porte de release face aux attentes de style Cosmos/Tendermint et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/fr/release/cosmos-comparison-gate.md`

## Pourquoi lire ce document

- la porte de release face aux attentes de style Cosmos/Tendermint
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
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## Structure de la source anglaise

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## Source canonique

- [Document canonique anglais](../../en/release/cosmos-comparison-gate.md)

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Required Evidence Properties — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Release Rule — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `--longrun-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--chaos-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--ops-runbook-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--external-audit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--formal-safety-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--fuzz-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--sdk-conformance-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-web3-conformance-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--p2p-scale-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--state-sync-light-client-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--snapshot-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--validator-economics-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--upgrade-governance-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--mev-fee-market-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--kms-evidence` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--bls-audit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
