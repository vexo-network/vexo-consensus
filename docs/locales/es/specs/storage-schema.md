# Esquema de almacenamiento

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica la especificación normativa de Storage Schema. Si es tu primera lectura, sigue este orden.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Ese orden coincide con la forma correcta de leerlo: primero el alcance y el estado, luego las reglas de mensajes, seguridad y vivacidad, y al final las evidencias.

## Resumen

Este documento ayuda a entender namespace de durable storage, key schema y recovery marker y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/es/specs/storage-schema.md`

## Por qué leer este documento

- namespace de durable storage, key schema y recovery marker
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

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Estructura de la fuente inglesa

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## Fuente canónica

- [Documento canónico en inglés](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Backend — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Records — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Indexes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: EVM Records — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Recovery Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Snapshot Validation — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Schema Migration — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `store.Store` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_ethstate` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBalance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getProof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bank/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `auth/nonce/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/code/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/storage/{0x_address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_ethstate/{height}/meta` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_ethstate/{height}/accounts/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_ethstate/{height}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `seen_ttl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `code/{address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `storage/{address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipts/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/{address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
