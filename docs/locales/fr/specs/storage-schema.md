# Schéma de stockage

> Locale: fr · Français
> Ce document est une traduction directe en français de la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


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
- Après modification, exécutez `make docs-check` pour vérifier l’arborescence locale et les garde-fous de traduction.

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
- `eth_getProof`
- `stateRoot`
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

## Backend Pebble et enregistrements binaires

Le backend par défaut est Pebble (`github.com/cockroachdb/pebble/v2`), ouvert avec un cache de blocs de 256 MiB et des memtables de 64 MiB. Les enregistrements sont stockés dans l'encodage binaire canonique du paquet `types` : uvarints minimaux, chaînes d'octets et listes préfixées par leur longueur, hachages sous forme de leurs 32 octets, valeur optionnelle derrière un octet de présence `0` ou `1`, et champs dans l'ordre de la structure qui les déclare. Un décodeur refuse tout le reste, octets en trop compris, de sorte qu'un enregistrement endommagé est signalé au lieu d'être lu comme une autre valeur ; chaque enregistrement a un seul encodage. Un répertoire de données écrit par un build antérieur (LevelDB, enregistrements JSON) n'est pas lisible et n'est pas converti : Pebble refuse de l'ouvrir avec une erreur `format major version 1`. Restaurez un tel nœud depuis un snapshot ou synchronisez-le dans un répertoire de données vide.

Identifiants associés :

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Historique d'état (undo log) et cache de l'arbre

L'historique d'état est un undo log de versions par clé. Quand une clé change à une hauteur (les écritures d'un bloc à sa hauteur, une écriture directe (`Set`, `Delete`, `SetBatch`) après le premier état validé à la hauteur validée suivante), le store enregistre la valeur que la clé avait avant, absence comprise, comme version de la clé à cette hauteur, une fois par clé et par hauteur ; les écritures directes antérieures au bloc qui valide leur hauteur gardent la version enregistrée par la première (`hdirect` désigne cette hauteur). La valeur d'une clé à une hauteur validée est celle qu'enregistre sa plus basse version au-dessus de cette hauteur, ou sa valeur actuelle s'il n'y en a pas. Enregistrer un changement ne lit que la valeur actuelle de la clé, jamais son historique, et une lecture historique est un seek dans les versions de la clé. `PruneBelow` supprime toute version à ou sous la hauteur à partir de laquelle il conserve. Les nœuds de l'arbre sont versionnés de la même façon. Deux structures en mémoire réduisent le travail d'arbre d'un bloc ; aucune n'est persistée ni ne change un résultat : le cache des nœuds de l'arbre, jusqu'à 2^18 nœuds encodés des arbres actuels (chaque batch du store le met à jour, et une génération impaire pendant la validation d'un tel batch ne laisse un lecteur utiliser le cache qu'à l'état de son snapshot), et, par namespace, la racine et les changements de nœuds du dernier calcul de racine, indexés par cette génération et un digest SHA-256 des écritures du namespace, si bien que l'app hash, les state roots et le commit ne calculent les mêmes écritures qu'une fois.

## Compactage du WAL du mempool et batch de genesis

Un commit ne compacte le WAL du mempool que lorsqu'il contient au moins 4 MiB et le double de ce qu'il contenait après le dernier compactage ou la dernière ouverture. Chaque octet ajouté n'est ainsi réécrit qu'environ une fois en moyenne, et le log qu'un redémarrage rejoue reste sous le double du contenu vivant ou 4 MiB. Un compactage explicite (`Node.Compact`, ou un démarrage qui écarte des transactions rejouées que l'application refuse) le réécrit toujours. `InitChain` écrit l'état de genesis de tous les modules en un seul batch : une genesis de nombreux comptes coûte une écriture durable, et une genesis qui échoue ne laisse rien dans le store.

## Finality proofs dans le batch du commit et retrait différé du mempool

Le chemin de commit du nœud écrit les finality proofs que complète le QC du bloc dans le même batch block/state/app-write (`CommitBlockStateWithWrites` les reçoit en dernier argument). Un commit coûte ainsi une seule écriture durable au lieu d'une de plus par proof. Une proof que le store refuserait est laissée hors du batch et enregistrée par l'étape qui suit le commit, qui signale l'échec. Un commit retire ses transactions du pool et met en file l'enregistrement du retrait sans attendre son sync. Le commit suivant attend que cet enregistrement soit durable avant d'exécuter son bloc, car un redémarrage ne refait le retrait que pour le dernier bloc commité. L'enregistrement partage en général le sync d'une admission ultérieure.

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
- `eth_getBalance` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_getProof` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `bank/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `auth/nonce/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm/code/{0x_address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm/storage/{0x_address}/{slot}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `seen_ttl` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `code/{address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `storage/{address}/{slot}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `receipts/{tx_hash}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `logs/{address}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_receipts` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `receipt_index/{tx_hash}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `blob_sidecars/by_tx/{tx_hash}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `blob_sidecars/by_blob/{blob_hash}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `tree:` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `receipt_height/{height}/{tx_hash}` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `receipt_pruned_below` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
