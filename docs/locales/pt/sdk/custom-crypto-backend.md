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
- Objetivo
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

Mantenha estes nomes de interface inalterados: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Goal — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Interfaces — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Runtime Suite — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Domain Separation — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Production BLS Requirements — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Production VRF Requirements — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Remote Signer Requirements — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Test Backends — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `vexo-consensus` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `supranational/blst` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo.consensus.proposal.v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo.consensus.vote.v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo.consensus.timeout_vote.v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo.finality.proof.v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.adapter_name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `BLSAdapter.Metadata().Name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.audit_evidence_sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bls_pop` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `github.com/supranational/blst` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RELEASE_CGO_ENABLED=1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RELEASE_REQUIRE_BLS=1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make release-portable RELEASE_REQUIRE_BLS=0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `circl-bls12381-g1sigg2-basic-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bls_proof_of_possession` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.adapter_name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.audit_report` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.dependency_audit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.audit_evidence_sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.key_source` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `committee.backend` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.NewProductionVRF` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `production_adapter: true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `ecvrf-p256-sha256-tai-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf_public_key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `remote-vrf-http-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `remote-http:<base-url>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `POST /prove` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `public_key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `issued_at_unix_nano` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `deadline_unix_nano` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo.remote_vrf.prove.v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `POST /verify` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo.remote_vrf.verify.v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `{ "valid": true, "nonce": "<same nonce>" }` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `VEXO_REMOTE_VRF_TOKEN` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `Authorization: Bearer <token>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.tls_cert_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.tls_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.tls_ca_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.tls_server_name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `keys serve-vrf` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--auth-token` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--auth-token-env` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod keys serve-vrf` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.NewRemoteVRFService` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--home` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `remote-vrf-nonces.jsonl` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `remote-vrf-audit.jsonl` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--nonce-path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--audit-log` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.RemoteVRFServiceConfig.ReplayStore` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RequireDurableReplayStore: true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.NewFileRemoteVRFReplayStore` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf_key_paths` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `VEXO_KEY_PASSPHRASE` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.keys` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod keys serve-remote` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--guard-path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_proposal` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_vote` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_timeout_vote` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `finality_proof` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
