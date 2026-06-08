# Finality Proof Format

> Locale: pt · Português
> Este documento é um guia traduzido a partir da documentação canônica em inglês. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Objetivo

Este documento cobre campos de finality proof, ordem de verificação e validator set binding. Comandos, campos JSON, nomes RPC, config key e identificadores de código usados na implementação e operação permanecem em inglês por compatibilidade.

## Escopo principal

- Verifique os itens abaixo ao ler este documento. Comandos, campos JSON, métodos RPC, chaves de configuração e identificadores de código permanecem em inglês por compatibilidade.
- Para texto normativo detalhado, use o documento em inglês.
- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/pt/specs/finality-proof-format.md`

## Identificadores preservados

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## Seções em inglês

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## Notas operacionais

- `MUST`, `SHOULD`, `MAY`, exemplos de comando, exemplos JSON e nomes RPC preservam a grafia em inglês.
- Após alterar esta tradução, execute `make docs-check`.
- Se esta página divergir da fonte inglesa, use a fonte inglesa e atualize este arquivo locale na mesma mudança.

## Fonte canônica

- [English canonical document](../../en/specs/finality-proof-format.md)
