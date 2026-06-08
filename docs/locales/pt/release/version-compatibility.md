# Version Compatibility Matrix

> Locale: pt · Português
> Este documento é um guia traduzido a partir da documentação canônica em inglês. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Objetivo

Este documento cobre matriz de compatibilidade de versões e critérios de upgrade. Comandos, campos JSON, nomes RPC, config key e identificadores de código usados na implementação e operação permanecem em inglês por compatibilidade.

## Escopo principal

- Verifique os itens abaixo ao ler este documento. Comandos, campos JSON, métodos RPC, chaves de configuração e identificadores de código permanecem em inglês por compatibilidade.
- Para texto normativo detalhado, use o documento em inglês.
- Canonical path: `docs/release/version-compatibility.md`
- Locale path: `docs/locales/pt/release/version-compatibility.md`

## Identificadores preservados

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `/v1/*`
- `vexod upgrade plan --json`
- `vexod upgrade apply`
- `rollback_required`
- `make release-candidate`

## Seções em inglês

- Version Compatibility Matrix
- Current Matrix
- Upgrade Compatibility Checklist
- Rollback Drill

## Notas operacionais

- `MUST`, `SHOULD`, `MAY`, exemplos de comando, exemplos JSON e nomes RPC preservam a grafia em inglês.
- Após alterar esta tradução, execute `make docs-check`.
- Se esta página divergir da fonte inglesa, use a fonte inglesa e atualize este arquivo locale na mesma mudança.

## Fonte canônica

- [English canonical document](../../en/release/version-compatibility.md)
