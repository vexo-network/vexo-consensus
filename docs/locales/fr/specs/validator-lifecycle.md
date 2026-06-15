# Validator Lifecycle

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


## Ordre de lecture

Ce document explique la spécification normative de Validator Lifecycle. Si c'est votre première lecture, suivez cet ordre.

1. Scope
2. Admission
3. Validator Set
4. Rotation
5. Evidence Lifecycle
6. Slashing
7. Jail and Unbonding

Cet ordre correspond à la manière de lire le document : d'abord le périmètre et l'état, puis les règles des messages, de sûreté et de vivacité, et enfin les preuves.

## Vue d’ensemble

Ce document aide à comprendre le cycle validator join, rotation, jail, slashing et leave et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/fr/specs/validator-lifecycle.md`

## Pourquoi lire ce document

- le cycle validator join, rotation, jail, slashing et leave
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Structure de la source anglaise

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Source canonique

- [Document canonique anglais](../../en/specs/validator-lifecycle.md)

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Scope — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Admission — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Validator Set — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Rotation — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Evidence Lifecycle — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Slashing — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Jail and Unbonding — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `vexovaloper...` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexovalcons...` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo...` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `staking tx withdraw-unbonded` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
