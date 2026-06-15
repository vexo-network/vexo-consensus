# Validator Lifecycle

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica a especificação normativa de Validator Lifecycle. Se esta for a sua primeira leitura, siga esta ordem.

1. Scope
2. Admission
3. Validator Set
4. Rotation
5. Evidence Lifecycle
6. Slashing
7. Jail and Unbonding

Essa ordem corresponde à forma correta de ler o documento: primeiro o escopo e o estado, depois as regras de mensagens, segurança e vivacidade, e por fim as evidências.

## Visão geral

Este documento ajuda a entender ciclo validator join, rotation, jail, slashing e leave e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/pt/specs/validator-lifecycle.md`

## Por que ler este documento

- ciclo validator join, rotation, jail, slashing e leave
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Estrutura da fonte inglesa

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/validator-lifecycle.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Admission — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Validator Set — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Rotation — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Evidence Lifecycle — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Slashing — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Jail and Unbonding — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `vexovaloper...` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexovalcons...` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo...` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `staking tx withdraw-unbonded` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
