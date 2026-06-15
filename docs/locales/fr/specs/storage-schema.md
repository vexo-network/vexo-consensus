# Storage Schema

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


## Ordre de lecture

Ce document explique la spécification normative de Storage Schema. Si c'est votre première lecture, suivez cet ordre.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Cet ordre correspond à la manière de lire le document : d'abord le périmètre et l'état, puis les règles des messages, de sûreté et de vivacité, et enfin les preuves.

## Vue d’ensemble

Ce document aide à comprendre les namespaces de durable storage, key schema et recovery marker et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/fr/specs/storage-schema.md`

## Pourquoi lire ce document

- les namespaces de durable storage, key schema et recovery marker
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
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Structure de la source anglaise

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

## Source canonique

- [Document canonique anglais](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Scope — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Backend — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Records — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Indexes — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: EVM Records — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Recovery Rules — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Snapshot Validation — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Schema Migration — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `store.Store` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_ethstate` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_getBalance` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_getProof` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `bank/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `auth/nonce/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm/code/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm/storage/{0x_address}/{slot}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_ethstate/{height}/meta` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_ethstate/{height}/accounts/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_ethstate/{height}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `seen_ttl` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `code/{address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `storage/{address}/{slot}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `receipts/{tx_hash}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `logs/{address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
