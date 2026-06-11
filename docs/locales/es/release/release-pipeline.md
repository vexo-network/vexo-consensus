# Release Pipeline

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

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
## Estructura de la fuente inglesa

- Release Pipeline
- Goals
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
