# Runbook de lançamento

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

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
