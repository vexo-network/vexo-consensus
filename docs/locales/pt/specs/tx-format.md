# Transaction Format

> Locale: pt · Português
> Este documento é um guia traduzido a partir da documentação canônica em inglês. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Objetivo

Este documento cobre transaction format, signing, fee e regras de gas. Comandos, campos JSON, nomes RPC, config key e identificadores de código usados na implementação e operação permanecem em inglês por compatibilidade.

## Escopo principal

- Verifique os itens abaixo ao ler este documento. Comandos, campos JSON, métodos RPC, chaves de configuração e identificadores de código permanecem em inglês por compatibilidade.
- Para texto normativo detalhado, use o documento em inglês.
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/pt/specs/tx-format.md`

## Identificadores preservados

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`

## Seções em inglês

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## Notas operacionais

- `MUST`, `SHOULD`, `MAY`, exemplos de comando, exemplos JSON e nomes RPC preservam a grafia em inglês.
- Após alterar esta tradução, execute `make docs-check`.
- Se esta página divergir da fonte inglesa, use a fonte inglesa e atualize este arquivo locale na mesma mudança.

## Fonte canônica

- [English canonical document](../../en/specs/tx-format.md)
