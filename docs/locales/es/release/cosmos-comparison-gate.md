# Cosmos/Tendermint Comparison Gate

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica el flujo de release y operación de Cosmos Comparison Gate. Si es tu primera lectura, sigue este orden.

1. Required Evidence Properties
2. Release Rule

Ese orden coincide con el uso real: primero los objetivos y gates, luego los artefactos y requisitos de evidencia, y al final los pasos de ejecución.

## Resumen

Este documento ayuda a entender la puerta de release frente a expectativas tipo Cosmos/Tendermint y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/es/release/cosmos-comparison-gate.md`

## Por qué leer este documento

- la puerta de release frente a expectativas tipo Cosmos/Tendermint
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
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## Estructura de la fuente inglesa

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## Fuente canónica

- [Documento canónico en inglés](../../en/release/cosmos-comparison-gate.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Required Evidence Properties — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Rule — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `--longrun-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--chaos-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--ops-runbook-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--external-audit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--formal-safety-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--fuzz-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--sdk-conformance-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-web3-conformance-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--p2p-scale-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--state-sync-light-client-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--snapshot-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--validator-economics-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--upgrade-governance-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--mev-fee-market-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--kms-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--bls-audit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
