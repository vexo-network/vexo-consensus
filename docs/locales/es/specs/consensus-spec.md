# Especificación del consenso

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica la especificación normativa de Consensus Spec. Si es tu primera lectura, sigue este orden.

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

Ese orden coincide con la forma correcta de leerlo: primero el alcance y el estado, luego las reglas de mensajes, seguridad y vivacidad, y al final las evidencias.

## Resumen

Este documento ayuda a entender especificación normativa de la state machine de consenso y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/es/specs/consensus-spec.md`

## Por qué leer este documento

- especificación normativa de la state machine de consenso
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

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## Estructura de la fuente inglesa

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## Fuente canónica

- [Documento canónico en inglés](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Bloques vacíos y recuperación de round

Con `create_empty_blocks=false`, una height estable con mempool vacío es un estado idle normal. Cuando entra una transacción, el nodo puede avanzar al siguiente local proposer round para construir un bloque con transacciones, manteniendo las reglas QC/finality.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Roles — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: State — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Message Types — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Safety Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Finality Rule — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Execution Commit Policy — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Liveness Assumptions — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Empty Blocks and Round Recovery — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Evidence — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `chain_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator_set_hash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `locked_qc` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `high_qc` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `last_timeout_cert` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `last_finalized` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `>= 2/3` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `B3.height = B2.height + 1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `B2.height = B1.height + 1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit = "qc"` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit = "finalized"` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `block_committed` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `create_empty_blocks = false` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_height = 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `actual_hash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `actual_time_unix_nano` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `parity_shards` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
