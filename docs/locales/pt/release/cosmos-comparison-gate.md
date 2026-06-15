# Marco de comparação Cosmos/Tendermint

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica o fluxo de release e operação de Cosmos Comparison Gate. Se esta for a sua primeira leitura, siga esta ordem.

1. Required Evidence Properties
2. Release Rule

Essa ordem corresponde ao uso real: primeiro os objetivos e gates, depois os artefatos e requisitos de evidência, e por fim as etapas de execução.

## Visão geral

Este documento ajuda a entender o gate de release frente a expectativas estilo Cosmos/Tendermint e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/pt/release/cosmos-comparison-gate.md`

## Por que ler este documento

- o gate de release frente a expectativas estilo Cosmos/Tendermint
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

- `release gate`
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## Estrutura da fonte inglesa

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## Fonte canônica

- [Documento canônico em inglês](../../en/release/cosmos-comparison-gate.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Required Evidence Properties — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Rule — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `--longrun-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--chaos-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--ops-runbook-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--external-audit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--formal-safety-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--fuzz-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--sdk-conformance-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-web3-conformance-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--p2p-scale-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--state-sync-light-client-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--snapshot-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--validator-economics-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--upgrade-governance-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--mev-fee-market-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--kms-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--bls-audit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
