# Release Pipeline

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Visão geral

Este documento ajuda a entender pipeline de release com binários assinados, checksums e SBOM e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/pt/release/release-pipeline.md`

## Por que ler este documento

- pipeline de release com binários assinados, checksums e SBOM
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
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## Estrutura da fonte inglesa

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Runbook de lançamento

## VRF audit evidence SHA-256

`release gate` não fixa apenas a evidence de auditoria BLS; a evidence de auditoria VRF também deve ser fixada por SHA-256. O arquivo `--vrf-audit` deve estar em `evidence-manifest.json`, e `--vrf-audit-sha256` deve corresponder exatamente ao conteúdo. Com config, `vrf.audit_evidence_sha256` é o digest pin padrão. A regra confirma que VRF service, KMS/HSM custody, TLS/mTLS ou pinned CA, auth token e nonce replay defense estão ligados à evidence de release.

## Fonte canônica

- [Documento canônico em inglês](../../en/release/release-pipeline.md)

## Termos de attestation das evidências de release

Em releases públicos, cada entrada de `evidence-manifest.json` deve ser verificada por assinatura Ed25519. Mantenha os seguintes flags de CLI e campos JSON sem tradução.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
