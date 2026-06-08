# Validator Lifecycle

> Locale: pt · Português
> Este documento é um guia traduzido a partir da documentação canônica em inglês. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Objetivo

Este documento cobre ciclo validator join, rotation, jail, slashing e leave. Comandos, campos JSON, nomes RPC, config key e identificadores de código usados na implementação e operação permanecem em inglês por compatibilidade.

## Escopo principal

- Verifique os itens abaixo ao ler este documento. Comandos, campos JSON, métodos RPC, chaves de configuração e identificadores de código permanecem em inglês por compatibilidade.
- Para texto normativo detalhado, use o documento em inglês.
- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/pt/specs/validator-lifecycle.md`

## Identificadores preservados

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Seções em inglês

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Notas operacionais

- `MUST`, `SHOULD`, `MAY`, exemplos de comando, exemplos JSON e nomes RPC preservam a grafia em inglês.
- Após alterar esta tradução, execute `make docs-check`.
- Se esta página divergir da fonte inglesa, use a fonte inglesa e atualize este arquivo locale na mesma mudança.

## Fonte canônica

- [English canonical document](../../en/specs/validator-lifecycle.md)
