# Visão geral do protocolo de consenso

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender o modelo de consenso, termos execution/commit/finality e limite de segurança e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/consensus-protocol.md`
- Locale path: `docs/locales/pt/consensus-protocol.md`

## Por que ler este documento

- o modelo de consenso, termos execution/commit/finality e limite de segurança
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

- `FinalizeBlock`
- `consensus_config.json`
- `execution_commit`
- `finalized`
- `qc`
- `require_network_safety`
- `block_committed`
- `deterministic`
- `ed25519`
- `bls`

## Estrutura da fonte inglesa

- Visão geral do protocolo de consenso
- Model
- Execution Terms
- Safety Boundary
- Crypto Boundary
- Operational Boundary

## Fonte canônica

- [Documento canônico em inglês](../en/consensus-protocol.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Model — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Execution Terms — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Safety Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Crypto Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Operational Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `consensus_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `block_committed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
