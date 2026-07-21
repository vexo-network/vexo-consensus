# Guide de mise à jour EVM

> Locale: fr · Français
> Ce document est la traduction française de la source anglaise. Les décisions de protocole, de sécurité et de publication suivent la source anglaise.

Ce guide explique comment mettre à jour la pile EVM intégrée sans casser la gestion du chain ID, la compatibilité Web3 ou les preuves de publication. Il s’adresse aux opérateurs et mainteneurs qui doivent faire évoluer go-ethereum, ajuster les fork presets ou modifier le comportement EVM dans une release contrôlée.

## Ce qui compte comme une mise à jour EVM

Traitez comme une évolution sensible pour la release toute modification qui peut affecter l’exécution de style Ethereum ou le comportement visible côté Web3 :

- montée de version de `go-ethereum` dans `modules/evm/backend/geth`
- changements dans `modules/evm/ethcompat`
- changements dans `modules/evm`
- changements de `execution.evm_fork_preset`
- changements de `execution.evm_chain_config_json`
- changements de l’admission des raw transactions, du gas accounting, des receipts, des traces, des proofs ou des champs de réponse bloc
- changements du traitement des comptes Web3 gérés comme `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction` ou `eth_sendTransaction`

## Ordre de mise à jour sûr

Suivez cet ordre pour garder le code, la configuration et la documentation alignés :

1. Mettez d’abord à jour l’adapter geth isolé.
2. Mettez ensuite à jour le corpus de fixtures et les tests de conformance.
3. Si la sémantique change, mettez à jour `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md` et `docs/sdk/rpc-api-versioning.md`.
4. Si la forme des preuves de release change, mettez à jour `docs/release/release-pipeline.md`.
5. Si les réglages visibles par l’opérateur changent, mettez à jour les guides de configuration du nœud.
6. Relancez la matrice de validation avant de merger.

Ne montez pas une version runtime EVM et ne l’expédiez pas en même temps, sauf si les suites de conformance, les smoke checks RPC et les vérifications Docker ont toutes réussi.

## Flux de mise à jour

### 1. Figer le périmètre

Consignez précisément l’intention de la mise à jour :

- fork behavior uniquement
- transaction admission uniquement
- execution semantics uniquement
- RPC compatibility uniquement
- traitement blob / receipt / trace uniquement
- comportement des comptes gérés ou des wallets uniquement

Cette séparation garde la revue focalisée et évite de faire bouger du code sans rapport.

### 2. Modifier la couche la plus étroite

Privilégiez ces frontières :

- `modules/evm/backend/geth` pour les changements d’intégration avec go-ethereum
- `modules/evm/ethcompat` pour le décodage des raw transactions, la préservation des hash et les fixtures
- `modules/evm` pour le state transition, les receipts, les logs, le stockage et les snapshots
- `rpc` pour les changements de surface Web3 request/response
- `cmd/vexod` seulement si le CLI ou le flux de release doit exposer le nouveau comportement

Si le changement atteint les application modules, gardez la frontière de module explicite et conservez des écritures d’état déterministes.

### 3. Actualiser les valeurs par défaut de configuration

Lorsque la sémantique change, mettez à jour la config par défaut dans le même patch :

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- si besoin, les champs RPC de comptes gérés dans `network_config.json`
- le `module_config.json` pour l’EVM chain ID

N’essayez jamais d’expliquer le comportement runtime avec un flag CLI caché. La configuration doit rendre le comportement du nœud lisible à partir des fichiers seuls.

### 4. Exécuter la pile de conformance

Lancez au minimum :

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Vérifiez ensuite les parcours visibles par l’utilisateur qui cassent souvent en premier :

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Pour un déploiement Docker single-host, vérifiez aussi :

```text
http://127.0.0.1:28657/web3
```

Contrôlez au minimum les comportements suivants :

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

Déployez ensuite un contrat simple, un proxy contract, puis testez le chemin UUPS upgrade avec le même endpoint RPC que celui utilisé en production.

### 5. Valider le proxy et l’upgrade

La mise à jour EVM n’est terminée que si tout cela est vrai :

- un déploiement de contrat simple réussit
- un déploiement de proxy réussit
- un appel UUPS upgrade réussit
- après upgrade, les lectures de storage et de code renvoient le résultat attendu
- le suivi des nonce reste monotone
- le block producer accepte les transactions sans erreur unsafe proposal

Si le déploiement du proxy passe mais que l’upgrade échoue, ce n’est pas encore publiable. Il faut traiter cela comme un release blocker, pas comme un simple avertissement.

### 6. Rafraîchir les preuves

Quand la surface EVM change, mettez aussi à jour le bundle de release evidence :

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- toute référence SHA-256 épinglée

Les preuves de release doivent dire ce qui a changé, ce qui a été testé et quel commit ou quelle version a été vérifié. Ne dites jamais qu’une mise à jour EVM est terminée si la preuve ne correspond pas au code réellement exécuté.

## Matrice de validation

Utilisez ce tableau comme merge gate.

| Check | Pourquoi c’est important |
| --- | --- |
| `make evm-conformance` | détecte les régressions de fork rule et d’exécution |
| `go test ./modules/evm -count=1` | vérifie receipts, logs, storage, balances et snapshots |
| `go test ./rpc -count=1` | vérifie la compatibilité Web3 request/response |
| `make network-e2e` | confirme que le nœud démarre, a des peers et commit toujours |
| Docker single-host smoke | confirme le chemin utilisé par Remix et les outils navigateur |
| Contract deploy | confirme l’admission des transactions et la génération des receipts |
| Proxy deploy | confirme les hypothèses ABI et storage layout |
| UUPS upgrade | confirme la sémantique d’upgrade et les lectures après upgrade |

Si un seul point est rouge, ne considérez pas la mise à jour comme terminée.

## Critères de rollback

Revenez en arrière si l’un des cas suivants arrive :

- `eth_chainId` change de manière inattendue
- `eth_sendRawTransaction` commence à rejeter des transactions valides
- `eth_call` ou `eth_estimateGas` divergent des fork rules attendues
- les receipts, logs ou proofs ne correspondent plus à l’état committed
- les transactions de proxy ou d’upgrade commencent à échouer
- les preuves de release ne correspondent plus au chemin de code actuel

Le rollback doit restaurer ensemble la dernière version d’adapter connue comme bonne, les valeurs par défaut de config et le jeu de fixtures.

## Annexe de parité technique

Cette annexe garde le guide de mise à jour aligné avec le reste de l’arborescence documentaire.

- Conservez `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc` et `cmd/vexod` comme frontières stables d’implémentation.
- Conservez l’orthographe de `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction` et `eth_sendTransaction`.
- Conservez aussi l’orthographe de `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures` et `--evm-web3-conformance-evidence`.
- La question opérationnelle reste simple : cette mise à jour préserve-t-elle l’exécution de style Ethereum tout en restant compatible avec la sécurité de Vexo consensus et de release ?

<!-- vexo-docs:technical-parity -->
