# Validator Lifecycle

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

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
