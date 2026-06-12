# Custom Storage and Transport Guide

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender implementação e registro de custom storage e transport adapter e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/sdk/custom-storage-transport.md`
- Locale path: `docs/locales/pt/sdk/custom-storage-transport.md`

## Por que ler este documento

- implementação e registro de custom storage e transport adapter
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
- `store.HistoricalSnapshotKVStore`
- `store.SnapshotKVStore`
- `transport.Transport`

## Estrutura da fonte inglesa

- Custom Storage and Transport Guide
- Custom Storage
- Storage Requirements
- Custom Transport
- Transport Requirements
- Compatibility

## Fonte canônica

- [Documento canônico em inglês](../../en/sdk/custom-storage-transport.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Custom Storage — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Storage Requirements — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Custom Transport — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Transport Requirements — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Compatibility — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `store.Store` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `store.HistoricalSnapshotKVStore` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `store.SnapshotKVStore` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `store.AppBlockCommitStore` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod start` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `runtime.NewNetworkSafeWithStore` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `runtime.NewNetworkSafeWithStoreContext` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `runtime.NewNetworkSafeWithStoreAndCryptoRegistryContext` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `config.ValidateNetworkSafety` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `app.AtomicBlockApplication` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `transport.Transport` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `transport.GRPCConfig.RequireTLS` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
