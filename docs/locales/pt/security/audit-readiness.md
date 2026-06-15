# Preparação para auditoria de segurança

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender threat model, premissas de segurança e evidências de auditoria e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/pt/security/audit-readiness.md`

## Por que ler este documento

- threat model, premissas de segurança e evidências de auditoria
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
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## Estrutura da fonte inglesa

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Objetivos de segurança
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

A entrega de auditoria deve incluir VRF adapter audit evidence além de BLS. Fixe o SHA-256 de um arquivo como `docs/security/ecvrf-audit-evidence.json` em `vrf.audit_evidence_sha256` ou `--vrf-audit-sha256`, e revise dependency audit, key custody, TLS/mTLS ou pinned CA, auth, replay defense e service availability no mesmo limite.

## Fonte canônica

- [Documento canônico em inglês](../../en/security/audit-readiness.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Threat Model — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Security Assumptions — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Known Limitations — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Formal-ish Safety Argument — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Required Evidence for Audit — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Auditor Focus Areas — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Practical Audit Walkthrough — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Remote Signer Audit Notes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: EVM/Web3 Audit Notes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Snapshot and WAL Audit Notes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `docs/security/blst-audit-evidence.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `remote-vrf-http-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod keys serve-vrf` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release collect-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/*` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `chain_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `go.mod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/recovery/report` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
