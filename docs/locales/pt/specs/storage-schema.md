# Storage Schema

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender namespace de durable storage, key schema e recovery marker e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/pt/specs/storage-schema.md`

## Por que ler este documento

- namespace de durable storage, key schema e recovery marker
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
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Estrutura da fonte inglesa

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

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Backend — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Records — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Indexes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: EVM Records — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Recovery Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Snapshot Validation — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Schema Migration — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `store.Store` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_ethstate` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_getBalance` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_getProof` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bank/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `auth/nonce/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm/code/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm/storage/{0x_address}/{slot}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_ethstate/{height}/meta` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_ethstate/{height}/accounts/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_ethstate/{height}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `seen_ttl` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `code/{address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `storage/{address}/{slot}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `receipts/{tx_hash}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `logs/{address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
