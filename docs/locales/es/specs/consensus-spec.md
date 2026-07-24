# Especificación del consenso

> Locale: es · Español
> Este documento es una traducción directa al español de la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


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
- Esta página es una traducción directa del original en inglés; la auditoría, el release y la seguridad se deciden con la fuente inglesa.

## Qué debería poder hacer después

- Explicar qué decisión de implementación u operación apoya este documento.
- Relacionar los requisitos normativos de la fuente inglesa con la configuración actual de la red.
- Verificar chain ID, validator ID, fee/gas y direcciones peer antes de copiar ejemplos.

## Checklist de uso seguro

- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- No traduzca comandos, config key, nombres RPC, campos JSON ni identificadores de código.
- Antes de copiar ejemplos, adapte chain ID, validator ID, fee/gas y direcciones peer a su red.
- Después de modificar la documentación, ejecute `make docs-check` para verificar el árbol local y los controles de traducción.

## Puntos de atención

- Esta página es una traducción directa del original en inglés; la auditoría, el release y la seguridad se deciden con la fuente inglesa.
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

Con `create_empty_blocks=false`, una height estable con mempool vacío es un estado idle normal. Cuando entra una transacción, el nodo solo propone si es el proposer determinista del `(height, round)` actual; un nodo que no es proposer no salta localmente a otra round. Las rounds solo avanzan mediante un timeout certificate válido o una transición de finality certificada, y los errores de ejecución o almacenamiento no se tratan como timeout.

## Timeout de ronda adaptativo

Con `adaptive_round_timeout_enabled = true`, el nodo calcula el timeout usando el valor base, el p95 móvil del procesamiento proposal/vote/commit, el resultado del progreso y el déficit de peers activos. Un timeout multiplica por 1,5, el progreso reduce por 0,8 y la latencia observada aporta un margen de 3 veces, limitado entre la base y 8 veces la base. No cambia safe-vote, proposer, quorum power, QC ni three-chain finality.

## Compuerta de finalidad durante recuperación

Con `recovery_finality_gate_enabled = true`, si difieren las alturas durables de application state y block index, su mínimo es la safe recovery height. Los finalized application commits superiores se aplazan hasta recuperar la coherencia. El timeout actual y los aplazamientos se observan mediante `/v1/metrics` y `/metrics/text`.

## Orden determinista de transacciones

La proposal deriva un salt de `chain_id` y height, ordena por nonce ascendente la transaction chain de cada signer y fusiona sus cabeceras por salted transaction hash. Para el mismo candidate set elimina el orden local de llegada al mempool, pero no garantiza first-seen fairness, censorship resistance, confidentiality ni order fairness formal.

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
