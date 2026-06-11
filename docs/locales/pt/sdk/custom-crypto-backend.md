# Custom Crypto Backend Guide

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender integração de custom crypto backend como BLS, VRF e signer e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/pt/sdk/custom-crypto-backend.md`

## Por que ler este documento

- integração de custom crypto backend como BLS, VRF e signer
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

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`
## Estrutura da fonte inglesa

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## VRF audit evidence SHA-256

O VRF backend deve expor uma fronteira de auditoria tão clara quanto BLS. Preencha `vrf.adapter_name`, `vrf.audit_report`, `vrf.dependency_audit`, `vrf.audit_evidence_sha256` e `vrf.key_source`; se os metadata do adapter divergem da config, o runtime deve fail closed. O adapter ECVRF integrado verifica o go.mod dependency pin e o audit evidence digest; o remote VRF adapter usa uma referência externa de auditoria KMS/HSM.

## Fonte canônica

- [Documento canônico em inglês](../../en/sdk/custom-crypto-backend.md)

## Remote VRF service

`vexod keys serve-vrf` expõe `POST /prove` e `POST /verify` com uma chave ECVRF, e `vexod keys verify-vrf` valida o remote prover de ponta a ponta. Mantenha `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1` e `vexo.remote_vrf.verify.v1` sem tradução.

Keep these interface names unchanged: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.
