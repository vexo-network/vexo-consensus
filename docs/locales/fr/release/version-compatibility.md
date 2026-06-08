# Version Compatibility Matrix

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre la matrice de compatibilité de versions et les critères d’upgrade. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/release/version-compatibility.md`
- Locale path: `docs/locales/fr/release/version-compatibility.md`

## Identifiants à conserver

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `/v1/*`
- `vexod upgrade plan --json`
- `vexod upgrade apply`
- `rollback_required`
- `make release-candidate`

## Sections anglaises

- Version Compatibility Matrix
- Current Matrix
- Upgrade Compatibility Checklist
- Rollback Drill

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/release/version-compatibility.md)
