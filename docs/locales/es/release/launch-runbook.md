# Runbook de lanzamiento

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica el flujo de release y operación de Launch Runbook. Si es tu primera lectura, sigue este orden.

1. At a Glance
2. Prelaunch Gate
3. Release Candidate Gate
4. Genesis Gate
5. Launch Window
6. Postlaunch Archive

Ese orden coincide con el uso real: primero los objetivos y gates, luego los artefactos y requisitos de evidencia, y al final los pasos de ejecución.

## Resumen

Este documento ayuda a entender checklist operativa y procedimiento antes del lanzamiento de red y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/es/release/launch-runbook.md`

## Por qué leer este documento

- checklist operativa y procedimiento antes del lanzamiento de red
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
## Estructura de la fuente inglesa

- Runbook de lanzamiento
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Evidencia de conformidad EVM/Web3

Antes de una publicación pública, archiva `--evm-web3-conformance-evidence` separado de `--sdk-conformance-evidence`. El archivo debe contener `evm_fixtures`, `evm_execution`, `web3_rpc` y `evm_corpus` para que `release gate` pueda rechazar resúmenes no verificables.

## VRF audit evidence SHA-256

Al validar un release candidate, pasa a `release gate` los digests de auditoría BLS y VRF. Usa al menos `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` y `--evidence-manifest`, y comprueba que cada archivo evidence coincida con el SHA-256 del manifest.

## Fuente canónica

- [Documento canónico en inglés](../../en/release/launch-runbook.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Prelaunch Gate — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Candidate Gate — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Genesis Gate — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Launch Window — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Postlaunch Archive — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `release docs-quality` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `checksums.txt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `sbom-go-modules.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `sbom-go-version.txt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-manifest.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release-audit-pack.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release collect-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network analyze-longrun` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `longrun-evidence.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-default-fixtures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-tx-fixtures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-tx-fixtures-dir` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-execution-fixtures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-execution-fixtures-dir` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-tx-fixtures-sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-execution-fixtures-sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-web3-conformance-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_fixtures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_execution` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_rpc` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_corpus` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod ops conformance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `relayer soak-plan` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `chain_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evidence-manifest.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
