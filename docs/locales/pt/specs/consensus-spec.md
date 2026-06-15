# Especificação do consenso

> Locale: pt · Português
> Este documento é uma tradução direta para o português da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica a especificação normativa de Consensus Spec. Se esta for a sua primeira leitura, siga esta ordem.

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

Essa ordem corresponde à forma correta de ler o documento: primeiro o escopo e o estado, depois as regras de mensagens, segurança e vivacidade, e por fim as evidências.

## Visão geral

Este documento ajuda a entender especificação normativa da state machine de consenso e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/pt/specs/consensus-spec.md`

## Por que ler este documento

- especificação normativa da state machine de consenso
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

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## Estrutura da fonte inglesa

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Blocos vazios e recuperação de round

Com `create_empty_blocks=false`, height parada com mempool vazio é idle normal. Quando uma transação chega, o nó pode avançar para o próximo local proposer round e produzir um bloco com transações, preservando as regras QC/finality.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Roles — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: State — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Message Types — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Safety Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Finality Rule — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Execution Commit Policy — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Liveness Assumptions — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Empty Blocks and Round Recovery — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Evidence — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `chain_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator_set_hash` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `locked_qc` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `high_qc` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `last_timeout_cert` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `last_finalized` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `>= 2/3` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `B3.height = B2.height + 1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `B2.height = B1.height + 1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit = "qc"` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit = "finalized"` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `block_committed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `create_empty_blocks = false` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_height = 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `actual_hash` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `actual_time_unix_nano` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `parity_shards` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
