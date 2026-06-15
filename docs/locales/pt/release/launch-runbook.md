# Runbook de lançamento

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica o fluxo de release e operação de Launch Runbook. Se esta for a sua primeira leitura, siga esta ordem.

1. At a Glance
2. Prelaunch Gate
3. Release Candidate Gate
4. Genesis Gate
5. Launch Window
6. Postlaunch Archive

Essa ordem corresponde ao uso real: primeiro os objetivos e gates, depois os artefatos e requisitos de evidência, e por fim as etapas de execução.

## Visão geral

Este documento ajuda a entender checklist operacional e procedimento antes do lançamento da rede e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/pt/release/launch-runbook.md`

## Por que ler este documento

- checklist operacional e procedimento antes do lançamento da rede
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## Estrutura da fonte inglesa

- Runbook de lançamento
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Evidência de conformidade EVM/Web3

Antes de uma publicação pública, arquive `--evm-web3-conformance-evidence` separado de `--sdk-conformance-evidence`. O arquivo deve conter `evm_fixtures`, `evm_execution`, `web3_rpc` e `evm_corpus` para que `release gate` rejeite resumos que não possam ser verificados.

## VRF audit evidence SHA-256

Ao validar um release candidate, passe ao `release gate` os digests de auditoria BLS e VRF. Use pelo menos `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` e `--evidence-manifest`, e confirme que cada arquivo evidence bate com o SHA-256 do manifest.

## Fonte canônica

- [Documento canônico em inglês](../../en/release/launch-runbook.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Prelaunch Gate — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Candidate Gate — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Genesis Gate — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Launch Window — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Postlaunch Archive — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `release docs-quality` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `checksums.txt` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `sbom-go-modules.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `sbom-go-version.txt` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-manifest.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-audit-pack.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release collect-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network analyze-longrun` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `longrun-evidence.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-default-fixtures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-tx-fixtures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-tx-fixtures-dir` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-execution-fixtures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-execution-fixtures-dir` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-tx-fixtures-sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-execution-fixtures-sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-web3-conformance-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_fixtures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_execution` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_rpc` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_corpus` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod ops conformance` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `relayer soak-plan` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `chain_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evidence-manifest.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
