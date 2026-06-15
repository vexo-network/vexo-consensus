# Node Initialization

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento serve tanto para quem cria um node home pela primeira vez quanto para quem já opera nós. Se esta for a sua primeira leitura, siga esta ordem.

1. O que você está construindo
2. Execução local em cinco minutos
3. Rede local com quatro validadores
4. Web3 e Remix
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

Essa ordem corresponde ao que o operador precisa validar primeiro: entender o que é um node home, confirmar a inicialização local, diferenciar validator e archive, e depois revisar peers, timing e tratamento de falhas.

## Visão geral

Este documento ajuda a entender inicialização de nós archive/validator e uso de arquivos de configuração separados e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/pt/operators/node-initialization.md`

## Por que ler este documento

- inicialização de nós archive/validator e uso de arquivos de configuração separados
- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Este documento localizado ajuda na compreensão; auditoria, release e segurança são decididos pela fonte inglesa.

## O que você deve conseguir fazer

- Explicar qual decisão de implementação ou operação este documento apoia.
- Relacionar os requisitos normativos da fonte inglesa com a configuração atual da rede.
- Verificar chain ID, validator ID, fee/gas e endereços peer antes de copiar exemplos.

## Checklist de uso seguro

- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Não traduza comandos, config key, nomes RPC, campos JSON nem identificadores de código.
- Antes de copiar exemplos, ajuste chain ID, validator ID, fee/gas e endereços peer para sua rede.
- Após alterar documentos, execute `make docs-check` para verificar locale tree e guards de tradução.

## Pontos de atenção

- Este documento localizado ajuda na compreensão; auditoria, release e segurança são decididos pela fonte inglesa.
- Quando a implementação mudar, atualize a fonte inglesa e todos os documentos localizados na mesma alteração.

## Interfaces que devem ser preservadas

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
## Estrutura da fonte inglesa

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## Fonte canônica

- [Documento canônico em inglês](../../en/operators/node-initialization.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Nota operacional recente

Em um novo home de nó, revise juntos `p2p.dial_timeout`, `p2p.auth_replay_path` e `p2p.require_auth_replay_store` em `network_config.json`. O padrão `10s` cobre TCP dial, TLS, signed handshake e replay-store. Em redes públicas, mantenha isso na configuração revisada, não escondido em flags de shell.

## Sincronização de estado na inicialização na inicialização

O bloco `state_sync` em `network_config.json` atende novos nós archive, validator substitutos e nós restaurados em uma máquina limpa. Quando `state_sync.enabled` é true, `vexod start` tenta `state_sync.snapshot_urls` em ordem, verifica chain ID, checksum, state root e KV namespace, restaura no LevelDB, reconstrói índices e só então inicia o nó. Se o estado local já atende `state_sync.min_height` e `state_sync.trust_local_higher` é true, o store local é mantido e `state_sync_skipped` é registrado.

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

Monitore `state_sync_candidate_failed`, `state_sync_candidate_rejected` e `state_sync_applied`. Em rede pública, não use snapshot de terceiros sem política de confiança e finality/light-client evidence.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Validator Node — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Archive Node — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Split Configuration Files — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Key Types — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Config-Based Peers — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Consensus Timing — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Multi-Validator Network — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `network_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod start` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--timeout-propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--create-empty-blocks` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--p2p-auth-token` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--rpc-admin-token` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-account-key-env` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-account-key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `VEXO_KEY_PASSPHRASE` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--passphrase` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--encrypt-keys` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `node.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator.vrf.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety=true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--key-type bls` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `genesis.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bls_pop` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `module_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `mempool_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `log_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `data/` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json:p2p.node_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `shutdown_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_max_subscriptions_per_connection` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_idle_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `auth_replay_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_auth_replay_store` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dial_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `data/p2p_auth_replay.jsonl` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--key-type ed25519` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf_key_paths` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf_public_key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `<home>/<name>_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.evm_account_key_envs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.evm_account_private_keys` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_accounts` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sign` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_signTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sendTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_account_key_envs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod config paths --home <home>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `"require_network_safety": true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `host:port` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.listen_address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.seeds` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.node_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.node_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_cert_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_ca_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_server_name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.dial_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_prevote` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_precommit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `create_empty_blocks: false` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit: "finalized"` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit: "qc"` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `round_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `create_empty_blocks` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod network up` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make network-e2e` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator-%d` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_advertise_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_advertise_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_listen_host` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_listen_host` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
