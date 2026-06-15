# Formato da prova de finality

> Locale: pt · Português
> Este documento é uma tradução direta para o português da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica a especificação normativa de Finality Proof Format. Se esta for a sua primeira leitura, siga esta ordem.

1. Scope
2. Proof Fields
3. Header Fields
4. Quorum Certificate Fields
5. Commit Chain Fields
6. Verification Algorithm
7. Accountable Safety Detection
8. Ed25519 Model
9. BLS Model

Essa ordem corresponde à forma correta de ler o documento: primeiro o escopo e o estado, depois as regras de mensagens, segurança e vivacidade, e por fim as evidências.

## Visão geral

Este documento ajuda a entender campos de finality proof, ordem de verificação e validator set binding e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/pt/specs/finality-proof-format.md`

## Por que ler este documento

- campos de finality proof, ordem de verificação e validator set binding
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
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## Estrutura da fonte inglesa

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Proof Fields — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Header Fields — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Quorum Certificate Fields — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Commit Chain Fields — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Verification Algorithm — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Accountable Safety Detection — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Ed25519 Model — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: BLS Model — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `finality.Proof` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/{height}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `strict: true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/status.latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/*` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `Proof.ValidatorSetHeight <= Header.Height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `Proof.ValidatorSetHash == loaded_set.Hash()` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `Header.ValidatorSetHash == loaded_set.Hash()` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `QuorumCert.Height == Header.Height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `Header.TxRoot` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `HeaderHash(link.Header)` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `finality.AttackDetector` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--validator-set` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `supranational/blst` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
