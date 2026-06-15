# Release Pipeline

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica el flujo de release y operación de Release Pipeline. Si es tu primera lectura, sigue este orden.

1. Goals
2. Release Commands
3. CI Gates
4. Evidence Quality Rules
5. Artifacts
6. Reproducibility Notes
7. Signed Binaries
8. SBOM
9. Audit Pack
10. Release Candidate Targets
11. Launch Runbook

Ese orden coincide con el uso real: primero los objetivos y gates, luego los artefactos y requisitos de evidencia, y al final los pasos de ejecución.

## Resumen

Este documento ayuda a entender pipeline de release con binarios firmados, checksums y SBOM y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/es/release/release-pipeline.md`

## Por qué leer este documento

- pipeline de release con binarios firmados, checksums y SBOM
- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- Este documento localizado ayuda a comprender; auditoría, release y seguridad se deciden con la fuente inglesa.

## Qué debería poder hacer después

- Explicar qué decisión de implementación u operación apoya este documento.
- Relacionar los requisitos normativos de la fuente inglesa con la configuración actual de la red.
- Verificar chain ID, validator ID, fee/gas y direcciones peer antes de copiar ejemplos.

## Checklist de uso seguro

- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- No traduzca comandos, config key, nombres RPC, campos JSON ni identificadores de código.
- Antes de copiar ejemplos, adapte chain ID, validator ID, fee/gas y direcciones peer a su red.
- Después de modificar documentación, ejecute `make docs-check` para verificar locale tree y guards de traducción.

## Puntos de atención

- Este documento localizado ayuda a comprender; auditoría, release y seguridad se deciden con la fuente inglesa.
- Si cambia la implementación, actualice la fuente inglesa y todos los documentos localizados en el mismo cambio.

## Interfaces que deben conservarse

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
## Estructura de la fuente inglesa

- Release Pipeline
- Objetivos
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Runbook de lanzamiento

## Evidencia de conformidad EVM/Web3

`--sdk-conformance-evidence` y `--evm-web3-conformance-evidence` son evidencias separadas. Un resumen textual que diga que “EVM passed” no es suficiente; la evidencia EVM/Web3 debe incluir las secciones legibles por máquina `evm_fixtures`, `evm_execution`, `web3_rpc` y `evm_corpus`, y debe estar enlazada a `evidence-manifest.json` por SHA-256 antes de cualquier afirmación pública de compatibilidad.

## Política de release candidate

Una release candidate pública usa por defecto `make release-candidate`. Ese target es el gate real, entra en `release-candidate-real` y exige `RELEASE_CGO_ENABLED=1` para que el artifact incluya realmente el adapter BLS `supranational/blst` basado en cgo. `make release-candidate-plan` solo sirve para PR smoke y planificación operativa; usa fixtures integradas y planes dry-run, por lo que no debe presentarse como evidence final. Si necesita un artifact no-cgo, use `make release-portable RELEASE_REQUIRE_BLS=0`, pero no lo publique como release BLS-capable. Cuando `RELEASE_CGO_ENABLED=1` y `RELEASE_TARGETS` no está definido, el Makefile compila solo el target del host actual. Para varios OS/arquitecturas, defina `RELEASE_TARGETS` explícitamente en runners con los cross-compilers cgo necesarios.

## VRF audit evidence SHA-256

`release gate` no solo fija la evidencia de auditoría BLS; la evidencia de auditoría VRF también debe fijarse con SHA-256. El archivo `--vrf-audit` debe estar en `evidence-manifest.json`, y `--vrf-audit-sha256` debe coincidir exactamente con su contenido. Con config, `vrf.audit_evidence_sha256` actúa como digest pin por defecto. Esta regla confirma que VRF service, KMS/HSM custody, TLS/mTLS o pinned CA, auth token y defensa contra nonce replay quedan unidos a la evidencia de release.

## Fuente canónica

- [Documento canónico en inglés](../../en/release/release-pipeline.md)

## Términos de attestation para evidencias de release

En una publicación pública, cada entrada de `evidence-manifest.json` debe verificarse con una firma Ed25519. Mantén sin traducir los siguientes flags de CLI y campos JSON.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
<!-- vexo-docs-ops-update-2026-06 -->

## Cómo leer el E2E de red

`make network-e2e` no es solo un test de build: arranca 4 validators con el binario real y verifica signed-shape smoke transaction, conexión peer, crecimiento de height y clean stop. `NETWORK_E2E_GO_TIMEOUT` es el límite externo de Go test y debe superar el timeout interno de red.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Goals — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Commands — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: CI Gates — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Evidence Quality Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Artifacts — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Reproducibility Notes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Signed Binaries — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: SBOM — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Audit Pack — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Candidate Targets — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Launch Runbook — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `network analyze-longrun` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release collect-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `ops-runbook` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p-scale` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `state-sync-light-client` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `snapshot-replay` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make check` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make fuzz-smoke` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod consensus adversarial` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod ops conformance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod network longrun` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod network chaos-plan` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make network-e2e` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make race` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `NETWORK_E2E_GO_TIMEOUT` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make test` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make vet` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make docs-check` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make build` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make release-candidate-smoke VERSION=ci`
- `make release-candidate-plan VERSION=ci` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make release-candidate VERSION=<rc> RELEASE_CGO_ENABLED=1 RC_EVM_CONFORMANCE_FLAGS=...` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evidence-manifest.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--allow-external-pending` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--private-rc` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo-release-evidence-attestation-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release evidence-manifest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--signing-key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--signing-key-env` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `<evidence-file>.sig` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `<evidence-file>.sig.pub` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `<evidence-file>.pub` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `dist/` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod-<version>-<os>-<arch>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `checksums.txt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `checksums.txt.asc` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `sbom-go-modules.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `sbom-go-version.txt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-manifest.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-audit-pack.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `longrun-analysis.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs-quality.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RELEASE_CGO_ENABLED=1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `supranational/blst` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `go build -trimpath` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `BUILD_DATE` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make release-candidate` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make release-portable RELEASE_REQUIRE_BLS=0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RELEASE_TARGETS` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-candidate` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-candidate-real` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod ops conformance --strict` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RC_EVM_CONFORMANCE_FLAGS` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RC_LONGRUN_DURATION` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-candidate-plan` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RELEASE_REQUIRE_BLS=0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `allow_noop_migrations=true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod upgrade apply --allow-empty-migrations` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--bls-audit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--bls-audit-sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--config <path>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.audit_evidence_sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--vrf-audit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--vrf-audit-sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.audit_evidence_sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/security/blst-audit-evidence.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/security/ecvrf-audit-evidence.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
