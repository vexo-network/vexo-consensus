# Esquema de almacenamiento

> Locale: es · Español
> Este documento es una traducción directa al español de la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


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
- `eth_getProof`
- `stateRoot`
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

## Backend Pebble y registros binarios

El backend por defecto es Pebble (`github.com/cockroachdb/pebble/v2`), abierto con una caché de bloques de 256 MiB y memtables de 64 MiB. Los registros se guardan en la codificación binaria canónica del paquete `types`: uvarints mínimos, cadenas de bytes y listas con prefijo de longitud, hashes como sus 32 bytes, un valor opcional tras un byte de presencia `0` o `1`, y los campos en el orden de la estructura que los declara. Un decodificador rechaza cualquier otra cosa, incluidos bytes sobrantes, así que un registro dañado se informa en lugar de leerse como otro valor; cada registro tiene una sola codificación. Un directorio de datos escrito por un build anterior (LevelDB, registros JSON) no es legible ni se convierte: Pebble se niega a abrirlo con un error de `format major version 1`. Restaure ese nodo desde un snapshot o sincronícelo en un directorio de datos vacío.

Identificadores relacionados:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Historial de estado (undo log) y caché del árbol

El historial de estado es un undo log de versiones por clave. Cuando una clave cambia en una altura (las escrituras de un bloque en su altura, una escritura directa (`Set`, `Delete`, `SetBatch`) tras el primer estado confirmado en la siguiente altura confirmada), el store registra el valor que la clave tenía antes, ausencia incluida, como versión de la clave en esa altura, una vez por clave y altura; las escrituras directas anteriores al bloque que confirma su altura conservan la versión que registró la primera (`hdirect` indica esa altura). El valor de una clave en una altura confirmada es el que registra su versión más baja por encima de esa altura, o su valor actual si no tiene ninguna. Registrar un cambio solo lee el valor actual de la clave, nunca su historial, y una lectura histórica es un seek en las versiones de la clave. `PruneBelow` borra toda versión en o por debajo de la altura desde la que retiene. Los nodos del árbol se versionan igual. Dos estructuras en memoria reducen el trabajo de árbol de un bloque; ninguna se persiste ni cambia un resultado: la caché de nodos del árbol, con hasta 2^18 nodos codificados de los árboles actuales (cada batch del store la actualiza, y una generación impar mientras se confirma tal batch hace que un lector use la caché solo en el estado de su snapshot), y, por namespace, la raíz y los cambios de nodos del último cálculo de raíz, indexados por esa generación y un digest SHA-256 de las escrituras del namespace, de modo que el app hash, las state roots y el commit calculan las mismas escrituras una sola vez.

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
- `eth_getBalance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getProof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bank/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `auth/nonce/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/code/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/storage/{0x_address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `seen_ttl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `code/{address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `storage/{address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipts/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/{address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_receipts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipt_index/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blob_sidecars/by_tx/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blob_sidecars/by_blob/{blob_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `tree:` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipt_height/{height}/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipt_pruned_below` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
