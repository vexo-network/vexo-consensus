# Storage Schema

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre les namespaces de durable storage, key schema et recovery marker. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/fr/specs/storage-schema.md`

## Identifiants à conserver

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

## Sections anglaises

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

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/specs/storage-schema.md)
