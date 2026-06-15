# Node Initialization

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


## Ordre de lecture

Ce document s'adresse à la fois aux personnes qui créent un node home pour la première fois et à celles qui exploitent déjà un nœud. Si c'est votre première lecture, suivez cet ordre.

1. Ce que vous construisez
2. Exécution locale en cinq minutes
3. Réseau local à quatre validateurs
4. Web3 et Remix
5. Validator Node
6. Archive Node
7. Split Configuration Files
8. Which File Do I Edit?
9. Key Types
10. Config-Based Peers
11. Consensus Timing
12. Multi-Validator Network
13. Troubleshooting
14. Minimal Operator Checklist

Cet ordre correspond aux vérifications que l'opérateur doit faire en premier : comprendre ce qu'est un node home, valider le lancement local, distinguer validator et archive, puis vérifier les peers, le timing et la gestion des incidents.

## Vue d’ensemble

Ce document aide à comprendre l’initialisation des nœuds archive/validator et l’exploitation des fichiers de configuration séparés et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/fr/operators/node-initialization.md`

## Pourquoi lire ce document

- l’initialisation des nœuds archive/validator et l’exploitation des fichiers de configuration séparés
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

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key-env`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`
- `config.json`
- `module_config.json`
- `consensus_config.json`
- `mempool_config.json`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## Structure de la source anglaise

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## Source canonique

- [Document canonique anglais](../../en/operators/node-initialization.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Note opérationnelle récente

Pour un nouveau répertoire de nœud, vérifiez ensemble `p2p.dial_timeout`, `p2p.auth_replay_path` et `p2p.require_auth_replay_store` dans `network_config.json`. Le timeout par défaut `10s` couvre le dial TCP, TLS, le signed handshake et la vérification replay-store. Sur un réseau public, ces paramètres doivent rester dans la configuration revue, pas dans des flags shell cachés.

## Synchronisation d’état au démarrage au démarrage

Le bloc `state_sync` de `network_config.json` sert aux nouveaux nœuds archive, aux validator de remplacement et aux nœuds restaurés sur une machine propre. Quand `state_sync.enabled` vaut true, `vexod start` essaie les URL de `state_sync.snapshot_urls`, vérifie chain ID, checksum, state root et KV namespace, restaure dans LevelDB, reconstruit les index, puis démarre le nœud. Si l'état local atteint déjà `state_sync.min_height` et que `state_sync.trust_local_higher` vaut true, le nœud garde le store local et journalise `state_sync_skipped`.

```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```

Surveillez `state_sync_candidate_failed`, `state_sync_candidate_rejected` et `state_sync_applied`. Pour un réseau public, n'utilisez pas une source snapshot tierce sans politique de confiance et sans finality/light-client evidence.

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Validator Node — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Archive Node — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Split Configuration Files — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Key Types — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Config-Based Peers — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Consensus Timing — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Multi-Validator Network — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `network_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod start` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--timeout-propose` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--create-empty-blocks` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--p2p-auth-token` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--rpc-admin-token` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-account-key-env` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-account-key` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `VEXO_KEY_PASSPHRASE` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--passphrase` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--encrypt-keys` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `node.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator.vrf.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `require_network_safety=true` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--key-type bls` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `blst-bls12381-minpk-v1` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `genesis.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `bls_pop` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `module_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `consensus_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `mempool_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `log_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `data/` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network_config.json:p2p.node_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `shutdown_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `web3_max_subscriptions_per_connection` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `web3_idle_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `auth_replay_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `require_auth_replay_store` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `dial_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `data/p2p_auth_replay.jsonl` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--key-type ed25519` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vrf_key_paths` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vrf_public_key` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `<home>/<name>_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.evm_account_key_envs` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.evm_account_private_keys` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_accounts` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_sign` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_signTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_sendTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_account_key_envs` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod config paths --home <home>` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `"require_network_safety": true` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution_commit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `require_network_safety` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `host:port` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.listen_address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.peers` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.seeds` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.node_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.node_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_cert_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_ca_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_server_name` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.dial_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_propose` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_prevote` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_precommit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_commit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `create_empty_blocks: false` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution_commit: "finalized"` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution_commit: "qc"` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `round_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `create_empty_blocks` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod network up` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `make network-e2e` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator-%d` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_advertise_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_advertise_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_listen_host` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_listen_host` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
