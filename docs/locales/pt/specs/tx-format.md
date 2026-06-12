# Transaction Format

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender transaction format, signing, fee e regras de gas e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/pt/specs/tx-format.md`

## Por que ler este documento

- transaction format, signing, fee e regras de gas
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
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## Estrutura da fonte inglesa

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

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/tx-format.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Canonical Payload — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Address Format — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Signed Envelope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Required Ante Metadata — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: CheckTx Requirements — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Fee and Gas — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Load Test Payloads — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: CLI Examples — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `gas_limit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_chain_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `chain_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `base_fee` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `max(min_fee, base_fee * gas)` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blob_base_fee` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blob_gas` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blob_gas_fee_cap` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_sendRawBlobTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blob_hashes` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_getBlobSidecarByTxHash` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_getBlobSidecarByBlobHash` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_chainId` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `net_version` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sendRawTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dynamic_base_fee` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `target_gas` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dynamic_blob_base_fee` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `target_blob_gas` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bank:send` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
