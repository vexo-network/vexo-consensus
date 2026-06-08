# RPC API Versioning

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre le versionnement RPC API, les alias de compatibilité et la politique de stabilité. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/fr/sdk/rpc-api-versioning.md`

## Identifiants à conserver

- `/v1`
- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`

## Sections anglaises

- RPC API Versioning
- Stability Goal
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/sdk/rpc-api-versioning.md)
