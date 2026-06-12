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
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-smoke`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
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
- Objetivos
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Runbook de lançamento

## Evidência de conformidade EVM/Web3

`--sdk-conformance-evidence` e `--evm-web3-conformance-evidence` são evidências separadas. Um resumo dizendo que “EVM passed” não basta; a evidência EVM/Web3 deve incluir as seções legíveis por máquina `evm_fixtures`, `evm_execution`, `web3_rpc` e `evm_corpus`, e precisa estar vinculada ao `evidence-manifest.json` por SHA-256 antes de qualquer declaração pública de compatibilidade.

## Política de release candidate

Uma release candidate pública usa por padrão `make release-candidate`. Esse target é o gate real, aponta para `release-candidate-real` e exige `RELEASE_CGO_ENABLED=1` para que o artifact inclua de fato o adapter BLS `supranational/blst` baseado em cgo. `make release-candidate-plan` serve apenas para PR smoke e planejamento operacional; ele usa fixtures embutidas e planos dry-run, portanto não deve ser entregue como evidence final. Se precisar de um artifact no-cgo, use `make release-portable RELEASE_REQUIRE_BLS=0`, mas não o publique como release BLS-capable. Quando `RELEASE_CGO_ENABLED=1` e `RELEASE_TARGETS` não está definido, o Makefile compila apenas o target do host atual. Para vários OS/arquiteturas, defina `RELEASE_TARGETS` explicitamente em runners com os cross-compilers cgo necessários.

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
<!-- vexo-docs-ops-update-2026-06 -->

## Interpretação do E2E de rede

`make network-e2e` não é apenas um teste de build: ele inicia 4 validators com o binário real e verifica signed-shape smoke transaction, conexão peer, avanço de height e clean stop. `NETWORK_E2E_GO_TIMEOUT` é o limite externo do Go test e deve ser maior que o timeout interno da rede.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Goals — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Commands — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: CI Gates — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Evidence Quality Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Artifacts — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Reproducibility Notes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Signed Binaries — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: SBOM — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Audit Pack — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Candidate Targets — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Launch Runbook — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `network analyze-longrun` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release collect-evidence` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `ops-runbook` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p-scale` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `state-sync-light-client` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `snapshot-replay` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make check` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make fuzz-smoke` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod consensus adversarial` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod ops conformance` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod network longrun` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod network chaos-plan` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make network-e2e` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make race` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `NETWORK_E2E_GO_TIMEOUT` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make test` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make vet` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make docs-check` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make build` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make release-candidate-smoke VERSION=ci`
- `make release-candidate-plan VERSION=ci` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make release-candidate VERSION=<rc> RELEASE_CGO_ENABLED=1 RC_EVM_CONFORMANCE_FLAGS=...` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evidence-manifest.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--allow-external-pending` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--private-rc` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo-release-evidence-attestation-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release evidence-manifest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--signing-key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--signing-key-env` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `<evidence-file>.sig` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `<evidence-file>.sig.pub` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `<evidence-file>.pub` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dist/` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod-<version>-<os>-<arch>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `checksums.txt` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `checksums.txt.asc` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `sbom-go-modules.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `sbom-go-version.txt` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-manifest.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-audit-pack.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `longrun-analysis.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs-quality.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RELEASE_CGO_ENABLED=1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `supranational/blst` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `go build -trimpath` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `BUILD_DATE` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make release-candidate` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make release-portable RELEASE_REQUIRE_BLS=0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RELEASE_TARGETS` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-candidate` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-candidate-real` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod ops conformance --strict` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RC_EVM_CONFORMANCE_FLAGS` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RC_LONGRUN_DURATION` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `release-candidate-plan` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `RELEASE_REQUIRE_BLS=0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `allow_noop_migrations=true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod upgrade apply --allow-empty-migrations` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--bls-audit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--bls-audit-sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--config <path>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `crypto.audit_evidence_sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--vrf-audit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--vrf-audit-sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf.audit_evidence_sha256` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/security/blst-audit-evidence.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/security/ecvrf-audit-evidence.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
