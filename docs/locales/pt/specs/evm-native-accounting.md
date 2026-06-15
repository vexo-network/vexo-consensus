# EVM e contabilidade nativa

> Locale: pt · Português
> Este documento é uma tradução direta para o português da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica a especificação normativa de Evm Native Accounting. Se esta for a sua primeira leitura, siga esta ordem.

1. Core Rule
2. Amount Encoding
3. Fee Accounting
4. EVM Execution
5. State Root Policy
6. Compatibility Boundary
7. Failure Modes

Essa ordem corresponde à forma correta de ler o documento: primeiro o escopo e o estado, depois as regras de mensagens, segurança e vivacidade, e por fim as evidências.

## Visão geral

Este documento ajuda a entender conexão consistente entre native coin e EVM gas/accounting e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/pt/specs/evm-native-accounting.md`

## Por que ler este documento

- conexão consistente entre native coin e EVM gas/accounting
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

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`
- `bank query balance`

## Estrutura da fonte inglesa

- EVM e contabilidade nativa
- Core Rule
- Amount Encoding
- Fee Accounting
- Execução EVM
- State Root Policy
- Compatibility Boundary
- Failure Modes

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/evm-native-accounting.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Core Rule — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Amount Encoding — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Fee Accounting — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: EVM Execution — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: State Root Policy — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Compatibility Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Failure Modes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `base_fee * gas` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `contract.Invocation` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `value_hex` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `gas_price_hex` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `max_fee_per_gas_hex` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `max_priority_fee_per_gas_hex` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_getBalance` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sendRawBlobTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_sendRawBlobTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sendRawTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution.strict_evm_state_root` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
