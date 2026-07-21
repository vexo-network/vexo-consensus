# Versionamento da API RPC

> Locale: pt · Português
> Este documento é uma tradução direta para o português da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender versionamento da RPC API, aliases de compatibilidade e política de estabilidade e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/pt/sdk/rpc-api-versioning.md`

## Por que ler este documento

- versionamento da RPC API, aliases de compatibilidade e política de estabilidade
- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Esta página é uma tradução direta do original em inglês; auditoria, release e segurança continuam sendo decididos pela fonte inglesa.

## O que você deve conseguir fazer

- Explicar qual decisão de implementação ou operação este documento apoia.
- Relacionar os requisitos normativos da fonte inglesa com a configuração atual da rede.
- Verificar chain ID, validator ID, fee/gas e endereços peer antes de copiar exemplos.

## Checklist de uso seguro

- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Não traduza comandos, config key, nomes RPC, campos JSON nem identificadores de código.
- Antes de copiar exemplos, ajuste chain ID, validator ID, fee/gas e endereços peer para sua rede.
- Após alterar os documentos, execute `make docs-check` para verificar a árvore local e os controles de tradução.

## Pontos de atenção

- Esta página é uma tradução direta do original em inglês; auditoria, release e segurança continuam sendo decididos pela fonte inglesa.
- Quando a implementação mudar, atualize a fonte inglesa e todos os documentos localizados na mesma alteração.

## Interfaces que devem ser preservadas

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
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`

## Estrutura da fonte inglesa

- RPC API Versioning
- Objetivo de estabilidade
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## Fonte canônica

- [Documento canônico em inglês](../../en/sdk/rpc-api-versioning.md)

## RPC capability discovery

A nova interface RPC capability discovery mostra quais funções provider estão realmente conectadas. Operadores usam `/v1/capabilities`; integrações SDK usam `rpc.Config.RequiredCapabilities` ou `rpc.Config.RequireAllCapabilities` para falhar no startup quando faltar uma capacidade.

Mantenha estes nomes de interface inalterados: `/v1/capabilities`, `CapabilityResponse`, `CapabilitySnapshot`, `RequiredCapabilities`, `RequireAllCapabilities`, `metrics`, `blocks`, `finality`, `strict_replay`, `consensus_control`.

<!-- vexo-docs:technical-parity -->
- `admin_token` and `admin_tokens` are stable configuration keys and must remain unchanged when describing optional bearer-token enforcement.

## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Stability Goal — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Current Stable API — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Versioning Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Capability Discovery — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Compatibility Aliases — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Error Format — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Query Proofs — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Event Queries — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: IBC Queries — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Web3 JSON-RPC Bridge — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Web3 EVM Configuration — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Operational Compatibility — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `/v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/healthz` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/readyz` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/status` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/diagnostics` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/capabilities` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/metrics` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/metrics/text` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/tx` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/recovery` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/snapshot/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/snapshot/export` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/snapshot/chunk?index=0&size=10000` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/blocks` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/blocks/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/blocks/{height}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/state/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/state/{height}/{namespace}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/events?key={attribute_key}&value={attribute_value}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/proof?namespace={namespace}&key={key}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/proof?namespace={namespace}&key={key}&height=latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/{height}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/ibc/client/{client_id}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/ibc/connection/{connection_id}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/ibc/channel/{port_id}/{channel_id}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/validators/{height}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/committee/{height}/{round}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/prune` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/replay` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/consensus/start` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/consensus/stop` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `tls_cert_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `tls_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `tls_ca_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `tls_server_name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod start` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `strict: true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_gasPrice` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_web3Capabilities` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.NewNetworkSafeServer` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.NewNetworkSafeHandlerWithConfig` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.Config.RequiredCapabilities` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.Config.RequireAllCapabilities` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `pending_txs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `state_by_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `app_query` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `strict_replay` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_control` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/status` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/tx` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/blocks/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/*` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v2/*` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/proof` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `commit_chain` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/status.latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/events` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `Index: true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `{ "path": [...], "value": ... }` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `packets/{source_port}/{source_channel}/{sequence}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_modules` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_clientVersion` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_sha3` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_accounts` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_coinbase` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `net_version` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `net_listening` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `net_peerCount` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_chainId` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_protocolVersion` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_syncing` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_mining` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_hashrate` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_blockNumber` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_blobBaseFee` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_maxPriorityFeePerGas` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_feeHistory` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
