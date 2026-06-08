# Consensus Protocol Overview

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.

## Vue d’ensemble

Ce document aide à comprendre le modèle de consensus, les termes execution/commit/finality et la limite de sécurité et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/consensus-protocol.md`
- Locale path: `docs/locales/fr/consensus-protocol.md`

## Pourquoi lire ce document

- le modèle de consensus, les termes execution/commit/finality et la limite de sécurité
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

- `FinalizeBlock`
- `consensus_config.json`
- `execution_commit`
- `finalized`
- `qc`
- `require_network_safety`
- `block_committed`
- `deterministic`
- `ed25519`
- `bls`

## Structure de la source anglaise

- Consensus Protocol Overview
- Model
- Execution Terms
- Safety Boundary
- Crypto Boundary
- Operational Boundary

## Source canonique

- [English canonical document](../en/consensus-protocol.md)
