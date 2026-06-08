# Documentation Vexo

Ce répertoire est le point d'entrée français de la documentation Vexo. La documentation anglaise (`en`) reste la source normative; cette arborescence conserve les mêmes chemins pour faciliter la navigation.

## À lire en premier

1. [Vue d'ensemble du consensus](./consensus-protocol.md)
2. [Spécification du consensus](./specs/consensus-spec.md)
3. [Format des transactions](./specs/tx-format.md)
4. [Cycle de vie des validateurs](./specs/validator-lifecycle.md)
5. [Préparation à l'audit de sécurité](./security/audit-readiness.md)

## Ensembles de documents

| Section | Chemin | Rôle |
|---|---|---|
| Opérateurs | `operators/` | Initialisation des nœuds, ajout de validateurs, configuration |
| Release | `release/` | Pipeline de publication, runbook, compatibilité, gates |
| SDK | `sdk/` | Modules applicatifs, crypto/storage/transport personnalisés, versioning RPC |
| Sécurité | `security/` | Modèle de menace, hypothèses, préparation d'audit |
| Spécifications | `specs/` | Consensus, réseau, stockage, transactions, finality proof |

Les commandes, clés JSON, méthodes RPC et identifiants de code restent en anglais.
