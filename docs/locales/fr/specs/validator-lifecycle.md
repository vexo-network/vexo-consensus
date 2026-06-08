# Validator Lifecycle

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre le cycle validator join, rotation, jail, slashing et leave. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/fr/specs/validator-lifecycle.md`

## Identifiants à conserver

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Sections anglaises

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/specs/validator-lifecycle.md)
