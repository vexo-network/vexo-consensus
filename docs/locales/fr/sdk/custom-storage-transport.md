# Custom Storage and Transport Guide

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre l’implémentation et l’enregistrement de custom storage et transport adapter. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/sdk/custom-storage-transport.md`
- Locale path: `docs/locales/fr/sdk/custom-storage-transport.md`

## Identifiants à conserver

- `store.Store`
- `store.HistoricalSnapshotKVStore`
- `store.SnapshotKVStore`
- `transport.Transport`

## Sections anglaises

- Custom Storage and Transport Guide
- Custom Storage
- Storage Requirements
- Custom Transport
- Transport Requirements
- Compatibility

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/sdk/custom-storage-transport.md)
